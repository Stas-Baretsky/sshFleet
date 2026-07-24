package ssh

import (
	"bytes"
	"context"
	"fmt"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

type Session struct {
	session *gossh.Session

	host string

	address string
}

type Result struct {
	Host string

	Command string

	Output string

	Error error

	Duration time.Duration
}

func (s *Session) Run(
	ctx context.Context,
	command string,
) Result {

	start := time.Now()

	result := Result{

		Host: s.host,

		Command: command,
	}

	var stdout bytes.Buffer

	var stderr bytes.Buffer

	s.session.Stdout = &stdout

	s.session.Stderr = &stderr

	done := make(chan error, 1)

	go func() {

		done <- s.session.Run(
			command,
		)

	}()

	select {

	case <-ctx.Done():

		//
		// Прерываем выполнение команды
		//
		_ = s.session.Close()

		err := <-done

		if err != nil {

			result.Error = fmt.Errorf(
				"command canceled: %w",
				ctx.Err(),
			)

		} else {

			result.Error = ctx.Err()

		}

	case err := <-done:

		if err != nil {

			if stderr.Len() > 0 {

				result.Error = fmt.Errorf(
					"%w: %s",
					err,
					stderr.String(),
				)

			} else {

				result.Error = fmt.Errorf(
					"execute command: %w",
					err,
				)

			}

		}

	}

	result.Output = stdout.String()

	result.Duration = time.Since(start)

	return result
}

func (s *Session) Close() error {

	if s.session == nil {

		return nil

	}

	return s.session.Close()
}
