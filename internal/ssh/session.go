package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

type SessionOptions struct {
	RequestPTY bool
}

type Session struct {
	session *gossh.Session

	host string

	address string

	options SessionOptions
}

type Result struct {
	Host string

	Command string

	Output string

	Error error

	Duration time.Duration
}

func NewSession(
	session *gossh.Session,
	host string,
	address string,
	options SessionOptions,
) *Session {

	return &Session{

		session: session,

		host: host,

		address: address,

		options: options,
	}
}

func (s *Session) Run(
	ctx context.Context,
	command string,
) Result {

	start := time.Now()

	result := Result{
		Host:    s.host,
		Command: command,
	}

	//
	// PTY нужен только для оборудования,
	// которое требует интерактивный терминал
	//
	if s.options.RequestPTY {

		err := s.session.RequestPty(
			"xterm",
			120,
			40,
			gossh.TerminalModes{
				gossh.ECHO: 0,
			},
		)

		if err != nil {

			result.Error = fmt.Errorf(
				"request pty: %w",
				err,
			)

			result.Duration = time.Since(start)

			return result
		}
	}

	stdoutPipe, err := s.session.StdoutPipe()

	if err != nil {

		result.Error = fmt.Errorf(
			"stdout pipe: %w",
			err,
		)

		result.Duration = time.Since(start)

		return result
	}

	stderrPipe, err := s.session.StderrPipe()

	if err != nil {

		result.Error = fmt.Errorf(
			"stderr pipe: %w",
			err,
		)

		result.Duration = time.Since(start)

		return result
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = s.session.Start(
		command,
	)

	if err != nil {

		result.Error = fmt.Errorf(
			"start command: %w",
			err,
		)

		result.Duration = time.Since(start)

		return result
	}

	var wg sync.WaitGroup

	wg.Add(2)

	//
	// Читаем stdout сразу после Start
	//
	go func() {

		defer wg.Done()

		_, _ = io.Copy(
			&stdout,
			stdoutPipe,
		)

	}()

	//
	// Читаем stderr параллельно
	//
	go func() {

		defer wg.Done()

		_, _ = io.Copy(
			&stderr,
			stderrPipe,
		)

	}()

	//
	// Ждем либо завершения команды,
	// либо отмены контекста
	//
	wait := make(chan error, 1)

	go func() {
		wait <- s.session.Wait()
	}()
	fmt.Printf(
		"STDOUT=%q STDERR=%q ERR=%v\n",
		stdout.String(),
		stderr.String(),
		err,
	)

	select {

	case <-ctx.Done():

		//
		// Закрываем SSH канал
		//

		_ = s.session.Close()

		wg.Wait()

		result.Error = fmt.Errorf(
			"command canceled: %w",
			ctx.Err(),
		)
		fmt.Printf(
			"STDOUT=%q STDERR=%q ERR=%v\n",
			stdout.String(),
			stderr.String(),
			err,
		)

	case err := <-wait:

		//
		// ВАЖНО:
		// сначала дочитываем stdout/stderr
		//
		wg.Wait()

		result.Output = stdout.String()

		if err != nil {
			//
			// Eltex:
			//
			// ssh сервер выполняет команду,
			// отправляет stdout,
			// но не отправляет SSH exit-status
			//

			if isMissingExitStatus(err) &&
				result.Output != "" {
				err = nil
			}
			result.Error = nil
			if err != nil {
				fmt.Print(err)

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

	}

	fmt.Printf(
		"STDOUT=%q STDERR=%q ERR=%v\n",
		stdout.String(),
		stderr.String(),
		err,
	)

	result.Output = stdout.String()

	result.Duration = time.Since(start)

	return result
}

func isMissingExitStatus(
	err error,
) bool {

	if err == nil {

		return false

	}

	return strings.Contains(
		err.Error(),
		"without exit status",
	)

}

func (s *Session) Close() error {

	if s.session == nil {

		return nil

	}

	return s.session.Close()

}
