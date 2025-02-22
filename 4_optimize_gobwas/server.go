package main

import (
	"context"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"syscall"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var epoller *epoll

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}
	sessionID := uuid.New().String()
	if err := epoller.Add(conn, sessionID); err != nil {
		log.Printf("Failed to add connection %v", err)
		conn.Close()
	}
}

func wsAnnounce(w http.ResponseWriter, r *http.Request) {
	epoller.lock.RLock()
	defer epoller.lock.RUnlock()
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	msg, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, conn := range epoller.connections {
		if err := wsutil.WriteServerText(conn, msg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func wsPush(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	conn, err := epoller.GetBySessionID(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	msg, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := wsutil.WriteServerText(conn, msg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	shutdown, err := InitTelemetry()
	if err != nil {
		logger.Fatal("Failed to initialize telemetry", zap.Error(err))
	}
	defer shutdown()

	// Enable pprof hooks
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("pprof failed: %v", err)
		}
	}()

	// Start epoll
	// var err error
	epoller, err = MkEpoll()
	if err != nil {
		panic(err)
	}

	go MonitorMemory(context.Background())

	go Start()

	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/announce", wsAnnounce)
	http.HandleFunc("/push", wsPush)
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func Start() {
	for {
		connections, err := epoller.Wait()
		if err != nil {
			// syscall.EINTR is expected when the process is interrupted by a signal
			// so we don't need to log it
			if err != syscall.EINTR {
				log.Printf("Failed to epoll wait %v", err)
			}
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			if msg, _, err := wsutil.ReadClientData(conn); err != nil {
				if err := epoller.Remove(conn); err != nil {
					log.Printf("Failed to remove %v", err)
				}
				conn.Close()
			} else {
				fd := WebsocketFD(conn)
				epoller.lock.RLock()
				sessionID, ok := epoller.sessions.GetInverse(fd)
				epoller.lock.RUnlock()
				if !ok {
					log.Printf("session not found: %d", fd)
					continue
				}
				log.Printf("sessionID: %s, msg: %s", sessionID, string(msg))
				// log.Printf("msg: %s", string(msg))
			}
		}
	}
}
