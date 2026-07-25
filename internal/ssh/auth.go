package ssh

import (
	"fmt"
	"os"
	"ssh/internal/config"

	gossh "golang.org/x/crypto/ssh"
)

func BuildAuth(
	cfg config.AuthConfig,
) (gossh.AuthMethod, error) {

	switch cfg.Type {

	case "key":
		return buildKeyAuth(
			cfg,
		)
	case "password":
		return buildPasswordAuth(
			cfg.Password,
		)
	default:
		return nil, fmt.Errorf(
			"unsupported ssh auth type: %q",
			cfg.Type,
		)
	}
}

func buildKeyAuth(
	cfg config.AuthConfig,
) (gossh.AuthMethod, error) {

	if cfg.PrivateKey == "" {

		return nil, fmt.Errorf(
			"private key path is empty",
		)
	}

	keyData, err := os.ReadFile(
		cfg.PrivateKey,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"read private key %s: %w",
			cfg.PrivateKey,
			err,
		)
	}

	var signer gossh.Signer

	//
	// Незашифрованный ключ
	//
	if cfg.Passphrase == "" {
		signer, err = gossh.ParsePrivateKey(
			keyData,
		)

	} else {

		//
		// Ключ с passphrase
		//
		signer, err = gossh.ParsePrivateKeyWithPassphrase(
			keyData,
			[]byte(cfg.Passphrase),
		)

	}

	if err != nil {

		return nil, fmt.Errorf(
			"parse private key: %w",
			err,
		)
	}

	return gossh.PublicKeys(
		signer,
	), nil
}

func buildPasswordAuth(
	password string,
) (gossh.AuthMethod, error) {

	if password == "" {

		return nil, fmt.Errorf(
			"password is empty",
		)
	}

	return gossh.Password(
		password,
	), nil
}
