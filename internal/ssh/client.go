package ssh

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

type Client struct {
	config  *gossh.ClientConfig
	timeout time.Duration
}

type Target struct {
	Name    string
	Address string
	Port    int
	User    string
}

type Connection struct {
	client *gossh.Client
	target Target
}

func NewClient(
	config *gossh.ClientConfig,
	timeout time.Duration,
) *Client {

	return &Client{
		config:  config,
		timeout: timeout,
	}
}

func (c *Client) Connect(
	ctx context.Context,
	target Target,
) (*Connection, error) {

	address := normalizeAddress(
		target,
	)

	config := *c.config

	//
	// Если пользователь указан в inventory,
	// он имеет приоритет над глобальным
	//
	if target.User != "" {
		config.User = target.User
	}

	dialer := net.Dialer{
		Timeout: c.timeout,
	}

	netConn, err := dialer.DialContext(
		ctx,
		"tcp",
		address,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"dial %s: %w",
			address,
			err,
		)
	}

	sshConn, chans, reqs, err := gossh.NewClientConn(
		netConn,
		address,
		&config,
	)

	if err != nil {

		netConn.Close()

		return nil, fmt.Errorf(
			"ssh handshake %s: %w",
			address,
			err,
		)
	}

	client := gossh.NewClient(
		sshConn,
		chans,
		reqs,
	)

	return &Connection{
		client: client,
		target: target,
	}, nil
}

func (c *Connection) NewSession() (*Session, error) {

	session, err := c.client.NewSession()

	if err != nil {
		return nil, err
	}

	// err = session.RequestPty(
	// 	"xterm",
	// 	120,
	// 	40,
	// 	gossh.TerminalModes{
	// 		gossh.ECHO: 0,
	// 	},
	// )

	// if err != nil {
	// 	session.Close()

	// 	return nil, fmt.Errorf(
	// 		"request pty: %w",
	// 		err,
	// 	)
	// }

	return &Session{
		session: session,
		host:    c.target.Name,
		address: normalizeAddress(c.target),
	}, nil
}

func (c *Connection) Close() error {
	return c.client.Close()
}

func normalizeAddress(
	target Target,
) string {
	port := target.Port

	if port == 0 {

		port = 22

	}

	return net.JoinHostPort(
		target.Address,
		strconv.Itoa(port),
	)
}
