package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/99designs/keyring"
)

const (
	keyringService = "r2go"
	secretFileName = "secrets.bin"
)

type Secrets struct {
	ring     keyring.Keyring
	fallback string
}

func OpenSecrets(dir string) (*Secrets, error) {
	s := &Secrets{fallback: filepath.Join(dir, secretFileName)}
	ring, err := keyring.Open(keyring.Config{
		ServiceName:              keyringService,
		AllowedBackends:          []keyring.BackendType{keyring.WinCredBackend, keyring.KeychainBackend, keyring.SecretServiceBackend, keyring.KWalletBackend},
		KeychainName:             "r2go",
		KeychainTrustApplication: true,
	})
	if err == nil {
		s.ring = ring
	}
	return s, nil
}

// Location says where secrets are kept: the OS keyring, or the fallback file.
func (s *Secrets) Location() string {
	if s.ring != nil {
		switch runtime.GOOS {
		case "windows":
			return "Windows Credential Manager"
		case "darwin":
			return "macOS Keychain"
		default:
			return "system keyring"
		}
	}
	return s.fallback
}

func s3Key(name string) string       { return "account:" + name + ":s3-secret" }
func legacyS3Key(name string) string { return "account:" + name + ":secret" }
func apiKey(name string) string      { return "account:" + name + ":cf-api-token" }

func (s *Secrets) SetS3Secret(name, value string) error { return s.set(s3Key(name), value) }
func (s *Secrets) SetAPIToken(name, value string) error { return s.set(apiKey(name), value) }

func (s *Secrets) S3Secret(name string) (string, bool) {
	if value, ok := s.get(s3Key(name)); ok {
		return value, true
	}
	// v1 migration: old builds stored the S3 secret under :secret.
	if value, ok := s.get(legacyS3Key(name)); ok {
		_ = s.set(s3Key(name), value)
		return value, true
	}
	return "", false
}

func (s *Secrets) APIToken(name string) (string, bool) { return s.get(apiKey(name)) }

func (s *Secrets) RemoveAPIToken(name string) error { return s.remove(apiKey(name)) }

func (s *Secrets) RemoveAccount(name string) error {
	_ = s.remove(s3Key(name))
	_ = s.remove(legacyS3Key(name))
	_ = s.remove(apiKey(name))
	return nil
}

func (s *Secrets) Rename(oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	if value, ok := s.S3Secret(oldName); ok {
		if err := s.SetS3Secret(newName, value); err != nil {
			return err
		}
	}
	if value, ok := s.APIToken(oldName); ok {
		if err := s.SetAPIToken(newName, value); err != nil {
			return err
		}
	}
	return s.RemoveAccount(oldName)
}

func (s *Secrets) set(key, value string) error {
	if s.ring != nil {
		if err := s.ring.Set(keyring.Item{Key: key, Data: []byte(value)}); err != nil {
			return err
		}
		// If an earlier run had to use the plaintext fallback, remove the
		// migrated value after a successful native-keyring write.
		m, err := s.readFallback()
		if err == nil {
			if _, ok := m[key]; ok {
				delete(m, key)
				_ = s.writeFallback(m)
			}
		}
		return nil
	}
	m, err := s.readFallback()
	if err != nil {
		return err
	}
	m[key] = value
	return s.writeFallback(m)
}

func (s *Secrets) get(key string) (string, bool) {
	if s.ring != nil {
		item, err := s.ring.Get(key)
		if err == nil {
			return string(item.Data), true
		}
		// If a previous run used the file fallback, migrate it into the keyring.
		m, ferr := s.readFallback()
		if ferr == nil {
			if value, ok := m[key]; ok {
				if err := s.ring.Set(keyring.Item{Key: key, Data: []byte(value)}); err == nil {
					delete(m, key)
					_ = s.writeFallback(m)
				}
				return value, true
			}
		}
		return "", false
	}
	m, err := s.readFallback()
	if err != nil {
		return "", false
	}
	value, ok := m[key]
	return value, ok
}

func (s *Secrets) remove(key string) error {
	if s.ring != nil {
		_ = s.ring.Remove(key)
	}
	m, err := s.readFallback()
	if err != nil {
		return err
	}
	delete(m, key)
	return s.writeFallback(m)
}

func (s *Secrets) readFallback() (map[string]string, error) {
	b, err := os.ReadFile(s.fallback)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return nil, fmt.Errorf("decode fallback secrets: %w", err)
	}
	m := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (s *Secrets) writeFallback(m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(b)
	return os.WriteFile(s.fallback, []byte(encoded), 0o600)
}
