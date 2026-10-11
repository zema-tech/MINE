package api

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/zema-tech/MINE/panel/backend/internal/daemon"
)

// ---------- Files (proxied to daemon) ----------

func (h *Handler) daemonConnFor(c *fiber.Ctx, serverUUID string) (*daemon.Conn, *string, error) {
	s, err := h.serverByID(c)
	if err != nil {
		return nil, nil, err
	}
	conn, ok := daemon.DefaultHub.Get(s.NodeID)
	if !ok {
		return nil, nil, fiber.NewError(502, "node offline")
	}
	id := s.UUID.String()
	_ = serverUUID
	return conn, &id, nil
}

func (h *Handler) fileOp(c *fiber.Ctx, op, path, dest, content string) error {
	s, err := h.serverByID(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "server not found"})
	}
	conn, ok := daemon.DefaultHub.Get(s.NodeID)
	if !ok {
		return c.Status(502).JSON(fiber.Map{"error": "node offline"})
	}
	reqID := uuid.NewString()
	if err := conn.Send(fiber.Map{
		"type":      "file_op",
		"server_id": s.UUID.String(),
		"payload": fiber.Map{
			"op": reqIDOp(op), "request_id": reqID,
			"path": path, "dest": dest, "content": content,
		},
	}); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "failed to reach daemon"})
	}
	raw, err := conn.AwaitResponse(reqID, 20*time.Second)
	if err != nil {
		return c.Status(504).JSON(fiber.Map{"error": "daemon timeout"})
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "bad daemon response"})
	}
	if ok, _ := out["success"].(bool); !ok {
		msg := "file op failed"
		if e, _ := out["error"].(string); e != "" {
			msg = e
		}
		return c.Status(400).JSON(fiber.Map{"error": msg})
	}
	return c.JSON(out)
}

func reqIDOp(op string) string { return op }

func (h *Handler) ListFiles(c *fiber.Ctx) error {
	return h.fileOp(c, "list", c.Query("path", "/"), "", "")
}

func (h *Handler) ReadFile(c *fiber.Ctx) error {
	return h.fileOp(c, "read", c.Query("path", ""), "", "")
}

type writeReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (h *Handler) WriteFile(c *fiber.Ctx) error {
	var req writeReq
	if err := c.BodyParser(&req); err != nil || req.Path == "" {
		return c.Status(400).JSON(fiber.Map{"error": "path and content required"})
	}
	return h.fileOp(c, "write", req.Path, "", req.Content)
}

func (h *Handler) DeleteFile(c *fiber.Ctx) error {
	return h.fileOp(c, "delete", c.Query("path", ""), "", "")
}
