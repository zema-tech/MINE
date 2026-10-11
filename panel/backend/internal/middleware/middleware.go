package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/zema-tech/MINE/panel/backend/internal/auth"
)

// Ctx keys.
const (
	CtxUserID  = "mine_user_id"
	CtxIsAdmin = "mine_is_admin"
)

// JWTAuth validates Bearer tokens and stores claims in context.
func JWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
		}
		claims, err := auth.ParseJWT(secret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		c.Locals(CtxUserID, claims.UserID)
		c.Locals(CtxIsAdmin, claims.IsAdmin)
		return c.Next()
	}
}

// RequireAdmin blocks non-admin users.
func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if v, _ := c.Locals(CtxIsAdmin).(bool); !v {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin only"})
		}
		return c.Next()
	}
}

// UserID returns the authenticated user id (0 if absent).
func UserID(c *fiber.Ctx) uint {
	if v, ok := c.Locals(CtxUserID).(uint); ok {
		return v
	}
	return 0
}

// RateLimit creates a per-IP sliding limiter: max requests per minute.
func RateLimit(maxPerMin int) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        maxPerMin,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
		},
	})
}
