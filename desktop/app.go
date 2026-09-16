package main

import (
	"context"
	"os"
	"path/filepath"

	"ssh/internal/connsettings"
	"ssh/internal/groups"
	"ssh/internal/history"
	"ssh/internal/runner"
	"ssh/internal/store"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App - корневая структура, чьи методы биндятся во фронтенд
// (window.go.main.App.* в JS)
type App struct {
	ctx context.Context

	groupsStore *groups.Store
	connStore   *connsettings.Store

	historyDir string
	runs       *runner.Manager
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {

	a.ctx = ctx

	groupsPath, err := store.GroupsFile()

	if err != nil {
		wailsruntime.LogErrorf(ctx, "resolve groups file: %v", err)
	}

	a.groupsStore = groups.NewStore(groupsPath)

	connPath, err := store.ConnectionFile()

	if err != nil {
		wailsruntime.LogErrorf(ctx, "resolve connection file: %v", err)
	}

	a.connStore = connsettings.NewStore(connPath)

	historyDir, err := store.HistoryDir()

	if err != nil {
		wailsruntime.LogErrorf(ctx, "resolve history dir: %v", err)
	}

	a.historyDir = historyDir
	a.runs = runner.NewManager(historyDir)
}

// GetGroups возвращает все сохраненные группы устройств
func (a *App) GetGroups() ([]groups.Group, error) {
	return a.groupsStore.List()
}

// SaveGroup создает новую группу (пустой ID) или обновляет существующую
func (a *App) SaveGroup(g groups.Group) (groups.Group, error) {
	return a.groupsStore.Save(g)
}

// DeleteGroup удаляет группу по ID
func (a *App) DeleteGroup(id string) error {
	return a.groupsStore.Delete(id)
}

// GetConnectionSettings возвращает текущие настройки подключения
// (или значения по умолчанию, если пользователь их еще не сохранял)
func (a *App) GetConnectionSettings() (connsettings.Settings, error) {
	return a.connStore.Load()
}

// SaveConnectionSettings валидирует и сохраняет настройки подключения
func (a *App) SaveConnectionSettings(s connsettings.Settings) (connsettings.Settings, error) {

	if err := s.Validate(); err != nil {
		return connsettings.Settings{}, err
	}

	if err := a.connStore.Save(s); err != nil {
		return connsettings.Settings{}, err
	}

	return s, nil
}

// PickPrivateKeyFile открывает системный диалог выбора файла приватного ключа
func (a *App) PickPrivateKeyFile() (string, error) {
	return a.pickFile("Выберите приватный SSH-ключ")
}

// PickKnownHostsFile открывает системный диалог выбора файла known_hosts
func (a *App) PickKnownHostsFile() (string, error) {
	return a.pickFile("Выберите файл known_hosts")
}

func (a *App) pickFile(title string) (string, error) {

	defaultDir := ""

	if home, err := os.UserHomeDir(); err == nil {
		defaultDir = filepath.Join(home, ".ssh")
	}

	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDir,
	})
}

// StartRunRequest - то, что реально присылает фронтенд для запуска
// (настройки подключения на форму не завязаны - берутся из сохраненных)
type StartRunRequest struct {
	Description string               `json:"description"`
	Devices     []runner.DeviceInput `json:"devices"`
	Commands    []string             `json:"commands"`
}

// StartRun запускает выполнение команд на устройствах и сразу
// возвращает начальный снимок статусов (все "running"). Дальнейший
// прогресс приходит событиями "run:progress" и "run:finished"
func (a *App) StartRun(req StartRunRequest) (history.Meta, error) {

	settings, err := a.connStore.Load()

	if err != nil {
		return history.Meta{}, err
	}

	full := runner.Request{
		Description: req.Description,
		Devices:     req.Devices,
		Commands:    req.Commands,
		Settings:    settings,
	}

	return a.runs.Start(a.ctx, full, runner.Callbacks{
		OnProgress: func(evt runner.ProgressEvent) {
			wailsruntime.EventsEmit(a.ctx, "run:progress", evt)
		},
		OnFinished: func(meta history.Meta) {
			wailsruntime.EventsEmit(a.ctx, "run:finished", meta)
		},
	})
}

// CancelRun прерывает выполняющийся запуск
func (a *App) CancelRun(runID string) error {
	return a.runs.Cancel(runID)
}

// ListHistory возвращает все прошлые запуски, от новых к старым
func (a *App) ListHistory() ([]history.Meta, error) {
	return history.ListRuns(a.historyDir)
}

// GetRun возвращает метаданные одного запуска (текущего или из истории)
func (a *App) GetRun(runID string) (history.Meta, error) {
	return history.LoadRun(a.historyDir, runID)
}

// GetDeviceOutput возвращает текст вывода одного устройства в рамках запуска
func (a *App) GetDeviceOutput(runID string, deviceID string) (string, error) {
	return history.ReadDeviceOutput(a.historyDir, runID, deviceID)
}
