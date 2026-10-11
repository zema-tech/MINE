package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zema-tech/MINE/panel/backend/internal/auth"
	"github.com/zema-tech/MINE/panel/backend/internal/config"
	"github.com/zema-tech/MINE/panel/backend/internal/middleware"
	"github.com/zema-tech/MINE/panel/backend/internal/models"
	"github.com/zema-tech/MINE/panel/backend/internal/repository"
	"gorm.io/gorm"
)

// Handler bundles dependencies for all routes.
type Handler struct {
	DB  *gorm.DB
	Cfg config.Config
}

// New creates a Handler.
func New(db *gorm.DB, cfg config.Config) *Handler { return &Handler{DB: db, Cfg: cfg} }

// RegisterRoutes mounts /api/*.
func (h *Handler) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api", middleware.RateLimit(300))

	authG := api.Group("/auth", middleware.RateLimit(20))
	authG.Post("/register", h.Register)
	authG.Post("/login", h.Login)
	authG.Post("/logout", h.Logout)
	authG.Post("/2fa/setup", middleware.JWTAuth(h.Cfg.JWTSecret), h.TwoFASetup)
	authG.Post("/2fa/verify", h.TwoFAVerify)

	srv := api.Group("/servers", middleware.JWTAuth(h.Cfg.JWTSecret))
	srv.Get("/", h.ListServers)
	srv.Post("/", h.CreateServer)
	srv.Get("/:id", h.GetServer)
	srv.Put("/:id", h.UpdateServer)
	srv.Delete("/:id", h.DeleteServer)
	srv.Post("/:id/start", h.PowerAction("start"))
	srv.Post("/:id/stop", h.PowerAction("stop"))
	srv.Post("/:id/restart", h.PowerAction("restart"))
	srv.Post("/:id/kill", h.PowerAction("kill"))
	srv.Post("/:id/command", h.SendCommand)
	srv.Get("/:id/files", h.ListFiles)
	srv.Get("/:id/files/read", h.ReadFile)
	srv.Put("/:id/files/write", h.WriteFile)
	srv.Delete("/:id/files", h.DeleteFile)

	admin := api.Group("/", middleware.JWTAuth(h.Cfg.JWTSecret), middleware.RequireAdmin())
	admin.Get("/nodes", h.ListNodes)
	admin.Post("/nodes", h.CreateNode)
	admin.Delete("/nodes/:id", h.DeleteNode)
	admin.Get("/users", h.ListUsers)
	admin.Get("/templates", h.ListTemplates)
	admin.Post("/templates", h.CreateTemplate)
	admin.Put("/templates/:id", h.UpdateTemplate)
	admin.Delete("/templates/:id", h.DeleteTemplate)
	admin.Get("/activity", h.ListActivity)

	api.Get("/templates", middleware.JWTAuth(h.Cfg.JWTSecret), h.ListTemplates)
}

// ---------- Auth ----------

type registerReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req registerReq
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" || req.Username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "email, username and password required"})
	}
	if len(req.Password) < 8 {
		return c.Status(400).JSON(fiber.Map{"error": "password must be >= 8 chars"})
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "hashing failed"})
	}
	u := models.User{Email: req.Email, Username: req.Username, PasswordHash: hash, Role: models.RoleClient}
	if err := h.DB.Create(&u).Error; err != nil {
		return c.Status(409).JSON(fiber.Map{"error": "email or username taken"})
	}
	uid := u.ID
	repository.LogActivity(h.DB, &uid, nil, "user.register", "{}", c.IP())
	return c.Status(201).JSON(fiber.Map{"id": u.ID, "email": u.Email, "username": u.Username})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	var u models.User
	if err := h.DB.Where("email = ?", req.Email).First(&u).Error; err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if !auth.VerifyPassword(u.PasswordHash, req.Password) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if u.TOTPEnabled && !auth.VerifyTOTP(u.TOTPSecret, req.Code) {
		return c.Status(401).JSON(fiber.Map{"error": "2fa code required/invalid", "need_2fa": true})
	}
	tok, err := auth.IssueJWT(h.Cfg.JWTSecret, u.ID, u.Admin(), 12*time.Hour)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "token issue failed"})
	}
	repository.LogActivity(h.DB, &u.ID, nil, "user.login", "{}", c.IP())
	return c.JSON(fiber.Map{"token": tok, "user": fiber.Map{
		"id": u.ID, "email": u.Email, "username": u.Username,
		"is_admin": u.Admin(), "totp_enabled": u.TOTPEnabled,
	}})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	// Stateless JWT: client discards token. Logged for audit.
	uid := middleware.UserID(c)
	repository.LogActivity(h.DB, &uid, nil, "user.logout", "{}", c.IP())
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) TwoFASetup(c *fiber.Ctx) error {
	uid := middleware.UserID(c)
	var u models.User
	if err := h.DB.First(&u, uid).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	secret, uri, err := auth.GenerateTOTPSecret(u.Email)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "totp failed"})
	}
	u.TOTPSecret = secret
	h.DB.Save(&u)
	return c.JSON(fiber.Map{"secret": secret, "otpauth_url": uri})
}

type verifyReq struct {
	UserID uint   `json:"user_id"`
	Code   string `json:"code"`
}

func (h *Handler) TwoFAVerify(c *fiber.Ctx) error {
	var req verifyReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	// Allow verifying own setup (JWT) or login-step (user_id without JWT).
	id := req.UserID
	if id == 0 {
		id = middleware.UserID(c)
	}
	var u models.User
	if err := h.DB.First(&u, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	if !auth.VerifyTOTP(u.TOTPSecret, req.Code) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid code"})
	}
	u.TOTPEnabled = true
	h.DB.Save(&u)
	return c.JSON(fiber.Map{"ok": true})
}
