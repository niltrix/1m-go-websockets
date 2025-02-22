package main

import (
	"fmt"
	"log"
	"net"
	"reflect"
	"sync"
	"syscall"

	"github.com/vishalkuo/bimap"
	"golang.org/x/sys/unix"
)

type epoll struct {
	fd          int                       // epoll 파일 디스크립터
	sessions    *bimap.BiMap[string, int] // sessionID 와 파일 디스크립터 매핑을 위한 양방향 맵
	connections map[int]net.Conn          // websocket 의 connection 과 파일 디스크립터 매핑
	lock        *sync.RWMutex             // connections map 에 대한 락
}

func MkEpoll() (*epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &epoll{
		fd:          fd,
		lock:        &sync.RWMutex{},
		sessions:    bimap.NewBiMap[string, int](),
		connections: make(map[int]net.Conn),
	}, nil
}

func (e *epoll) Add(conn net.Conn, sessionID string) error {
	// Extract file descriptor associated with the connection
	fd := WebsocketFD(conn)
	// epoll_ctl(2) 를 통해 등록할 conn 의 받을 이벤트 타입을 지정
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.POLLIN | unix.POLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}
	// 연결 정보 추가시 RW 락 걸어줌
	e.lock.Lock()
	defer e.lock.Unlock()
	e.connections[fd] = conn
	e.sessions.Insert(sessionID, fd)
	// 매번 정보출력은 TMI 이므로 100개 단위로 연결 정보 출력
	if len(e.connections)%100 == 0 {
		log.Printf("Total number of connections: %v", len(e.connections))
	}
	return nil
}

func (e *epoll) Remove(conn net.Conn) error {
	fd := WebsocketFD(conn)
	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	// 연결 정보 삭제시 RW 락 걸어줌
	e.lock.Lock()
	defer e.lock.Unlock()
	e.sessions.DeleteInverse(fd)
	delete(e.connections, fd)
	if len(e.connections)%100 == 0 {
		log.Printf("Total number of connections: %v", len(e.connections))
	}
	return nil
}

func (e *epoll) Wait() ([]net.Conn, error) {
	events := make([]unix.EpollEvent, 100)
	n, err := unix.EpollWait(e.fd, events, 100)
	if err != nil {
		return nil, err
	}
	// 연결 정보 조회시 Write에만 락 걸어줌
	e.lock.RLock()
	defer e.lock.RUnlock()
	var connections []net.Conn
	for i := 0; i < n; i++ {
		conn := e.connections[int(events[i].Fd)]
		connections = append(connections, conn)
	}
	return connections, nil
}

func (e *epoll) GetBySessionID(sessionId string) (net.Conn, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()
	fd, ok := e.sessions.Get(sessionId)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionId)
	}
	conn, ok := e.connections[fd]
	if !ok {
		return nil, fmt.Errorf("connection not found: %d", fd)
	}
	return conn, nil
}

/*
net.Conn 의 내부 시스템 파일 디스크립터를 추출 ( EpollCtl 에 수신할 이벤트 등록하기 위해)
*/
func WebsocketFD(conn net.Conn) int {
	tcpConn := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

	return int(pfdVal.FieldByName("Sysfd").Int())
	// tcpConn, ok := conn.(*net.TCPConn)
	// if !ok {
	// 	return -1
	// }
	// fd, err := tcpConn.File()
	// if err != nil {
	// 	return -1
	// }
	// return int(fd.Fd())
}
