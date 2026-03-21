//go:build !windows

package mount

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/winfsp/cgofuse/fuse"

	"r2go/internal/s3client"
)

type Mount struct {
	Account  string
	Bucket   string
	Root     string
	Path     string
	ReadOnly bool
}

type Manager struct{}

func NewManager() *Manager { return &Manager{} }

func (m *Manager) Mounts() []*Mount { return nil }

func (m *Manager) Find(account, bucket string) *Mount { return nil }

func (m *Manager) Mount(client *s3client.Client, account, bucket, root, target string, readOnly bool) (*Mount, error) {
	return nil, fmt.Errorf("mounting is not supported on this platform yet")
}

func (m *Manager) Unmount(account, bucket string) error {
	return fmt.Errorf("mounting is not supported on this platform yet")
}

func (m *Manager) UnmountAll() {}

func WinFspInstalled() bool { return false }

func DefaultMountTarget(bucket string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "CloudMounts", bucket)
	}
	return filepath.Join(home, "CloudMounts", bucket)
}

var _ = fuse.NewFileSystemHost
