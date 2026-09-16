// Package groups хранит именованные наборы устройств ("группы"),
// которые пользователь может один раз создать в интерфейсе и потом
// быстро подставлять в список опроса, не вводя адреса заново.
package groups

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// Device - устройство внутри сохраненной группы
type Device struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Group - именованный набор устройств
type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Devices []Device `json:"devices"`
}

// Store читает и пишет группы в JSON-файл на диске
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

// List возвращает все сохраненные группы, отсортированные по имени
func (s *Store) List() ([]Group, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadLocked()

	if err != nil {
		return nil, err
	}

	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	return list, nil
}

// Save создает новую группу (если g.ID пуст) или обновляет существующую
func (s *Store) Save(
	g Group,
) (Group, error) {

	if strings.TrimSpace(g.Name) == "" {
		return Group{}, errors.New("имя группы не может быть пустым")
	}

	if len(g.Devices) == 0 {
		return Group{}, errors.New("в группе должно быть хотя бы одно устройство")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadLocked()

	if err != nil {
		return Group{}, err
	}

	if g.ID == "" {

		g.ID = uuid.NewString()

		list = append(list, g)

	} else {

		found := false

		for i := range list {

			if list[i].ID == g.ID {

				list[i] = g
				found = true

				break
			}
		}

		if !found {
			list = append(list, g)
		}
	}

	if err := s.persistLocked(list); err != nil {
		return Group{}, err
	}

	return g, nil
}

// Delete удаляет группу по ID. Отсутствие группы с таким ID не считается ошибкой
func (s *Store) Delete(
	id string,
) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.loadLocked()

	if err != nil {
		return err
	}

	out := list[:0]

	for _, g := range list {

		if g.ID != id {
			out = append(out, g)
		}
	}

	return s.persistLocked(out)
}

func (s *Store) loadLocked() ([]Group, error) {

	data, err := os.ReadFile(s.path)

	if err != nil {

		if os.IsNotExist(err) {
			return []Group{}, nil
		}

		return nil, err
	}

	if len(data) == 0 {
		return []Group{}, nil
	}

	var list []Group

	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return list, nil
}

func (s *Store) persistLocked(
	list []Group,
) error {

	if list == nil {
		list = []Group{}
	}

	data, err := json.MarshalIndent(list, "", "  ")

	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, s.path)
}
