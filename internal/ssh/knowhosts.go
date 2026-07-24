package ssh

import (
	"fmt"
	"net"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func makeHostKeyCallback(
	path string,
	strict bool,
) (gossh.HostKeyCallback, error) {

	if !strict {

		return func(
			host string,
			addr net.Addr,
			key gossh.PublicKey,
		) error {

			return nil
		}, nil
	}

	if path == "" {

		return nil, fmt.Errorf(
			"known_hosts path is empty",
		)
	}

	callback, err := knownhosts.New(
		path,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"create knownhosts callback: %w",
			err,
		)
	}

	return callback, nil
}
