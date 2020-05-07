package sfu

import (
	"sync"

	"github.com/pion/webrtc/v2"
)

//Connection is a struct
type Connection struct {
	localTrack     *webrtc.Track
	peerConnection *webrtc.PeerConnection
}

//ConnectionsMap is synced map of Connection
type ConnectionsMap struct {
	sync.RWMutex
	connections map[string]*Connection
}

//NewConnectionsMap creates new ConnectionsMap
func NewConnectionsMap() *ConnectionsMap {
	return &ConnectionsMap{
		connections: make(map[string]*Connection),
	}
}

//Load returns a connection
func (m *ConnectionsMap) Load(key string) (conn *Connection, ok bool) {
	m.RLock()
	result, ok := m.connections[key]
	m.RUnlock()
	return result, ok
}

//Delete removes a connection
func (m *ConnectionsMap) Delete(key string) {
	m.Lock()
	delete(m.connections, key)
	m.Unlock()
}

//Store stores a connection in the map
func (m *ConnectionsMap) Store(key string, conn *Connection) {
	m.Lock()
	m.connections[key] = conn
	m.Unlock()
}
