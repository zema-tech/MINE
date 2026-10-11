package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// ServerSpec describes a game server container.
type ServerSpec struct {
	UUID     string
	Name     string
	Image    string
	Startup  string // shell command run inside container
	MemoryMB int64
	CPULimit int64 // percent of 1 core *100 (100 = 1 core)
	DiskMB   int64
	Env      []string
	DataDir  string // host path mounted at /home/container
}

// Manager wraps the Docker SDK with per-server lifecycle ops.
type Manager struct {
	cli *client.Client
	mu  sync.Mutex
}

// New connects to the local Docker socket.
func New() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{cli: cli}, nil
}

func containerName(uuid string) string { return "mine-" + uuid }

// EnsureImage pulls image if missing.
func (m *Manager) EnsureImage(ctx context.Context, image string, out func(string)) error {
	_, _, err := m.cli.ImageInspectWithRaw(ctx, image)
	if err == nil {
		return nil
	}
	rc, err := m.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	buf := make([]byte, 4096)
	for {
		n, rerr := rc.Read(buf)
		if n > 0 && out != nil {
			out(string(buf[:n]))
		}
		if rerr != nil {
			break
		}
	}
	return nil
}

// Create creates (or replaces) the server container with hardening.
func (m *Manager) Create(ctx context.Context, spec ServerSpec) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := containerName(spec.UUID)
	_ = m.removeIfExists(ctx, name)

	memBytes := spec.MemoryMB * 1024 * 1024
	nanoCPUs := spec.CPULimit * 1e7 // 100 -> 1e9 = 1 CPU

	resp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image:      spec.Image,
			Cmd:        []string{"sh", "-c", spec.Startup},
			WorkingDir: "/home/container",
			Env:        spec.Env,
			User:       "1000:1000",
			Labels:     map[string]string{"mine.server": spec.UUID, "mine.managed": "true"},
		},
		&container.HostConfig{
			 Mounts: []mount.Mount{
				{Type: mount.TypeBind, Source: spec.DataDir, Target: "/home/container"},
			},
			Memory:          memBytes,
			NanoCPUs:        nanoCPUs,
			CapDrop:         []string{"ALL"},
			SecurityOpt:     []string{"no-new-privileges"},
			ReadonlyRootfs:  false, // game servers need writable root for now
			AutoRemove:      false,
			RestartPolicy:   container.RestartPolicy{Name: "no"},
			NetworkMode:     "bridge",
		}, nil, nil, name)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (m *Manager) removeIfExists(ctx context.Context, name string) error {
	// Best effort remove by name filter.
	list, err := m.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range list {
		for _, n := range c.Names {
			if strings.TrimPrefix(n, "/") == name {
				timeout := 5
				_ = m.cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
				_ = m.cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
			}
		}
	}
	return nil
}

// Start boots the container.
func (m *Manager) Start(ctx context.Context, uuid string) error {
	return m.cli.ContainerStart(ctx, containerName(uuid), types.ContainerStartOptions{})
}

// Stop gracefully stops (10s timeout), then kills.
func (m *Manager) Stop(ctx context.Context, uuid string) error {
	timeout := 10
	if err := m.cli.ContainerStop(ctx, containerName(uuid), container.StopOptions{Timeout: &timeout}); err != nil {
		return m.Kill(ctx, uuid)
	}
	return nil
}

// Kill force-stops.
func (m *Manager) Kill(ctx context.Context, uuid string) error {
	return m.cli.ContainerKill(ctx, containerName(uuid), "KILL")
}

// Remove deletes the container.
func (m *Manager) Remove(ctx context.Context, uuid string) error {
	return m.cli.ContainerRemove(ctx, containerName(uuid), types.ContainerRemoveOptions{Force: true})
}

// Status returns running|exited|created|missing.
func (m *Manager) Status(ctx context.Context, uuid string) string {
	info, err := m.cli.ContainerInspect(ctx, containerName(uuid))
	if err != nil {
		return "missing"
	}
	if info.State.Running {
		return "running"
	}
	return info.State.Status
}

// Attach streams stdout/stderr; calls onData per chunk. Blocks until ctx done.
func (m *Manager) Attach(ctx context.Context, uuid string, onData func([]byte)) error {
	resp, err := m.cli.ContainerAttach(ctx, containerName(uuid),
		types.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		return err
	}
	defer resp.Close()
	_, err = stdcopy.StdCopy(&writerFunc{onData}, noopWriter{}, resp.Reader)
	return err
}

// ExecStdin writes input to the container's main process via exec.
func (m *Manager) ExecStdin(ctx context.Context, uuid, input string) error {
	exec, err := m.cli.ContainerExecCreate(ctx, containerName(uuid),
		types.ExecConfig{AttachStdin: true, Tty: false, Cmd: []string{"sh", "-c", fmt.Sprintf("echo %q > /proc/1/fd/0 || true", input)}})
	if err != nil {
		// Fallback: containers with TTY — attach stdin is handled by console package.
		return err
	}
	return m.cli.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
}

// StatsSnapshot returns CPU%, memory, net for the container.
func (m *Manager) StatsSnapshot(ctx context.Context, uuid string) (cpuPct float64, memBytes, memLimit, rx, tx uint64, err error) {
	resp, err := m.cli.ContainerStatsOneShot(ctx, containerName(uuid))
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	defer resp.Body.Close()
	var s containerStats
	if derr := decodeStats(resp.Body, &s); derr != nil {
		return 0, 0, 0, 0, 0, derr
	}
	cpuPct = calcCPU(s)
	memBytes = s.MemoryStats.Usage
	memLimit = s.MemoryStats.Limit
	for _, n := range s.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return cpuPct, memBytes, memLimit, rx, tx, nil
}

// DiskUsage returns bytes used under dataDir (du).
func DiskUsage(dataDir string) uint64 {
	var total uint64
	_ = walkDir(dataDir, &total)
	return total
}

type writerFunc struct{ f func([]byte) }

func (w *writerFunc) Write(p []byte) (int, error) { w.f(append([]byte(nil), p...)); return len(p), nil }

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// WaitExit blocks until container exits; returns exit code.
func (m *Manager) WaitExit(ctx context.Context, uuid string) (int64, error) {
	ch, errCh := m.cli.ContainerWait(ctx, containerName(uuid), container.WaitConditionNotRunning)
	select {
	case res := <-ch:
		return res.StatusCode, nil
	case err := <-errCh:
		return -1, err
	case <-ctx.Done():
		return -1, ctx.Err()
	case <-time.After(24 * time.Hour):
		return -1, fmt.Errorf("wait timeout")
	}
}

var _ = io.Discard
