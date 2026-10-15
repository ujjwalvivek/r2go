package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const configFileName = "config.json"

type Account struct {
	Name          string            `json:"name"`
	AccountID     string            `json:"account_id"`
	AccessKeyID   string            `json:"access_key_id"`
	Buckets       []string          `json:"buckets,omitempty"`
	PublicDomains map[string]string `json:"public_domains,omitempty"`
}

type Settings struct {
	DownloadDir string `json:"download_dir"`
	// View is "list" or "grid".
	View string `json:"view,omitempty"`
	// HideDetails hides the details panel; it is shown by default.
	HideDetails bool `json:"hide_details,omitempty"`
	// NoThumbnails stops image previews from being downloaded.
	NoThumbnails bool `json:"no_thumbnails,omitempty"`
}

type diskFile struct {
	Version  int       `json:"version"`
	Settings Settings  `json:"settings"`
	Accounts []Account `json:"accounts"`
}

type Manager struct {
	mu     sync.RWMutex
	saveMu sync.Mutex
	path   string
	data   diskFile
	sec    *Secrets
}

func New() (*Manager, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config directory: %w", err)
	}
	dir = filepath.Join(dir, "r2go")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}

	secrets, err := OpenSecrets(dir)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		path: filepath.Join(dir, configFileName),
		sec:  secrets,
		data: diskFile{
			Version: 2,
			Settings: Settings{
				DownloadDir: defaultDownloadDir(),
			},
		},
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
		return fmt.Errorf("parse %s: %w", m.path, err)
	}
	m.data.Version = 2
	if m.data.Settings.DownloadDir == "" {
		m.data.Settings.DownloadDir = defaultDownloadDir()
	}
	for i := range m.data.Accounts {
		if m.data.Accounts[i].PublicDomains == nil {
			m.data.Accounts[i].PublicDomains = map[string]string{}
		}
	}
	return nil
}

func (m *Manager) Save() error {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	m.mu.RLock()
	b, err := json.MarshalIndent(&m.data, "", "  ")
	path := m.path
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Dir is the folder holding config.json.
func (m *Manager) Dir() string { return filepath.Dir(m.path) }

// SecretsLocation says where credentials are stored, for display.
func (m *Manager) SecretsLocation() string { return m.sec.Location() }

func (m *Manager) Settings() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data.Settings
}

func (m *Manager) UpdateSettings(settings Settings) error {
	if settings.DownloadDir == "" {
		settings.DownloadDir = defaultDownloadDir()
	}
	m.mu.Lock()
	m.data.Settings = settings
	m.mu.Unlock()
	return m.Save()
}

func cloneAccount(account Account) Account {
	copy := account
	copy.Buckets = append([]string(nil), account.Buckets...)
	if account.PublicDomains != nil {
		copy.PublicDomains = make(map[string]string, len(account.PublicDomains))
		for bucket, domain := range account.PublicDomains {
			copy.PublicDomains[bucket] = domain
		}
	}
	return copy
}

func (m *Manager) Accounts() []Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Account, 0, len(m.data.Accounts))
	for _, account := range m.data.Accounts {
		out = append(out, cloneAccount(account))
	}
	return out
}

func (m *Manager) FindAccount(name string) (Account, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, account := range m.data.Accounts {
		if account.Name == name {
			return cloneAccount(account), true
		}
	}
	return Account{}, false
}

func normalizeAccount(account Account) Account {
	account.Name = strings.TrimSpace(account.Name)
	account.AccountID = strings.TrimSpace(account.AccountID)
	account.AccessKeyID = strings.TrimSpace(account.AccessKeyID)
	return account
}

func validateAccount(account Account) error {
	if account.Name == "" || account.AccountID == "" || account.AccessKeyID == "" {
		return errors.New("name, Account ID and Access Key ID are required")
	}
	return nil
}

func (m *Manager) AddAccount(account Account, s3Secret, apiToken string) error {
	account = normalizeAccount(account)
	if err := validateAccount(account); err != nil {
		return err
	}
	if _, ok := m.FindAccount(account.Name); ok {
		return fmt.Errorf("account %q already exists", account.Name)
	}
	account.PublicDomains = map[string]string{}
	if err := m.sec.SetS3Secret(account.Name, s3Secret); err != nil {
		return err
	}
	if apiToken != "" {
		if err := m.sec.SetAPIToken(account.Name, apiToken); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.data.Accounts = append(m.data.Accounts, account)
	m.mu.Unlock()
	return m.Save()
}

func (m *Manager) UpdateAccount(oldName string, account Account, s3Secret, apiToken string, replaceS3, replaceAPI bool) error {
	oldName = strings.TrimSpace(oldName)
	account = normalizeAccount(account)
	if err := validateAccount(account); err != nil {
		return err
	}
	if oldName != account.Name {
		if _, ok := m.FindAccount(account.Name); ok {
			return fmt.Errorf("account %q already exists", account.Name)
		}
	}

	m.mu.RLock()
	idx := -1
	for i := range m.data.Accounts {
		if m.data.Accounts[i].Name == oldName {
			idx = i
			break
		}
	}
	m.mu.RUnlock()
	if idx < 0 {
		return fmt.Errorf("account %q not found", oldName)
	}

	if oldName != account.Name {
		if err := m.sec.Rename(oldName, account.Name); err != nil {
			return err
		}
	}
	if replaceS3 {
		if err := m.sec.SetS3Secret(account.Name, s3Secret); err != nil {
			return err
		}
	}
	if replaceAPI {
		if apiToken == "" {
			_ = m.sec.RemoveAPIToken(account.Name)
		} else if err := m.sec.SetAPIToken(account.Name, apiToken); err != nil {
			return err
		}
	}

	m.mu.Lock()
	// Bucket cache and public-domain mapping are application-managed fields, not
	// editable account identity. Preserve the newest values in case a bucket
	// refresh completed while the account dialog was open.
	if idx >= len(m.data.Accounts) || m.data.Accounts[idx].Name != oldName {
		m.mu.Unlock()
		return fmt.Errorf("account %q changed while it was being edited", oldName)
	}
	account.Buckets = append([]string(nil), m.data.Accounts[idx].Buckets...)
	account.PublicDomains = cloneAccount(m.data.Accounts[idx]).PublicDomains
	m.data.Accounts[idx] = account
	m.mu.Unlock()
	return m.Save()
}

func (m *Manager) DeleteAccount(name string) error {
	m.mu.Lock()
	idx := -1
	for i := range m.data.Accounts {
		if m.data.Accounts[i].Name == name {
			idx = i
			break
		}
	}
	if idx >= 0 {
		m.data.Accounts = append(m.data.Accounts[:idx], m.data.Accounts[idx+1:]...)
	}
	m.mu.Unlock()
	if idx < 0 {
		return fmt.Errorf("account %q not found", name)
	}
	_ = m.sec.RemoveAccount(name)
	return m.Save()
}

func (m *Manager) S3Secret(name string) (string, bool) { return m.sec.S3Secret(name) }
func (m *Manager) APIToken(name string) (string, bool) { return m.sec.APIToken(name) }

func (m *Manager) CacheBuckets(accountName string, buckets []string) error {
	buckets = append([]string(nil), buckets...)
	sort.Strings(buckets)
	m.mu.Lock()
	found := false
	for i := range m.data.Accounts {
		if m.data.Accounts[i].Name == accountName {
			m.data.Accounts[i].Buckets = buckets
			found = true
			break
		}
	}
	m.mu.Unlock()
	if !found {
		return fmt.Errorf("account %q not found", accountName)
	}
	return m.Save()
}

func (m *Manager) PublicDomain(accountName, bucket string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, account := range m.data.Accounts {
		if account.Name == accountName {
			return account.PublicDomains[bucket]
		}
	}
	return ""
}

func (m *Manager) SetPublicDomain(accountName, bucket, domain string) error {
	domain, err := normalizePublicDomain(domain)
	if err != nil {
		return err
	}

	m.mu.Lock()
	found := false
	for i := range m.data.Accounts {
		if m.data.Accounts[i].Name != accountName {
			continue
		}
		found = true
		if m.data.Accounts[i].PublicDomains == nil {
			m.data.Accounts[i].PublicDomains = map[string]string{}
		}
		if domain == "" {
			delete(m.data.Accounts[i].PublicDomains, bucket)
		} else {
			m.data.Accounts[i].PublicDomains[bucket] = domain
		}
		break
	}
	m.mu.Unlock()
	if !found {
		return fmt.Errorf("account %q not found", accountName)
	}
	return m.Save()
}

func normalizePublicDomain(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("enter a valid public domain, for example cdn.example.com")
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("public domain must use https")
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("enter a domain only, without a path, query, or credentials")
	}
	return parsed.Host, nil
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	downloads := filepath.Join(home, "Downloads")
	if _, err := os.Stat(downloads); err == nil {
		return downloads
	}
	return home
}
