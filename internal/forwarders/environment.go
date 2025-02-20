package forwarders

import (
	"sync"
)

type ConnectionMap struct {
	sync.RWMutex
	connections map[string]DeliveryAttemptForwarder
}

// NewConnectionMap creates a new ConnectionMap
func NewConnectionMap() *ConnectionMap {
	return &ConnectionMap{
		connections: make(map[string]DeliveryAttemptForwarder),
	}
}

// GetConnection retrieves a connection from the map
func (cm *ConnectionMap) GetConnection(key string) (DeliveryAttemptForwarder, bool) {
	cm.RLock() // Read lock
	defer cm.RUnlock()
	conn, ok := cm.connections[key]
	return conn, ok
}

// SetConnection adds or updates a connection in the map
func (cm *ConnectionMap) SetConnection(key string, connection DeliveryAttemptForwarder) {
	cm.Lock() // Write lock
	defer cm.Unlock()
	cm.connections[key] = connection
}

// Example usage
var globalConnections = NewConnectionMap()
