package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/zema-tech/MINE/panel/backend/internal/daemon"
	"github.com/zema-tech/MINE/panel/backend/internal/middleware"
	"github.com/zema-tech/MINE/panel/backend/internal/models"
	"github.com/zema-tech/MINE/panel/backend/internal/repository"
)

// ---------- Servers ----------

func (h *Handler) ListServers(c *fiber.Ctx) error {
	uid := middleware.UserID(c)
	isAdmin, _ := c.Locals(middleware.CtxIsAdmin).(bool)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	per, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if per < 1 || per > 100 {
		per = 20
	}
	q := h.DB.Preload("Node").Preload("Template").Order("id DESC")
	if !isAdmin {
		q = q.Where("owner_id = ?", uid)
	}
	var total int64
	q.Model(&models.Server{}).Count(&total)
	var servers []models.Server
	if err := q.Offset((page - 1) * per).Limit(per).Find(&servers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(fiber.Map{"data": servers, "page": page, "per_page": per, "total": total})
}

type createServerReq struct {
	Name       string `json:"name"`
	NodeID     uint   `json:"node_id"`
	TemplateID uint   `json:"template_id"`
	MemoryMB   int    `json:"memory_mb"`
	DiskMB     int    `json:"disk_mb"`
	CPULimit   int    `json:"cpu_limit"`
	Variables  string `json:"variables"`
}

func (h *Handler) CreateServer(c *fiber.Ctx) error {
	uid := middleware.UserID(c)
	var req createServerReq
	if err := c.BodyParser(&req); err != nil || req.Name == "" || req.NodeID == 0 || req.TemplateID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "name, node_id, template_id required"})
	}
	if req.MemoryMB <= 0 {
		req.MemoryMB = 1024
	}
	if req.DiskMB <= 0 {
		req.DiskMB = 5120
	}
	if req.CPULimit <= 0 {
		req.CPULimit = 100
	}
	// Quota check.
	var u models.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	var count int64
	var memSum int64
	h.DB.Model(&models.Server{}).Where("owner_id = ?", uid).Count(&count)
	h.DB.Model(&models.Server{}).Where("owner_id = ?", uid).Select("COALESCE(SUM(memory_mb),0)").Scan(&memSum)
	isAdmin, _ := c.Locals(middleware.CtxIsAdmin).(bool)
	if !isAdmin {
		if int(count) >= u.MaxServers {
			return c.Status(403).JSON(fiber.Map{"error": "server quota exceeded"})
		}
		if int(memSum)+req.MemoryMB > u.MaxMemoryMB {
			return c.Status(403).JSON(fiber.Map{"error": "memory quota exceeded"})
		}
	}
	// Validate node + template exist.
	var node models.Node
	if err := h.DB.First(&node, req.NodeID).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "node not found"})
	}
	var tmpl models.Template
	if err := h.DB.First(&tmpl, req.TemplateID).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "template not found"})
	}
	if req.Variables == "" {
		req.Variables = "{}"
	}
	s := models.Server{
		UUID: uuid.New(), Name: req.Name, OwnerID: uid,
		NodeID: req.NodeID, TemplateID: req.TemplateID,
		MemoryMB: req.MemoryMB, DiskMB: req.DiskMB, CPULimit: req.CPULimit,
		Status: models.ServerOffline, Variables: req.Variables,
	}
	if err := h.DB.Create(&s).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "create failed"})
	}
	repository.LogActivity(h.DB, &uid, &s.ID, "server.create", `{"name":"`+s.Name+`"}`, c.IP())
	return c.Status(201).JSON(s)
}

func (h *Handler) serverByID(c *fiber.Ctx) (*models.Server, error) {
	id := c.Params("id")
	var s models.Server
	q := h.DB.Preload("Node").Preload("Template")
	// Accept numeric id or uuid.
	if _, err := strconv.Atoi(id); err == nil {
		if err := q.First(&s, id).Error; err != nil {
			return nil, err
		}
	} else {
		if err := q.Where("uuid = ?", id).First(&s).Error; err != nil {
			return nil, err
		}
	}
	uid := middleware.UserID(c)
	isAdmin, _ := c.Locals(middleware.CtxIsAdmin).(bool)
	if !isAdmin && s.OwnerID != uid {
		// Sub-user access?
		var sub models.SubUser
		if err := h.DB.Where("server_id = ? AND user_id = ?", s.ID, uid).First(&sub).Error; err != nil {
			return nil, fiber.NewError(403, "forbidden")
		}
	}
	return &s, nil
}

func (h *Handler) GetServer(c *fiber.Ctx) error {
	s, err := h.serverByID(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "server not found"})
	}
	return c.JSON(s)
}

func (h *Handler) UpdateServer(c *fiber.Ctx) error {
	s, err := h.serverByID(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "server not found"})
	}
	var req map[string]any
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	allowed := map[string]bool{"name": true, "memory_mb": true, "disk_mb": true, "cpu_limit": true, "variables": true}
	updates := map[string]any{}
	for k, v := range req {
		if allowed[k] {
			updates[k] = v
		}
	}
	if len(updates) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "nothing to update"})
	}
	if err := h.DB.Model(s).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "update failed"})
	}
	uid := middleware.UserID(c)
	repository.LogActivity(h.DB, &uid, &s.ID, "server.update", "{}", c.IP())
	return c.JSON(s)
}

func (h *Handler) DeleteServer(c *fiber.Ctx) error {
	s, err := h.serverByID(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "server not found"})
	}
	// Tell daemon to remove container (best effort).
	if conn, ok := daemon.DefaultHub.Get(s.NodeID); ok {
		_ = conn.Send(fiber.Map{"type": "power", "server_id": s.UUID.String(), "payload": fiber.Map{"action": "kill"}})
	}
	h.DB.Delete(&models.SubUser{}, "server_id = ?", s.ID)
	h.DB.Where("server_id = ?", s.ID).Delete(&models.ActivityLog{})
	if err := h.DB.Delete(s).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "delete failed"})
	}
	uid := middleware.UserID(c)
	repository.LogActivity(h.DB, &uid, nil, "server.delete", `{"name":"`+s.Name+`"}`, c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

// PowerAction returns a handler that forwards start/stop/restart/kill to the daemon.
func (h *Handler) PowerAction(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		s, err := h.serverByID(c)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "server not found"})
		}
		conn, ok := daemon.DefaultHub.Get(s.NodeID)
		if !ok {
			return c.Status(502).JSON(fiber.Map{"error": "node offline"})
		}
		if err := conn.Send(fiber.Map{
			"type":      "power",
			"server_id": s.UUID.String(),
			"payload":   fiber.Map{"action": action},
		}); err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "failed to reach daemon"})
		}
		status := models.ServerOffline
		switch action {
		case "start", "restart":
			status = models.ServerStarting
		case "stop":
			status = models.ServerStopping
		}
		h.DB.Model(s).Update("status", status)
		uid := middleware.UserID(c)
		repository.LogActivity(h.DB, &uid, &s.ID, "server.power:"+action, "{}", c.IP())
		return c.JSON(fiber.Map{"ok": true, "action": action})
	}
}

type commandReq struct {
	Command string `json:"command"`
}

func (h *Handler) SendCommand(c *fiber.Ctx) error {
	s, err := h.serverByID(c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "server not found"})
	}
	var req commandReq
	if err := c.BodyParser(&req); err != nil || req.Command == "" {
		return c.Status(400).JSON(fiber.Map{"error": "command required"})
	}
	conn, ok := daemon.DefaultHub.Get(s.NodeID)
	if !ok {
		return c.Status(502).JSON(fiber.Map{"error": "node offline"})
	}
	if err := conn.Send(fiber.Map{
		"type":      "console_input",
		"server_id": s.UUID.String(),
		"payload":   fiber.Map{"data": req.Command + "\n"},
	}); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "failed to reach daemon"})
	}
	return c.JSON(fiber.Map{"ok": true})
}
