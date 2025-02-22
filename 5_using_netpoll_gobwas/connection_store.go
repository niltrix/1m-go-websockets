package main

import (
	"net"
	"sync"
)

// ConnectionInfo는 연결과 관련된 모든 정보를 저장합니다
type ConnectionInfo struct {
	SessionID string
	ConnFD    int
	Conn      net.Conn
}

// ConnectionStore는 두 가지 맵을 사용하여 O(1) 검색을 구현합니다
type ConnectionStore struct {
	mu *sync.RWMutex

	// 두 개의 맵으로 각각의 키에 대해 O(1) 접근 가능
	bySessionID map[string]*ConnectionInfo
	byConnFD    map[int]*ConnectionInfo
}

// NewConnectionStore는 새로운 ConnectionStore를 생성합니다
func NewConnectionStore() *ConnectionStore {
	return &ConnectionStore{
		mu:          &sync.RWMutex{},
		bySessionID: make(map[string]*ConnectionInfo),
		byConnFD:    make(map[int]*ConnectionInfo),
	}
}

// Store는 새로운 연결 정보를 저장합니다
func (cs *ConnectionStore) Store(sessionID string, connFD int, conn net.Conn) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	info := &ConnectionInfo{
		SessionID: sessionID,
		ConnFD:    connFD,
		Conn:      conn,
	}

	cs.bySessionID[sessionID] = info
	cs.byConnFD[connFD] = info
}

// GetBySessionID는 세션 ID로 ConnectionInfo를 검색합니다
func (cs *ConnectionStore) GetBySessionID(sessionID string) (*ConnectionInfo, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	info, exists := cs.bySessionID[sessionID]
	return info, exists
}

// GetByConnFD는 커넥션 FD로 ConnectionInfo를 검색합니다
func (cs *ConnectionStore) GetByConnFD(connFD int) (*ConnectionInfo, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	info, exists := cs.byConnFD[connFD]
	return info, exists
}

// Delete는 모든 맵에서 연결 정보를 제거합니다
func (cs *ConnectionStore) Delete(info *ConnectionInfo) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.bySessionID, info.SessionID)
	delete(cs.byConnFD, info.ConnFD)
}
