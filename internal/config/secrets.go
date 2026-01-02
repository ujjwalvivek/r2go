package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
)

const (
	keyringService = "r2go"
	secretFileName = "secrets.bin"
)

type secrets struct {
	ring     keyring.Keyring
	fallback string
}

func openSecrets(dir string) (*secrets, error) {
	s := &secrets{fallback: filepath.Join(dir, secretFileName)}
	ring, err := keyring.Open(keyring.Config{
		ServiceName:                 keyringService,
		AllowedBackends:             []keyring.BackendType{keyring.WinCredBackend, keyring.KeychainBackend, keyring.SecretServiceBackend, keyring.KWalletBackend},
		KeychainName:                "r2go",
		KeychainTrustApplication:    true,
	})
	if err == nil {
		s.ring = ring
	}
	return s, nil
}

func (s *secrets) key(name string) string {
	return "account:" + name + ":secret"
}

func (s *secrets) Set(name, secret string) error {
	m, err := s.readFallback()
	if err != nil {
		return err
	}
	m[s.key(name)] = secret
	if s.ring != nil {
		return s.ring.Set(keyring.Item{Key: s.key(name), Data: []byte(secret)})
	}
	return s.writeMap(m)
}

func (s *secrets) Get(name string) (string, bool) {
	if s.ring != nil {
		item, err := s.ring.Get(s.key(name))
		if err != nil {
			return "", false
		}
		return string(item.Data), true
	}
	m, err := s.readFallback()
	if err != nil {
		return "", false
	}
	v, ok := m[s.key(name)]
	return v, ok
}

func (s *secrets) Remove(name string) error {
	m, err := s.readFallback()
	if err != nil {
		return err
	}
	delete(m, s.key(name))
	if s.ring != nil {
		return s.ring.Remove(s.key(name))
	}
	return s.writeMap(m)
}

func (s *secrets) Rename(old, new string) error {
	if v, ok := s.Get(old); ok {
		_ = s.Remove(old)
		return s.Set(new, v)
	}
	return nil
}

func (s *secrets) readFallback() (map[string]string, error) {
	b, err := os.ReadFile(s.fallback)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return nil, fmt.Errorf("decode secrets: %w", err)
	}
	m := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (s *secrets) writeMap(m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.fallback, []byte(base64.StdEncoding.EncodeToString(b)), 0o600)
}

func (s *secrets) writeFallback() error {
	m, err := s.readFallback()
	if err != nil {
		return err
	}
	return s.writeMap(m)
}
