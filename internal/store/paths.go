// Package store определяет расположение пользовательских данных
// десктоп-приложения на диске: настройки подключения, сохраненные
// группы устройств и историю запусков.
package store

import (
	"os"
	"path/filepath"
)

const appDirName = "SSHFleet"

// RootDir возвращает (создавая при необходимости) корневую директорию
// данных приложения в домашней папке пользователя
func RootDir() (string, error) {

	home, err := os.UserHomeDir()

	if err != nil {
		return "", err
	}

	dir := filepath.Join(
		home,
		appDirName,
	)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return dir, nil
}

// HistoryDir возвращает (создавая при необходимости) директорию,
// в которую складываются результаты запусков
func HistoryDir() (string, error) {

	root, err := RootDir()

	if err != nil {
		return "", err
	}

	dir := filepath.Join(
		root,
		"history",
	)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return dir, nil
}

// GroupsFile возвращает путь к файлу с сохраненными группами устройств
func GroupsFile() (string, error) {

	root, err := RootDir()

	if err != nil {
		return "", err
	}

	return filepath.Join(root, "groups.json"), nil
}

// ConnectionFile возвращает путь к файлу с настройками SSH-подключения
func ConnectionFile() (string, error) {

	root, err := RootDir()

	if err != nil {
		return "", err
	}

	return filepath.Join(root, "connection.json"), nil
}
