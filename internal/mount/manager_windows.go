//go:build windows

package mount

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/winfsp/cgofuse/fuse"
	"golang.org/x/sys/windows/registry"

	"r2go/internal/s3client"
)

type Mount struct {
	Account  string
	Bucket   string
	Root     string
	Path     string
	ReadOnly bool

	host *fuse.FileSystemHost
	fs   *FS
}

type Manager struct {
	mu     map[string]*Mount
	cache  string
}

func NewManager() *Manager {
	return &Manager{mu: map[string]*Mount{}}
}

func (m *Manager) Mounts() []*Mount {
	out := make([]*Mount, 0, len(m.mu))
	for _, mo := range m.mu {
		out = append(out, mo)
	}
	return out
}

func (m *Manager) Find(account, bucket string) *Mount {
	return m.mu[account+"/"+bucket]
}

func (m *Manager) Mount(client *s3client.Client, account, bucket, root, target string, readOnly bool) (*Mount, error) {
	if _, ok := m.mu[account+"/"+bucket]; ok {
		return nil, fmt.Errorf("already mounted: %s/%s", account, bucket)
	}
	if !WinFspInstalled() {
		return nil, fmt.Errorf("WinFsp is not installed. Install it with:\nwinget install WinFsp.WinFsp\nor from https://winfsp.dev")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, fmt.Errorf("create mount point: %w", err)
	}
	fs := NewFS(client, bucket, root, readOnly)
	host := fuse.NewFileSystemHost(fs)
	if !host.Mount(target, nil) {
		return nil, fmt.Errorf("mount failed for %s", target)
	}
	mo := &Mount{
		Account:  account,
		Bucket:   bucket,
		Root:     root,
		Path:     target,
		ReadOnly: readOnly,
		host:     host,
		fs:       fs,
	}
	m.mu[account+"/"+bucket] = mo
	return mo, nil
}

func (m *Manager) Unmount(account, bucket string) error {
	mo := m.mu[account+"/"+bucket]
	if mo == nil {
		return fmt.Errorf("not mounted: %s/%s", account, bucket)
	}
	if !mo.host.Unmount() {
		return fmt.Errorf("unmount failed for %s", mo.Path)
	}
	delete(m.mu, account+"/"+bucket)
	return nil
}

func (m *Manager) UnmountAll() {
	for k := range m.mu {
		mo := m.mu[k]
		mo.host.Unmount()
		delete(m.mu, k)
	}
}

func WinFspInstalled() bool {
	for _, path := range []string{
		`SOFTWARE\WinFsp`,
		`SOFTWARE\WOW6432Node\WinFsp`,
	} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
		if err == nil {
			k.Close()
			return true
		}
	}
	return false
}

func DefaultMountTarget(bucket string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "CloudMounts", bucket)
	}
	return filepath.Join(home, "CloudMounts", bucket)
}
