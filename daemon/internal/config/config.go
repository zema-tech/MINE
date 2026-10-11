package config

import "os"

// Config for mined (the node daemon).
type Config struct {
	PanelURL string // ws(s)://panel:8080/api/daemon/ws
	Token    string // node token issued by panel
	DataDir  string // /var/lib/mined — server volumes live here
	Port     string // local health port
}

func Load() Config {
	return Config{
		PanelURL: envOr("MINED_PANEL_URL", "ws://localhost:8080/api/daemon/ws"),
		Token:    firstNonEmpty(os.Getenv("MINED_TOKEN"), readFileEnv("MINED_TOKEN_FILE"), "dev_daemon_token"),
		DataDir:  envOr("MINED_DATA_DIR", "/var/lib/mined"),
		Port:     envOr("MINED_PORT", "8081"),
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func readFileEnv(envKey string) string {
	p := os.Getenv(envKey)
	if p == "" {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}
