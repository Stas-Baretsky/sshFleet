// Package runner связывает воедино ввод пользователя (устройства,
// команды, настройки подключения), internal/executor и internal/history:
// он запускает один опрос устройств, пишет результаты в историю и
// сообщает о прогрессе через колбэки (в десктоп-приложении это
// оборачивается в события Wails)
package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"ssh/internal/connsettings"
	"ssh/internal/executor"
	"ssh/internal/history"
	"ssh/internal/inventory"
	sshclient "ssh/internal/ssh"
)

// DeviceInput - устройство, как его вводит пользователь: адрес
// обязателен, имя опционально (если пусто, используется адрес)
type DeviceInput struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Request - все, что нужно для одного запуска
type Request struct {
	Description string                `json:"description"`
	Devices     []DeviceInput         `json:"devices"`
	Commands    []string              `json:"commands"`
	Settings    connsettings.Settings `json:"settings"`
}

// ProgressEvent - обновление состояния одного устройства в рамках запуска
type ProgressEvent struct {
	RunID  string                `json:"runId"`
	Device history.DeviceSummary `json:"device"`
}

// Callbacks - реакции на события запуска. OnFinished вызывается ровно
// один раз, когда обработаны все устройства (успешно, с ошибками или
// после отмены)
type Callbacks struct {
	OnProgress func(ProgressEvent)
	OnFinished func(history.Meta)
}

// Manager отслеживает запущенные выполнения, чтобы их можно было отменить
type Manager struct {
	mu         sync.Mutex
	historyDir string
	cancels    map[string]context.CancelFunc
}

func NewManager(
	historyDir string,
) *Manager {

	return &Manager{
		historyDir: historyDir,
		cancels:    make(map[string]context.CancelFunc),
	}
}

// Start проверяет запрос, создает запись в истории и запускает
// выполнение в фоновой горутине. Возвращает начальный снимок
// метаданных запуска (все устройства уже в статусе "running")
func (m *Manager) Start(
	parent context.Context,
	req Request,
	cb Callbacks,
) (history.Meta, error) {

	devices, err := resolveDevices(req.Devices, req.Settings.Port)

	if err != nil {
		return history.Meta{}, err
	}

	commands := cleanCommands(req.Commands)

	if len(commands) == 0 {
		return history.Meta{}, fmt.Errorf("список команд пуст")
	}

	if err := req.Settings.Validate(); err != nil {
		return history.Meta{}, err
	}

	run, err := history.NewRun(m.historyDir, req.Description, devices, commands)

	if err != nil {
		return history.Meta{}, fmt.Errorf("create history run: %w", err)
	}

	sshConfig, err := sshclient.BuildConfig(req.Settings.SSHConfig())

	if err != nil {
		return history.Meta{}, fmt.Errorf("build ssh config: %w", err)
	}

	client := sshclient.NewClient(sshConfig, req.Settings.SSHConfig().Timeout)

	exec := executor.New(client, req.Settings.ExecutorConfig(), req.Settings.SSHConfig())

	ctx, cancel := context.WithCancel(parent)

	m.mu.Lock()
	m.cancels[run.ID()] = cancel
	m.mu.Unlock()

	exec.OnDeviceDone = func(device inventory.Device) {

		id, ok := run.DeviceIDForName(device.Name)

		if !ok {
			return
		}

		summary, ok := run.MarkDone(id)

		if ok && cb.OnProgress != nil {
			cb.OnProgress(ProgressEvent{RunID: run.ID(), Device: summary})
		}
	}

	run.MarkAllRunning()

	initial := run.Snapshot()

	go func() {

		defer func() {

			m.mu.Lock()
			delete(m.cancels, run.ID())
			m.mu.Unlock()

			cancel()
		}()

		resultsCh := exec.Execute(ctx, devices, commands)

		for res := range resultsCh {

			id, ok := run.DeviceIDForName(res.Host)

			if !ok {
				continue
			}

			summary, ok := run.RecordResult(id, res)

			if ok && cb.OnProgress != nil {
				cb.OnProgress(ProgressEvent{RunID: run.ID(), Device: summary})
			}
		}

		meta := run.Finish(ctx.Err() != nil)

		if cb.OnFinished != nil {
			cb.OnFinished(meta)
		}
	}()

	return initial, nil
}

// Cancel прерывает выполняющийся запуск. Возвращает ошибку, если
// запуск с таким ID сейчас не выполняется (уже завершен или не существовал)
func (m *Manager) Cancel(
	runID string,
) error {

	m.mu.Lock()
	cancel, ok := m.cancels[runID]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("выполнение %q не найдено или уже завершено", runID)
	}

	cancel()

	return nil
}

func resolveDevices(
	inputs []DeviceInput,
	defaultPort int,
) ([]inventory.Device, error) {

	if len(inputs) == 0 {
		return nil, fmt.Errorf("список устройств пуст")
	}

	used := map[string]int{}

	devices := make([]inventory.Device, 0, len(inputs))

	for _, in := range inputs {

		address := strings.TrimSpace(in.Address)

		if address == "" {
			continue
		}

		name := strings.TrimSpace(in.Name)

		if name == "" {
			name = address
		}

		used[name]++

		if used[name] > 1 {
			name = fmt.Sprintf("%s (%d)", name, used[name])
		}

		devices = append(devices, inventory.Device{
			Name:    name,
			Address: address,
			Port:    defaultPort,
		})
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("не указан ни один адрес устройства")
	}

	return devices, nil
}

func cleanCommands(
	commands []string,
) []string {

	out := make([]string, 0, len(commands))

	for _, c := range commands {

		c = strings.TrimRight(c, "\r\n")

		if strings.TrimSpace(c) == "" {
			continue
		}

		out = append(out, c)
	}

	return out
}
