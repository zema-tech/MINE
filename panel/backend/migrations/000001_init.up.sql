-- MINE initial schema (golang-migrate)
-- Mirrors models in panel/backend/internal/models.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  username VARCHAR(64) NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  totp_secret TEXT NOT NULL DEFAULT '',
  totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  role VARCHAR(16) NOT NULL DEFAULT 'client',
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  max_servers INT NOT NULL DEFAULT 5,
  max_memory_mb INT NOT NULL DEFAULT 4096,
  max_disk_mb INT NOT NULL DEFAULT 20480,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
  id SERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  fqdn VARCHAR(255) NOT NULL,
  daemon_port INT NOT NULL DEFAULT 8081,
  token TEXT NOT NULL,
  memory_mb INT NOT NULL DEFAULT 0,
  disk_mb INT NOT NULL DEFAULT 0,
  location VARCHAR(64) NOT NULL DEFAULT '',
  online BOOLEAN NOT NULL DEFAULT FALSE,
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS templates (
  id SERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  game VARCHAR(64) NOT NULL DEFAULT '',
  docker_image VARCHAR(255) NOT NULL,
  startup_command TEXT NOT NULL,
  config_schema JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS servers (
  id SERIAL PRIMARY KEY,
  uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  name VARCHAR(128) NOT NULL,
  owner_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  node_id INT NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
  template_id INT NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
  memory_mb INT NOT NULL DEFAULT 1024,
  disk_mb INT NOT NULL DEFAULT 5120,
  cpu_limit INT NOT NULL DEFAULT 100,
  status VARCHAR(16) NOT NULL DEFAULT 'offline',
  variables JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_servers_owner ON servers(owner_id);
CREATE INDEX IF NOT EXISTS idx_servers_node ON servers(node_id);

CREATE TABLE IF NOT EXISTS allocations (
  id SERIAL PRIMARY KEY,
  node_id INT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  ip VARCHAR(64) NOT NULL,
  port INT NOT NULL,
  server_id INT REFERENCES servers(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(node_id, ip, port)
);

CREATE TABLE IF NOT EXISTS sub_users (
  id SERIAL PRIMARY KEY,
  server_id INT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permissions JSONB NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(server_id, user_id)
);

CREATE TABLE IF NOT EXISTS activity_logs (
  id SERIAL PRIMARY KEY,
  user_id INT REFERENCES users(id) ON DELETE SET NULL,
  server_id INT REFERENCES servers(id) ON DELETE SET NULL,
  action VARCHAR(64) NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  ip_address VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_activity_server ON activity_logs(server_id);
CREATE INDEX IF NOT EXISTS idx_activity_created ON activity_logs(created_at);
