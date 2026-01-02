package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configFileName = "config.json"
	defaultTheme   = "dark"
	defaultConcurr = 4
)

type Account struct {
	Name        string   `json:"name"`
	AccountID   string   `json:"account_id"`
	AccessKeyID string   `json:"access_key_id"`
	Buckets     []string `json:"buckets,omitempty"`
}

type Settings struct {
	Theme       string `json:"theme"`
	DownloadDir string `json:"download_dir"`
	Concurrency int    `json:"concurrency"`
}

type file struct {
	Version  int       `json:"version"`
	Settings Settings  `json:"settings"`
	Accounts []Account `json:"accounts"`
}

type Manager struct {
	path string
	data file
	sec  *secrets
}

func New() (*Manager, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	dir = filepath.Join(dir, "r2go")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	m := &Manager{
		path: filepath.Join(dir, configFileName),
		data: file{
			Version: 1,
			Settings: Settings{
				Theme:       defaultTheme,
				DownloadDir: defaultDownloadDir(),
				Concurrency: defaultConcurr,
			},
		},
	}
	m.sec, err = openSecrets(dir)
	if err != nil {
		return nil, err
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) load() error {
	b, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return m.Save()
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if len(b) == 0 {
		return m.Save()
	}
	if err := json.Unmarshal(b, &m.data); err != nil {
		return fmt.Errorf("parse config %s: %w", m.path, err)
	}
	if m.data.Settings.DownloadDir == "" {
		m.data.Settings.DownloadDir = defaultDownloadDir()
	}
	if m.data.Settings.Concurrency <= 0 {
		m.data.Settings.Concurrency = defaultConcurr
	}
	if m.data.Settings.Theme == "" {
		m.data.Settings.Theme = defaultTheme
	}
	return nil
}

func (m *Manager) Save() error {
	b, err := json.MarshalIndent(&m.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

func (m *Manager) Settings() *Settings {
	return &m.data.Settings
}

func (m *Manager) Accounts() []Account {
	out := make([]Account, len(m.data.Accounts))
	copy(out, m.data.Accounts)
	return out
}

func (m *Manager) FindAccount(name string) (Account, bool) {
	for _, a := range m.data.Accounts {
		if a.Name == name {
			return a, true
		}
	}
	return Account{}, false
}

func (m *Manager) AddAccount(a Account, secret string) error {
	if a.Name == "" || a.AccountID == "" || a.AccessKeyID == "" {
		return errors.New("name, account id and access key id are required")
	}
	if _, ok := m.FindAccount(a.Name); ok {
		return fmt.Errorf("account %q already exists", a.Name)
	}
	if err := m.sec.Set(a.Name, secret); err != nil {
		return err
	}
	m.data.Accounts = append(m.data.Accounts, a)
	return m.Save()
}

func (m *Manager) UpdateAccount(name string, a Account, secret string, setSecret bool) error {
	if a.Name == "" || a.AccountID == "" || a.AccessKeyID == "" {
		return errors.New("name, account id and access key id are required")
	}
	if name != a.Name {
		if _, ok := m.FindAccount(a.Name); ok {
			return fmt.Errorf("account %q already exists", a.Name)
		}
	}
	for i, cur := range m.data.Accounts {
		if cur.Name == name {
			if setSecret {
				if err := m.sec.Set(a.Name, secret); err != nil {
					return err
				}
			} else if name != a.Name {
				if err := m.sec.Rename(name, a.Name); err != nil {
					return err
				}
			}
			m.data.Accounts[i] = a
			return m.Save()
		}
	}
	return fmt.Errorf("account %q not found", name)
}

func (m *Manager) DeleteAccount(name string) error {
	for i, a := range m.data.Accounts {
		if a.Name == name {
			m.data.Accounts = append(m.data.Accounts[:i], m.data.Accounts[i+1:]...)
			_ = m.sec.Remove(name)
			return m.Save()
		}
	}
	return fmt.Errorf("account %q not found", name)
}

func (m *Manager) GetSecret(name string) (string, bool) {
	return m.sec.Get(name)
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	d := filepath.Join(home, "Downloads")
	if _, err := os.Stat(d); err == nil {
		return d
	}
	return home
}
