package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Role is a coarse RBAC role for users.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleModerator Role = "moderator"
	RoleClient    Role = "client"
)

// User owns servers or administrates the panel.
type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Email        string `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"not null" json:"-"`
	TOTPSecret   string `json:"-"`
	TOTPEnabled  bool   `json:"totp_enabled"`
	Role         Role   `gorm:"size:16;default:client" json:"role"`
	IsAdmin      bool   `gorm:"default:false" json:"is_admin"`
	MaxServers   int    `gorm:"default:5" json:"max_servers"`
	MaxMemoryMB  int    `gorm:"default:4096" json:"max_memory_mb"`
	MaxDiskMB    int    `gorm:"default:20480" json:"max_disk_mb"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) Admin() bool { return u.IsAdmin || u.Role == RoleAdmin }

// Node is a physical/virtual machine running mined (the daemon).
type Node struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"size:128;not null" json:"name"`
	FQDN       string `gorm:"size:255;not null" json:"fqdn"`
	DaemonPort int    `gorm:"default:8081" json:"daemon_port"`
	// Token is the daemon's registration secret (stored encrypted).
	Token      string `gorm:"size:512;not null" json:"-"`
	MemoryMB   int    `json:"memory_mb"`
	DiskMB     int    `json:"disk_mb"`
	Location   string `gorm:"size:64" json:"location"`
	Online     bool   `gorm:"default:false" json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Template (egg) describes how to run a game server.
type Template struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Name           string `gorm:"size:128;not null" json:"name"`
	Game           string `gorm:"size:64;not null" json:"game"`
	DockerImage    string `gorm:"size:255;not null" json:"docker_image"`
	StartupCommand string `gorm:"type:text;not null" json:"startup_command"`
	// ConfigSchema is a JSON schema for template variables.
	ConfigSchema string `gorm:"type:jsonb;default:'{}'" json:"config_schema"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ServerStatus is the lifecycle state of a game server.
type ServerStatus string

const (
	ServerInstalling ServerStatus = "installing"
	ServerOffline    ServerStatus = "offline"
	ServerStarting   ServerStatus = "starting"
	ServerOnline     ServerStatus = "online"
	ServerStopping   ServerStatus = "stopping"
	ServerSuspended  ServerStatus = "suspended"
)

// Server is a game server instance.
type Server struct {
	ID         uint         `gorm:"primaryKey" json:"id"`
	UUID       uuid.UUID    `gorm:"type:uuid;uniqueIndex;not null" json:"uuid"`
	Name       string       `gorm:"size:128;not null" json:"name"`
	OwnerID    uint         `gorm:"index;not null" json:"owner_id"`
	Owner      User         `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	NodeID     uint         `gorm:"index;not null" json:"node_id"`
	Node       Node         `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	TemplateID uint         `gorm:"index;not null" json:"template_id"`
	Template   Template     `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	MemoryMB   int          `json:"memory_mb"`
	DiskMB     int          `json:"disk_mb"`
	CPULimit   int          `json:"cpu_limit"` // percent of one core *100, e.g. 200 = 2 cores
	Status     ServerStatus `gorm:"size:16;default:offline" json:"status"`
	Variables  string       `gorm:"type:jsonb;default:'{}'" json:"variables"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

func (s *Server) BeforeCreate(tx *gorm.DB) error {
	if s.UUID == uuid.Nil {
		s.UUID = uuid.New()
	}
	return nil
}

// Allocation binds an IP:port on a node to a server.
type Allocation struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	NodeID   uint   `gorm:"index;not null" json:"node_id"`
	IP       string `gorm:"size:64;not null" json:"ip"`
	Port     int    `gorm:"not null" json:"port"`
	ServerID *uint  `gorm:"index" json:"server_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SubUser grants another user scoped permissions on a server.
type SubUser struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ServerID    uint   `gorm:"index;not null" json:"server_id"`
	UserID      uint   `gorm:"index;not null" json:"user_id"`
	Permissions string `gorm:"type:jsonb;default:'[]'" json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

// ActivityLog is an append-only audit trail.
type ActivityLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    *uint     `gorm:"index" json:"user_id,omitempty"`
	ServerID  *uint     `gorm:"index" json:"server_id,omitempty"`
	Action    string    `gorm:"size:64;not null" json:"action"`
	Metadata  string    `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	IPAddress string    `gorm:"size:64" json:"ip_address"`
	CreatedAt time.Time `gorm:"index;not null" json:"timestamp"`
}
