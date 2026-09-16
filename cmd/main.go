package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ssh/internal/command"
	"ssh/internal/config"
	"ssh/internal/executor"
	"ssh/internal/inventory"
	"ssh/internal/output"

	sshclient "ssh/internal/ssh"
)

func main() {

	logger := slog.New(
		slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelInfo,
			},
		),
	)

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	defer cancel()

	if err := run(
		ctx,
		logger,
	); err != nil {

		logger.Error(
			"application failed",
			"error",
			err,
		)

		os.Exit(1)
	}

}

func run(
	ctx context.Context,
	logger *slog.Logger,
) error {

	cfg, err := config.Load(
		"../configs/config.yaml",
	)

	if err != nil {

		return err
	}

	logger.Info(
		"config loaded",
	)

	inv, err := inventory.Load(
		cfg.Inventory.File,
	)

	if err != nil {

		return err
	}

	logger.Info(
		"inventory loaded",
		"devices",
		len(inv.Devices),
	)

	commands, err := command.Load(
		cfg.Commands.File,
	)

	if err != nil {

		return err
	}

	logger.Info(
		"commands loaded",
		"commands",
		len(commands),
	)

	sshConfig, err := sshclient.BuildConfig(
		cfg.SSH,
	)

	if err != nil {

		return err
	}

	client := sshclient.NewClient(
		sshConfig,
		cfg.SSH.Timeout,
	)

	exec := executor.New(
		client,
		cfg.Executor,
		cfg.SSH,
	)

	writer, err := output.NewWriter(
		cfg.Output,
	)

	if err != nil {

		return err
	}

	defer func() {

		if err := writer.Close(); err != nil {

			logger.Error(
				"close writer",
				"error",
				err,
			)

		}

	}()

	results := exec.Execute(
		ctx,
		inv.Devices,
		commands,
	)

	if err := writer.Write(
		ctx,
		results,
	); err != nil {

		return err
	}

	logger.Info(
		"execution completed",
	)

	return nil
}
