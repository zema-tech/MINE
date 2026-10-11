package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zema-tech/MINE/panel/backend/internal/auth"
	"github.com/zema-tech/MINE/panel/backend/internal/config"
	"github.com/zema-tech/MINE/panel/backend/internal/middleware"
	"github.com/zema-tech/MINE/panel/backend/internal/models"
	"github.com/zema-tech/MINE/panel/backend/internal/repository"
)

// ---------- Nodes (admin) ----------

func (h *Handler) ListNodes(c *fiber.Ctx) error {
	var nodes []models.Node
	if err := h.DB.Order("id ASC").Find(&nodes).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(nodes)
}

type createNodeReq struct {
	Name       string `json:"name"`
	FQDN       string `json:"fqdn"`
	DaemonPort int    `json:"daemon_port"`
	MemoryMB   int    `json:"memory_mb"`
	DiskMB     int    `json:"disk_mb"`
	Location   string `json:"location"`
}

func (h *Handler) CreateNode(c *fiber.Ctx) error {
	var req createNodeReq
	if err := c.BodyParser(&req); err != nil || req.Name == "" || req.FQDN == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name and fqdn required"})
	}
	if req.DaemonPort == 0 {
		req.DaemonPort = 8081
	}
	rawToken, err := config.RandomToken(32)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "token gen failed"})
	}
	enc, err := auth.EncryptSecret(h.Cfg.EncryptionKey, rawToken)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "encrypt failed"})
	}
	n := models.Node{
		Name: req.Name, FQDN: req.FQDN, DaemonPort: req.DaemonPort,
		Token: enc, MemoryMB: req.MemoryMB, DiskMB: req.DiskMB, Location: req.Location,
	}
	if err := h.DB.Create(&n).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "create failed"})
	}
	uid := middleware.UserID(c)
	repository.LogActivity(h.DB, &uid, nil, "node.create", `{"name":"`+n.Name+`"}`, c.IP())
	// Return raw token ONCE so the operator can configure the daemon.
	return c.Status(201).JSON(fiber.Map{
		"id": n.ID, "name": n.Name, "fqdn": n.FQDN,
		"daemon_port": n.DaemonPort, "token": rawToken,
	})
}

func (h *Handler) DeleteNode(c *fiber.Ctx) error {
	var n models.Node
	if err := h.DB.First(&n, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "node not found"})
	}
	var count int64
	h.DB.Model(&models.Server{}).Where("node_id = ?", n.ID).Count(&count)
	if count > 0 {
		return c.Status(409).JSON(fiber.Map{"error": "node still has servers"})
	}
	h.DB.Delete(&n)
	uid := middleware.UserID(c)
	repository.LogActivity(h.DB, &uid, nil, "node.delete", `{"name":"`+n.Name+`"}`, c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- Templates ----------

func (h *Handler) ListTemplates(c *fiber.Ctx) error {
	var t []models.Template
	if err := h.DB.Order("name ASC").Find(&t).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(t)
}

type templateReq struct {
	Name           string `json:"name"`
	Game           string `json:"game"`
	DockerImage    string `json:"docker_image"`
	StartupCommand string `json:"startup_command"`
	ConfigSchema   string `json:"config_schema"`
}

func (h *Handler) CreateTemplate(c *fiber.Ctx) error {
	var req templateReq
	if err := c.BodyParser(&req); err != nil || req.Name == "" || req.DockerImage == "" || req.StartupCommand == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name, docker_image, startup_command required"})
	}
	if req.ConfigSchema == "" {
		req.ConfigSchema = "{}"
	}
	t := models.Template{Name: req.Name, Game: req.Game, DockerImage: req.DockerImage, StartupCommand: req.StartupCommand, ConfigSchema: req.ConfigSchema}
	if err := h.DB.Create(&t).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "create failed"})
	}
	uid := middleware.UserID(c)
	repository.LogActivity(h.DB, &uid, nil, "template.create", `{"name":"`+t.Name+`"}`, c.IP())
	return c.Status(201).JSON(t)
}

func (h *Handler) UpdateTemplate(c *fiber.Ctx) error {
	var t models.Template
	if err := h.DB.First(&t, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "template not found"})
	}
	var req templateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	h.DB.Model(&t).Updates(models.Template{Name: req.Name, Game: req.Game, DockerImage: req.DockerImage, StartupCommand: req.StartupCommand, ConfigSchema: req.ConfigSchema})
	return c.JSON(t)
}

func (h *Handler) DeleteTemplate(c *fiber.Ctx) error {
	var t models.Template
	if err := h.DB.First(&t, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "template not found"})
	}
	var count int64
	h.DB.Model(&models.Server{}).Where("template_id = ?", t.ID).Count(&count)
	if count > 0 {
		return c.Status(409).JSON(fiber.Map{"error": "template in use"})
	}
	h.DB.Delete(&t)
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- Users / Activity (admin) ----------

func (h *Handler) ListUsers(c *fiber.Ctx) error {
	var users []models.User
	if err := h.DB.Order("id ASC").Limit(100).Find(&users).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	// Strip secrets (GORM json:"-" already hides hash, but be explicit).
	for i := range users {
		users[i].PasswordHash = ""
		users[i].TOTPSecret = ""
	}
	return c.JSON(users)
}

func (h *Handler) ListActivity(c *fiber.Ctx) error {
	var logs []models.ActivityLog
	q := h.DB.Order("id DESC").Limit(100)
	if sid := c.Query("server_id"); sid != "" {
		q = q.Where("server_id = ?", sid)
	}
	if err := q.Find(&logs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "db error"})
	}
	return c.JSON(logs)
}

// SeedAdmin creates the first admin from env if no users exist.
func (h *Handler) SeedAdmin() error {
	var count int64
	h.DB.Model(&models.User{}).Count(&count)
	if count > 0 {
		return nil
	}
	hash, err := auth.HashPassword(h.Cfg.AdminPassword)
	if err != nil {
		return err
	}
	u := models.User{
		Email: h.Cfg.AdminEmail, Username: "admin",
		PasswordHash: hash, Role: models.RoleAdmin, IsAdmin: true,
		MaxServers: 1000, MaxMemoryMB: 1 << 20, MaxDiskMB: 1 << 24,
		CreatedAt: time.Now(),
	}
	return h.DB.Create(&u).Error
}
