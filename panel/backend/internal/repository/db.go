package repository

import (
	"github.com/zema-tech/MINE/panel/backend/internal/models"
	"gorm.io/gorm"
)

// DB wraps gorm with constructor helpers.
type DB struct{ *gorm.DB }

// Migrate runs AutoMigrate for all entities (dev/test convenience).
// Production uses golang-migrate SQL files in migrations/.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Node{},
		&models.Template{},
		&models.Server{},
		&models.Allocation{},
		&models.SubUser{},
		&models.ActivityLog{},
	)
}

// LogActivity appends an audit row (best-effort, never fails requests).
func LogActivity(db *gorm.DB, userID, serverID *uint, action, metadata, ip string) {
	_ = db.Create(&models.ActivityLog{
		UserID: userID, ServerID: serverID,
		Action: action, Metadata: metadata, IPAddress: ip,
	}).Error
}
