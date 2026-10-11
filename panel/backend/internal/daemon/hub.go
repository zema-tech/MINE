package daemon

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

// Hub tracks one live WebSocket per node id.
type Hub struct {
	mu    sync.RWMutex
	conns map[uint]*Conn
}

// Conn wraps a daemon connection.
type Conn struct {
	NodeID   uint
	WS       *websocket.Conn
	writeMu  sync.Mutex
	LastSeen time.Time
	pending  map[string]chan json.RawMessage // request_id -> waiter
	pmu      sync.Mutex
}

var DefaultHub = &Hub{conns: map[uint]*Conn{}}

func (h *Hub) Register(nodeID uint, ws *websocket.Conn) *Conn {
	c := &Conn{NodeID: nodeID, WS: ws, LastSeen: time.Now(), pending: map[string]chan json.RawMessage{}}
	h.mu.Lock()
	h.conns[nodeID] = c
	h.mu.Unlock()
	return c
}

func (h *Hub) Unregister(nodeID uint, ws *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.conns[nodeID]; ok && cur.WS == ws {
		delete(h.conns, nodeID)
	}
}

func (h *Hub) Get(nodeID uint) (*Conn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.conns[nodeID]
	return c, ok
}

func (h *Hub) Online(nodeID uint) bool {
	_, ok := h.Get(nodeID)
	return ok
}

// Send writes a JSON frame thread-safely.
func (c *Conn) Send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.WS.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.WS.WriteMessage(websocket.TextMessage, b)
}

// AwaitResponse waits for a file_op_result with request_id (up to timeout).
func (c *Conn) AwaitResponse(requestID string, timeout time.Duration) (json.RawMessage, error) {
	ch := make(chan json.RawMessage, 1)
	c.pmu.Lock()
	c.pending[requestID] = ch
	c.pmu.Unlock()
	defer func() {
		c.pmu.Lock()
		delete(c.pending, requestID)
		c.pmu.Unlock()
	}()
	select {
	case raw := <-ch:
		return raw, nil
	case <-time.After(timeout):
		return nil, errors.New("daemon timeout")
	}
}

// Deliver routes an incoming file_op_result to its waiter.
func (c *Conn) Deliver(requestID string, raw json.RawMessage) {
	c.pmu.Lock()
	ch, ok := c.pending[requestID]
	c.pmu.Unlock()
	if ok {
		select {
		case ch <- raw:
		default:
		}
	}
}
