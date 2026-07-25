package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ilyakaznacheev/cleanenv"
)

func Load(
	path string,
) (*Config, error) {

	var cfg Config

	err := cleanenv.ReadConfig(
		path,
		&cfg,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"load config: %w",
			err,
		)
	}

	normalizePaths(
		&cfg,
	)
	if err := validate(&cfg); err != nil {

		return nil, fmt.Errorf(
			"validate config: %w",
			err,
		)
	}
	return &cfg, nil
}

func normalizePaths(
	cfg *Config,
) {
	cfg.SSH.Auth.PrivateKey =
		expandPath(
			cfg.SSH.Auth.PrivateKey,
		)

	cfg.SSH.KnownHosts =
		expandPath(
			cfg.SSH.KnownHosts,
		)

	cfg.Output.File =
		filepath.Clean(
			cfg.Output.File,
		)

	cfg.Inventory.File =
		filepath.Clean(
			cfg.Inventory.File,
		)

	cfg.Commands.File =
		filepath.Clean(
			cfg.Commands.File,
		)
}

func validate(
	cfg *Config,
) error {

	//
	// SSH
	//

	if cfg.SSH.User == "" {
		return fmt.Errorf(
			"ssh.user is required",
		)
	}
	switch cfg.SSH.Auth.Type {

	case "key":
		if cfg.SSH.Auth.PrivateKey == "" {
			return fmt.Errorf(
				"ssh.auth.private_key is required for key auth",
			)
		}

	case "password":
		if cfg.SSH.Auth.Password == "" {
			return fmt.Errorf(
				"ssh.auth.password is required for password auth",
			)
		}

	default:
		return fmt.Errorf(
			"unsupported ssh auth type: %s",
			cfg.SSH.Auth.Type,
		)
	}

	if cfg.SSH.StrictHostKeyChecking &&
		cfg.SSH.KnownHosts == "" {

		return fmt.Errorf(
			"ssh.known_hosts is required when strict checking enabled",
		)
	}

	//
	// Executor
	//

	if cfg.Executor.Workers <= 0 {
		return fmt.Errorf(
			"executor.workers must be greater than zero",
		)
	}

	if cfg.Executor.Retry.Attempts < 0 {
		return fmt.Errorf(
			"executor.retry.attempts cannot be negative",
		)
	}

	//
	// Output
	//

	switch cfg.Output.Format {

	case "text":
	case "json":
	case "csv":

	default:

		return fmt.Errorf(
			"unsupported output format: %s",
			cfg.Output.Format,
		)

	}

	if cfg.Output.File == "" {

		return fmt.Errorf(
			"output.file is required",
		)
	}

	//
	// Inventory
	//

	if cfg.Inventory.File == "" {

		return fmt.Errorf(
			"inventory.file is required",
		)
	}

	if cfg.Commands.File == "" {

		return fmt.Errorf(
			"commands.file is required",
		)
	}

	return nil
}

func expandPath(
	path string,
) string {

	if path == "" {
		return path
	}

	if path[0] != '~' {

		return path
	}

	home, err := os.UserHomeDir()

	if err != nil {

		return path
	}

	if path == "~" {

		return home
	}

	if len(path) > 1 &&
		path[1] == '/' {

		return filepath.Join(
			home,
			path[2:],
		)
	}
	return path
}
