package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/netpoll"
	"github.com/gobwas/ws"
	"github.com/google/uuid"
)

var (
	connStore *ConnectionStore
)

func init() {
	connStore = NewConnectionStore()
}

func main() {
	// netpoll 이벤트 루프 생성
	eventLoop, err := netpoll.NewEventLoop(
		handle,
		netpoll.WithOnPrepare(prepare),
	)
	if err != nil {
		panic(err)
	}

	log.Printf("서버가 8000번 포트에서 시작됩니다")
	// 서버 시작
	err = eventLoop.Serve("tcp://0.0.0.0:8000")
	if err != nil {
		panic(err)
	}
}

// 연결 준비 핸들러
func prepare(connection netpoll.Connection) context.Context {
	// websocket upgrade
	_, err := ws.Upgrade(connection)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		connection.Close()
		return nil
	}

	// 새로운 연결 정보 저장
	sessionID := uuid.New().String()
	connFD := int(connection.FD())
	connStore.Store(sessionID, connFD, connection)

	return context.Background()
}

// 메시지 처리 핸들러
func handle(ctx context.Context, connection netpoll.Connection) error {
	// 연결 FD로 ConnectionInfo 조회
	connFD := int(connection.FD())
	info, exists := connStore.GetByConnFD(connFD)
	if !exists {
		return fmt.Errorf("connection not found: %d", connFD)
	}

	// 메시지 읽기
	msg, err := connection.Reader().Next()
	if err != nil {
		connStore.Delete(info)
		return err
	}

	// 에코 응답
	writer := connection.Writer()
	_, err = writer.Write(msg)
	if err != nil {
		connStore.Delete(info)
		return err
	}

	return writer.Flush()
}
