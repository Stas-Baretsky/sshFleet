// Package connsettings хранит настройки SSH-подключения и исполнителя
// (логин, аутентификация, pty, тайминги, параллелизм), которые в
// десктоп-приложении задаются формой в UI, а не файлом config.yaml
package connsettings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ssh/internal/config"
)

// Settings - настройки SSH-подключения в виде, удобном для JSON
// (секунды числом, а не time.Duration) и формы в UI
type Settings struct {
	User string `json:"user"`

	// Порт по умолчанию для устройств, у которых он не задан явно
	Port int `json:"port"`

	TimeoutSeconds int `json:"timeoutSeconds"`

	KnownHosts string `json:"knownHosts"`

	StrictHostKeyChecking bool `json:"strictHostKeyChecking"`

	//
	// PTY - интерактивный shell-режим (нужен, например, для Eltex,
	// чтобы попадать в configure terminal), см. internal/ssh.RunShell
	//
	PTY bool `json:"pty"`

	IdleTimeoutSeconds float64 `json:"idleTimeoutSeconds"`

	// AuthType: "key" | "password"
	AuthType string `json:"authType"`

	PrivateKey string `json:"privateKey"`

	Passphrase string `json:"passphrase"`

	Password string `json:"password"`

	Workers int `json:"workers"`

	RetryAttempts int `json:"retryAttempts"`

	RetryDelaySeconds float64 `json:"retryDelaySeconds"`
}

// Default возвращает разумные значения по умолчанию для первого запуска
func Default() Settings {

	return Settings{
		Port:                  22,
		TimeoutSeconds:        10,
		StrictHostKeyChecking: false,
		PTY:                   true,
		IdleTimeoutSeconds:    2,
		AuthType:              "key",
		Workers:               10,
		RetryAttempts:         3,
		RetryDelaySeconds:     2,
	}
}

// Validate проверяет, что настроек достаточно для подключения
func (s Settings) Validate() error {

	if strings.TrimSpace(s.User) == "" {
		return fmt.Errorf("не указан пользователь SSH")
	}

	switch s.AuthType {

	case "key":

		if strings.TrimSpace(s.PrivateKey) == "" {
			return fmt.Errorf("не указан путь к приватному ключу")
		}

	case "password":

		if s.Password == "" {
			return fmt.Errorf("не указан пароль")
		}

	default:

		return fmt.Errorf("неизвестный тип аутентификации: %q", s.AuthType)
	}

	if s.StrictHostKeyChecking && strings.TrimSpace(s.KnownHosts) == "" {
		return fmt.Errorf("для строгой проверки ключа хоста нужно указать known_hosts")
	}

	if s.Workers <= 0 {
		return fmt.Errorf("количество параллельных подключений должно быть больше нуля")
	}

	if s.RetryAttempts < 0 {
		return fmt.Errorf("число повторов подключения не может быть отрицательным")
	}

	return nil
}

// SSHConfig конвертирует настройки в config.SSHConfig, используемый
// внутренними пакетами internal/ssh и internal/executor
func (s Settings) SSHConfig() config.SSHConfig {

	return config.SSHConfig{
		User:                  s.User,
		Timeout:               time.Duration(s.TimeoutSeconds) * time.Second,
		KnownHosts:            expandPath(s.KnownHosts),
		StrictHostKeyChecking: s.StrictHostKeyChecking,
		PTY:                   s.PTY,
		IdleTimeout:           secondsToDuration(s.IdleTimeoutSeconds),
		Auth: config.AuthConfig{
			Type:       s.AuthType,
			PrivateKey: expandPath(s.PrivateKey),
			Passphrase: s.Passphrase,
			Password:   s.Password,
		},
	}
}

// ExecutorConfig конвертирует настройки в config.ExecutorConfig
func (s Settings) ExecutorConfig() config.ExecutorConfig {

	return config.ExecutorConfig{
		Workers: s.Workers,
		Retry: config.RetryConfig{
			Attempts: s.RetryAttempts,
			Delay:    secondsToDuration(s.RetryDelaySeconds),
		},
	}
}

func secondsToDuration(
	seconds float64,
) time.Duration {

	return time.Duration(seconds * float64(time.Second))
}

// expandPath раскрывает "~" в начале пути до домашней директории
// пользователя (аналог internal/config.expandPath, который не экспортирован)
func expandPath(
	path string,
) string {

	path = strings.TrimSpace(path)

	if path == "" || path[0] != '~' {
		return path
	}

	home, err := os.UserHomeDir()

	if err != nil {
		return path
	}

	if path == "~" {
		return home
	}

	if len(path) > 1 && path[1] == '/' {
		return filepath.Join(home, path[2:])
	}

	return path
}

// Store читает и пишет настройки в JSON-файл на диске.
//
// ВНИМАНИЕ: пароль/пароль-фраза (если выбрана парольная аутентификация)
// хранятся в этом файле в открытом виде. Файл создается с правами 0600
// (доступ только владельцу), но для серьезного использования
// рекомендуется аутентификация по ключу
type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(
	path string,
) *Store {

	return &Store{
		path: path,
	}
}

func (s *Store) Load() (Settings, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)

	if err != nil {

		if os.IsNotExist(err) {
			return Default(), nil
		}

		return Settings{}, err
	}

	if len(data) == 0 {
		return Default(), nil
	}

	var settings Settings

	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}

	return settings, nil
}

func (s *Store) Save(
	settings Settings,
) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")

	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, s.path)
}
