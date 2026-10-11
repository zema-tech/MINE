package api

import (
	"sync"
)

// pubsub.go — tiny in-memory fan-out for console_output + stats frames
// received from daemons to browser WebSocket subscribers.

var (
	subMu  sync.RWMutex
	subs   = map[string]map[chan []byte]struct{}{}
	bufLen = 64
)

// Subscribe returns a channel receiving raw payload frames for serverUUID.
func Subscribe(serverUUID string) chan []byte {
	ch := make(chan []byte, bufLen)
	subMu.Lock()
	if subs[serverUUID] == nil {
		subs[serverUUID] = map[chan []byte]struct{}{}
	}
	subs[serverUUID][ch] = struct{}{}
	subMu.Unlock()
	return ch
}

// Unsubscribe removes ch.
func Unsubscribe(serverUUID string, ch chan []byte) {
	subMu.Lock()
	defer subMu.Unlock()
	if m, ok := subs[serverUUID]; ok {
		delete(m, ch)
		close(ch)
		if len(m) == 0 {
			delete(subs, serverUUID)
		}
	}
}

func publish(serverUUID string, frame []byte) {
	subMu.RLock()
	defer subMu.RUnlock()
	for ch := range subs[serverUUID] {
		select {
		case ch <- frame:
		default:
			// drop if slow consumer
		}
	}
}

// BroadcastConsole wraps daemon console_output payload for browsers.
func BroadcastConsole(serverUUID string, payload []byte) {
	frame := append([]byte(`{"type":"console_output","payload":`), payload...)
	frame = append(frame, '}')
	publish(serverUUID, frame)
}

// BroadcastStats wraps daemon stats payload for browsers.
func BroadcastStats(serverUUID string, payload []byte) {
	frame := append([]byte(`{"type":"stats","payload":`), payload...)
	frame = append(frame, '}')
	publish(serverUUID, frame)
}
