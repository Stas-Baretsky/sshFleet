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

	defer s.session.Close()

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

	stdinPipe, _ := s.session.StdinPipe()

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

	s.session.Start(command)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	io.Copy(&stdout, stdoutPipe)

	fmt.Fprintf(stdinPipe, "exit\n")

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

	result.Output = stdout.String()

	result.Duration = time.Since(start)

	return result
}

// RunShell открывает один интерактивный PTY-канал на всё устройство
// и последовательно "печатает" в него команды, как в реальном терминале.
//
// Нужен для оборудования (например Eltex), которое по exec-каналу
// пускает только в режим exec и не позволяет войти в configure terminal:
// там команда меняющая режим (configure terminal, username ... password ...)
// обязана выполняться в рамках одной и той же сессии, а не в отдельном
// exec-запросе на каждую команду.
func (s *Session) RunShell(
	ctx context.Context,
	commands []string,
	idleTimeout time.Duration,
) []Result {

	if idleTimeout <= 0 {
		idleTimeout = 2 * time.Second
	}

	err := s.session.RequestPty(
		"xterm",
		200,
		50,
		gossh.TerminalModes{
			gossh.ECHO: 1,
		},
	)

	if err != nil {

		return []Result{{
			Host:  s.host,
			Error: fmt.Errorf("request pty: %w", err),
		}}
	}

	stdin, err := s.session.StdinPipe()

	if err != nil {

		return []Result{{
			Host:  s.host,
			Error: fmt.Errorf("stdin pipe: %w", err),
		}}
	}

	stdout, err := s.session.StdoutPipe()

	if err != nil {

		return []Result{{
			Host:  s.host,
			Error: fmt.Errorf("stdout pipe: %w", err),
		}}
	}

	if err := s.session.Shell(); err != nil {

		return []Result{{
			Host:  s.host,
			Error: fmt.Errorf("start shell: %w", err),
		}}
	}

	//
	// done закрывается перед выходом из RunShell, чтобы читающая
	// горутина не зависла навсегда, пытаясь отправить в outCh
	// после того, как мы перестали читать
	//
	done := make(chan struct{})
	defer close(done)

	outCh := make(chan []byte)

	go func() {

		buf := make([]byte, 4096)

		for {

			n, readErr := stdout.Read(buf)

			if n > 0 {

				chunk := make([]byte, n)
				copy(chunk, buf[:n])

				select {
				case outCh <- chunk:
				case <-done:
					return
				}
			}

			if readErr != nil {
				return
			}
		}
	}()

	//
	// drain читает всё, что накопилось в outCh, пока не наступит
	// idleTimeout тишины (устройство "замолчало" - команда отработала)
	//
	drain := func(timeout time.Duration) string {

		var buf bytes.Buffer

		timer := time.NewTimer(timeout)
		defer timer.Stop()

		for {
			select {

			case chunk := <-outCh:

				buf.Write(chunk)

				if !timer.Stop() {
					<-timer.C
				}

				timer.Reset(timeout)

			case <-timer.C:

				return buf.String()

			case <-ctx.Done():

				return buf.String()
			}
		}
	}

	//
	// Дочитываем баннер логина/приглашение перед первой командой
	//
	drain(idleTimeout)

	results := make([]Result, 0, len(commands))

	for _, command := range commands {

		select {
		case <-ctx.Done():

			results = append(results, Result{
				Host:    s.host,
				Command: command,
				Error:   ctx.Err(),
			})

			return results

		default:
		}

		start := time.Now()

		_, writeErr := fmt.Fprintf(
			stdin,
			"%s\n",
			command,
		)

		result := Result{
			Host:    s.host,
			Command: command,
		}

		if writeErr != nil {

			result.Error = fmt.Errorf(
				"write command: %w",
				writeErr,
			)

			result.Duration = time.Since(start)

			results = append(results, result)

			return results
		}

		result.Output = drain(idleTimeout)
		result.Duration = time.Since(start)

		results = append(results, result)
	}

	_, _ = fmt.Fprint(stdin, "exit\n")
	_ = stdin.Close()

	return results
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
