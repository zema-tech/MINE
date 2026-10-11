// Package types defines the Panel↔Daemon WebSocket protocol.
//
// Transport: wss://, persistent connection initiated by the daemon.
// Auth: first message must be {"type":"auth","token":"<node token>"}.
// Heartbeat: daemon sends {"type":"heartbeat"} every 10s; panel replies
// {"type":"heartbeat_ack"}. Timeout 30s.
// All messages are JSON with a mandatory "type" field.
package types

// Message types exchanged over the daemon WebSocket.
const (
	TypeAuth          = "auth"
	TypeAuthOK        = "auth_ok"
	TypeAuthError     = "auth_error"
	TypeHeartbeat     = "heartbeat"
	TypeHeartbeatAck  = "heartbeat_ack"
	TypePower         = "power"
	TypePowerAck      = "power_ack"
	TypeConsoleOutput = "console_output"
	TypeConsoleInput  = "console_input"
	TypeStats         = "stats"
	TypeFileOp        = "file_op"
	TypeFileOpResult  = "file_op_result"
)

// Power actions.
const (
	PowerStart   = "start"
	PowerStop    = "stop"
	PowerRestart = "restart"
	PowerKill    = "kill"
)

// Message is the envelope for every frame.
type Message struct {
	Type     string `json:"type"`
	ServerID string `json:"server_id,omitempty"`
	Payload  any    `json:"payload,omitempty"`
}

// RawMessage decodes the envelope keeping the payload raw.
type RawMessage struct {
	Type     string `json:"type"`
	ServerID string `json:"server_id,omitempty"`
	Payload  any    `json:"payload,omitempty"`
}

// AuthPayload is sent by the daemon right after connecting.
type AuthPayload struct {
	Token    string `json:"token"`
	NodeID   uint   `json:"node_id,omitempty"`
	Version  string `json:"version,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// PowerPayload asks the daemon to change a container's power state.
type PowerPayload struct {
	Action string `json:"action"` // start|stop|restart|kill
}

// PowerAckPayload confirms a power action.
type PowerAckPayload struct {
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Status  string `json:"status,omitempty"`
}

// ConsoleOutputPayload streams stdout/stderr from container to panel.
type ConsoleOutputPayload struct {
	Data string `json:"data"`
}

// ConsoleInputPayload sends stdin from panel to container.
type ConsoleInputPayload struct {
	Data string `json:"data"`
}

// StatsPayload is sent by the daemon every 2s per running server.
type StatsPayload struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemoryBytes uint64 `json:"memory_bytes"`
	MemoryLimit uint64 `json:"memory_limit"`
	DiskBytes   uint64 `json:"disk_bytes"`
	NetRxBytes  uint64 `json:"net_rx_bytes"`
	NetTxBytes  uint64 `json:"net_tx_bytes"`
	UptimeSecs  int64  `json:"uptime_secs"`
	State       string `json:"state"`
}

// File operations.
const (
	FileList     = "list"
	FileRead     = "read"
	FileWrite    = "write"
	FileDelete   = "delete"
	FileRename   = "rename"
	FileMkdir    = "mkdir"
	FileCompress = "compress"
)

// FileOpPayload requests a filesystem operation scoped to /home/container.
type FileOpPayload struct {
	Op      string `json:"op"`
	RequestID string `json:"request_id"`
	Path    string `json:"path,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Content string `json:"content,omitempty"` // base64 for write
	Mode    string `json:"mode,omitempty"`
}

// FileEntry describes a single file.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mtime"`
	IsDir   bool   `json:"is_dir"`
}

// FileOpResultPayload answers a FileOpPayload (matched by RequestID).
type FileOpResultPayload struct {
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	Entries   []FileEntry `json:"entries,omitempty"`
	Content   string      `json:"content,omitempty"` // base64 for read
}
