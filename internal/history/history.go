// Package history сохраняет результаты каждого запуска на диск:
// по одному текстовому файлу на устройство плюс run.json с метаданными
// (статусы устройств, команды, время), чтобы запуск можно было
// впоследствии открыть и посмотреть в интерфейсе
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ssh/internal/inventory"
	"ssh/internal/output"
	sshclient "ssh/internal/ssh"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// DeviceSummary - текущее состояние одного устройства в рамках запуска
type DeviceSummary struct {
	// ID - идентификатор, безопасный для имени файла и для передачи
	// с фронтенда; именно им адресуется GetDeviceOutput
	ID string `json:"id"`

	Name    string `json:"name"`
	Address string `json:"address"`
	Status  Status `json:"status"`
	Error   string `json:"error,omitempty"`
}

// Meta - метаданные запуска, персистятся в run.json
type Meta struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	StartedAt   time.Time  `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	Cancelled   bool       `json:"cancelled,omitempty"`

	Commands []string        `json:"commands"`
	Devices  []DeviceSummary `json:"devices"`
}

type deviceEntry struct {
	summary DeviceSummary
	file    *os.File
}

// Run - один выполняющийся (или уже завершенный) запуск: директория
// на диске + текущее состояние устройств в памяти
type Run struct {
	mu sync.Mutex

	dir      string
	metaPath string

	meta Meta

	order   []string
	entries map[string]*deviceEntry
	byName  map[string]string

	fmtr *output.TextFormatter
}

// NewRun создает директорию запуска в baseDir, файл run.json
// и возвращает объект для дальнейшей записи результатов
func NewRun(
	baseDir string,
	description string,
	devices []inventory.Device,
	commands []string,
) (*Run, error) {

	if len(devices) == 0 {
		return nil, errors.New("список устройств пуст")
	}

	started := time.Now()

	id := started.Format("20060102-150405") + "_" + slugify(description)

	dir := filepath.Join(baseDir, id)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}

	r := &Run{
		dir:      dir,
		metaPath: filepath.Join(dir, "run.json"),
		entries:  make(map[string]*deviceEntry, len(devices)),
		byName:   make(map[string]string, len(devices)),
		fmtr:     output.NewTextFormatter(),
		meta: Meta{
			ID:          id,
			Description: description,
			StartedAt:   started,
			Commands:    commands,
		},
	}

	usedIDs := map[string]int{}

	for _, d := range devices {

		base := sanitizeID(d.Name)

		usedIDs[base]++

		deviceID := base

		if usedIDs[base] > 1 {
			deviceID = fmt.Sprintf("%s-%d", base, usedIDs[base])
		}

		r.order = append(r.order, deviceID)

		r.entries[deviceID] = &deviceEntry{
			summary: DeviceSummary{
				ID:      deviceID,
				Name:    d.Name,
				Address: d.Address,
				Status:  StatusPending,
			},
		}

		r.byName[d.Name] = deviceID
	}

	r.syncMetaLocked()

	if err := r.persistLocked(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Run) ID() string {
	return r.meta.ID
}

// DeviceIDForName возвращает внутренний ID устройства по имени,
// с которым оно было передано в executor (executor.Result.Host)
func (r *Run) DeviceIDForName(
	name string,
) (string, bool) {

	r.mu.Lock()
	defer r.mu.Unlock()

	id, ok := r.byName[name]

	return id, ok
}

// MarkAllRunning переводит все устройства из pending в running.
// Вызывается один раз перед стартом выполнения
func (r *Run) MarkAllRunning() {

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range r.order {

		e := r.entries[id]

		if e.summary.Status == StatusPending {
			e.summary.Status = StatusRunning
		}
	}

	r.syncMetaLocked()

	_ = r.persistLocked()
}

// RecordResult дописывает результат выполнения одной команды в файл
// устройства и обновляет его статус. Возвращает актуальное состояние
// устройства и false, если id неизвестен
func (r *Run) RecordResult(
	id string,
	res sshclient.Result,
) (DeviceSummary, bool) {

	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[id]

	if !ok {
		return DeviceSummary{}, false
	}

	if e.file == nil {

		f, err := os.OpenFile(
			filepath.Join(r.dir, id+".txt"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0o644,
		)

		if err == nil {
			e.file = f
		}
	}

	if e.file != nil {

		if data, err := r.fmtr.Write(res); err == nil {
			_, _ = e.file.Write(data)
		}
	}

	if res.Error != nil {

		e.summary.Status = StatusError
		e.summary.Error = res.Error.Error()

	} else if e.summary.Status != StatusError {

		e.summary.Status = StatusSuccess
	}

	r.syncMetaLocked()

	_ = r.persistLocked()

	return e.summary, true
}

// MarkDone фиксирует, что для устройства больше не будет результатов
// (вызывается из executor.OnDeviceDone). Если за все выполнение не
// пришло ни одного результата, статус считается ошибкой
func (r *Run) MarkDone(
	id string,
) (DeviceSummary, bool) {

	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[id]

	if !ok {
		return DeviceSummary{}, false
	}

	if e.summary.Status == StatusPending || e.summary.Status == StatusRunning {

		e.summary.Status = StatusError

		if e.summary.Error == "" {
			e.summary.Error = "не получено ни одного результата"
		}
	}

	r.syncMetaLocked()

	_ = r.persistLocked()

	return e.summary, true
}

// Finish закрывает файлы устройств, доводит незавершенные статусы
// до финального состояния и сохраняет run.json в последний раз
func (r *Run) Finish(
	cancelled bool,
) Meta {

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range r.order {

		e := r.entries[id]

		if e.summary.Status == StatusPending || e.summary.Status == StatusRunning {

			e.summary.Status = StatusError

			if e.summary.Error == "" {

				if cancelled {
					e.summary.Error = "выполнение отменено"
				} else {
					e.summary.Error = "выполнение прервано"
				}
			}
		}

		if e.file != nil {
			_ = e.file.Close()
			e.file = nil
		}
	}

	now := time.Now()

	r.meta.FinishedAt = &now
	r.meta.Cancelled = cancelled

	r.syncMetaLocked()

	_ = r.persistLocked()

	return r.meta
}

// Snapshot возвращает копию текущих метаданных запуска
func (r *Run) Snapshot() Meta {

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.meta
}

func (r *Run) syncMetaLocked() {

	devices := make([]DeviceSummary, 0, len(r.order))

	for _, id := range r.order {
		devices = append(devices, r.entries[id].summary)
	}

	r.meta.Devices = devices
}

func (r *Run) persistLocked() error {
	return writeMetaFile(r.metaPath, r.meta)
}

func writeMetaFile(
	path string,
	meta Meta,
) error {

	data, err := json.MarshalIndent(meta, "", "  ")

	if err != nil {
		return err
	}

	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

// ListRuns сканирует baseDir и возвращает метаданные всех запусков,
// от самого свежего к самому старому
func ListRuns(
	baseDir string,
) ([]Meta, error) {

	entries, err := os.ReadDir(baseDir)

	if err != nil {

		if os.IsNotExist(err) {
			return []Meta{}, nil
		}

		return nil, err
	}

	metas := make([]Meta, 0, len(entries))

	for _, entry := range entries {

		if !entry.IsDir() {
			continue
		}

		meta, err := LoadRun(baseDir, entry.Name())

		if err != nil {
			continue
		}

		metas = append(metas, meta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].StartedAt.After(metas[j].StartedAt)
	})

	return metas, nil
}

// LoadRun читает run.json конкретного запуска
func LoadRun(
	baseDir string,
	runID string,
) (Meta, error) {

	if err := validateID(runID); err != nil {
		return Meta{}, err
	}

	data, err := os.ReadFile(filepath.Join(baseDir, runID, "run.json"))

	if err != nil {
		return Meta{}, err
	}

	var meta Meta

	if err := json.Unmarshal(data, &meta); err != nil {
		return Meta{}, err
	}

	return meta, nil
}

// ReadDeviceOutput читает текстовый файл вывода конкретного устройства
// в рамках запуска
func ReadDeviceOutput(
	baseDir string,
	runID string,
	deviceID string,
) (string, error) {

	if err := validateID(runID); err != nil {
		return "", err
	}

	if err := validateID(deviceID); err != nil {
		return "", err
	}

	data, err := os.ReadFile(filepath.Join(baseDir, runID, deviceID+".txt"))

	if err != nil {

		if os.IsNotExist(err) {
			return "", nil
		}

		return "", err
	}

	return string(data), nil
}

func validateID(
	id string,
) error {

	if id == "" || strings.ContainsAny(id, `/\`) || id == "." || id == ".." {
		return fmt.Errorf("некорректный идентификатор: %q", id)
	}

	return nil
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// sanitizeID превращает произвольное имя устройства в безопасное
// для имени файла и для передачи как есть на фронтенд
func sanitizeID(
	name string,
) string {

	s := unsafeChars.ReplaceAllString(strings.TrimSpace(name), "_")

	s = strings.Trim(s, "_")

	if s == "" {
		return "device"
	}

	return s
}

// slugify превращает описание запуска в короткий безопасный суффикс
// для имени директории
func slugify(
	description string,
) string {

	s := strings.ToLower(strings.TrimSpace(description))

	s = unsafeChars.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")

	if s == "" {
		return "run"
	}

	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}

	return s
}
