package api

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/zema-tech/MINE/panel/backend/internal/auth"
	"github.com/zema-tech/MINE/panel/backend/internal/daemon"
	"github.com/zema-tech/MINE/panel/backend/internal/middleware"
	"github.com/zema-tech/MINE/panel/backend/internal/models"
)

// ---------- Daemon WebSocket ----------
// Daemons connect to GET /api/daemon/ws and authenticate with their node token.

// DaemonWS upgrades and handles one daemon connection.
func (h *Handler) DaemonWS(c *websocket.Conn) {
	// First frame must be auth.
	_, first, err := c.ReadMessage()
	if err != nil {
		_ = c.Close()
		return
	}
	var envelope struct {
		Type    string `json:"type"`
		Payload struct {
			Token string `json:"token"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(first, &envelope); err != nil || envelope.Type != "auth" {
		_ = c.WriteJSON(fiber.Map{"type": "auth_error", "payload": fiber.Map{"error": "auth required first"}})
		_ = c.Close()
		return
	}
	node, ok := h.authenticateDaemon(envelope.Payload.Token)
	if !ok {
		_ = c.WriteJSON(fiber.Map{"type": "auth_error", "payload": fiber.Map{"error": "bad token"}})
		_ = c.Close()
		return
	}
	conn := daemon.DefaultHub.Register(node.ID, c)
	defer daemon.DefaultHub.Unregister(node.ID, c)
	now := time.Now()
	h.DB.Model(node).Updates(map[string]any{"online": true, "last_seen_at": &now})

	_ = conn.Send(fiber.Map{"type": "auth_ok", "payload": fiber.Map{"node_id": node.ID}})

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			break
		}
		h.handleDaemonMessage(node, conn, msg)
	}
	now = time.Now()
	h.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(map[string]any{"online": false, "last_seen_at": &now})
}

func (h *Handler) authenticateDaemon(token string) (*models.Node, bool) {
	var nodes []models.Node
	if err := h.DB.Find(&nodes).Error; err != nil {
		return nil, false
	}
	for i := range nodes {
		dec, err := auth.DecryptSecret(h.Cfg.EncryptionKey, nodes[i].Token)
		if err != nil {
			continue
		}
		if dec == token {
			n := nodes[i]
			return &n, true
		}
	}
	return nil, false
}

func (h *Handler) handleDaemonMessage(node *models.Node, conn *daemon.Conn, raw []byte) {
	var env struct {
		Type     string          `json:"type"`
		ServerID string          `json:"server_id"`
		Payload  json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}
	now := time.Now()
	conn.LastSeen = now
	h.DB.Model(&models.Node{}).Where("id = ?", node.ID).Update("last_seen_at", &now)

	switch env.Type {
	case "heartbeat":
		_ = conn.Send(fiber.Map{"type": "heartbeat_ack"})
	case "file_op_result":
		var p struct {
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil && p.RequestID != "" {
			conn.Deliver(p.RequestID, env.Payload)
		}
	case "stats":
		// Stats are broadcast to console subscribers; persist lightweight status.
		BroadcastStats(env.ServerID, env.Payload)
	case "console_output":
		BroadcastConsole(env.ServerID, env.Payload)
	case "power_ack":
		var p struct {
			Action  string `json:"action"`
			Success bool   `json:"success"`
			Status  string `json:"status"`
		}
		_ = json.Unmarshal(env.Payload, &p)
		if env.ServerID != "" {
			status := models.ServerStatus(p.Status)
			if status == "" {
				if p.Action == "start" && p.Success {
					status = models.ServerOnline
				} else if p.Success {
					status = models.ServerOffline
				}
			}
			if status != "" {
				h.DB.Model(&models.Server{}).Where("uuid = ?", env.ServerID).Update("status", status)
			}
		}
	}
}

// ---------- Client console WebSocket ----------
// GET /api/servers/:id/console streams console_output + stats to the browser.
// The browser → panel frames are forwarded to the daemon as console_input.

func (h *Handler) ServerConsole(c *websocket.Conn) {
	// Token passed as query ?token= (EventSource/WS can't set headers easily).
	token := c.Query("token")
	if token == "" {
		_ = c.Close()
		return
	}
	claims, err := auth.ParseJWT(h.Cfg.JWTSecret, token)
	if err != nil {
		_ = c.Close()
		return
	}
	serverUUID := c.Params("id")
	var s models.Server
	if err := h.DB.Where("uuid = ?", serverUUID).First(&s).Error; err != nil {
		// also try numeric id
		if err2 := h.DB.First(&s, serverUUID).Error; err2 != nil {
			_ = c.Close()
			return
		}
	}
	if !claims.IsAdmin && s.OwnerID != claims.UserID {
		_ = c.Close()
		return
	}
	ch := Subscribe(serverUUID)
	defer Unsubscribe(serverUUID, ch)

	// Forward browser input → daemon.
	go func() {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var in struct {
				Type string `json:"type"`
				Data string `json:"data"`
			}
			if err := json.Unmarshal(msg, &in); err != nil {
				continue
			}
			data := in.Data
			if in.Type == "" {
				// raw string frame = console input
				data = string(msg)
				if len(data) > 0 && data[0] == '"' {
					var s string
					if json.Unmarshal(msg, &s) == nil {
						data = s
					}
				}
			}
			if conn, ok := daemon.DefaultHub.Get(s.NodeID); ok && data != "" {
				_ = conn.Send(fiber.Map{
					"type":      "console_input",
					"server_id": s.UUID.String(),
					"payload":   fiber.Map{"data": data},
				})
			}
		}
	}()

	for payload := range ch {
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
	_ = middleware.UserID(nil) // keep import if unused in future
}
