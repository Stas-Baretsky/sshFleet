package ssh

import (
	"fmt"

	"ssh/internal/config"

	gossh "golang.org/x/crypto/ssh"
)

func BuildConfig(
	cfg config.SSHConfig,
) (*gossh.ClientConfig, error) {

	auth, err := BuildAuth(
		cfg.Auth,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"build ssh auth: %w",
			err,
		)
	}

	callback, err := makeHostKeyCallback(
		cfg.KnownHosts,
		cfg.StrictHostKeyChecking,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"build host key callback: %w",
			err,
		)
	}

	return &gossh.ClientConfig{

		User: cfg.User,
		Auth: []gossh.AuthMethod{
			auth,
		},
		HostKeyCallback: callback,
		Timeout:         cfg.Timeout,
	}, nil
}
