package ssh

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

type Session struct {
	session *gossh.Session
	host    string
	address string
}

type Result struct {
	Host     string
	Command  string
	Output   string
	Error    error
	Duration time.Duration
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
			// Некоторые сетевые устройства (например, Eltex)
			// не отправляют exit-status после exec.
			// Если stderr пустой и stdout получен,
			// считаем выполнение успешным.
			if stderr.Len() == 0 &&
				stdout.Len() > 0 &&
				strings.Contains(err.Error(), "without exit status") {

				err = nil
			}

			if err != nil {
				if stderr.Len() > 0 {
					result.Error = fmt.Errorf("%w: %s", err, stderr.String())
				} else {
					result.Error = fmt.Errorf("execute command: %w", err)
				}
			}
		}
	}
	result.Output = stdout.String()
	result.Duration = time.Since(start)

	return result
}

// func (s *Session) Run(ctx context.Context, command string) Result {
// 	start := time.Now()

// 	result := Result{
// 		Host:    s.host,
// 		Command: command,
// 	}

// 	out, err := s.session.CombinedOutput(command)

// 	result.Output = string(out)
// 	result.Duration = time.Since(start)

// 	if err != nil {
// 		result.Error = err
// 	}

// 	return result
// }

// func (s *Session) Run(
// 	ctx context.Context,
// 	command string,
// ) Result {

// 	start := time.Now()

// 	result := Result{
// 		Host:    s.host,
// 		Command: command,
// 	}

// 	stdoutPipe, err := s.session.StdoutPipe()
// 	if err != nil {
// 		result.Error = fmt.Errorf("stdout pipe: %w", err)
// 		return result
// 	}

// 	stderrPipe, err := s.session.StderrPipe()
// 	if err != nil {
// 		result.Error = fmt.Errorf("stderr pipe: %w", err)
// 		return result
// 	}

// 	if err := s.session.Start(command); err != nil {
// 		result.Error = fmt.Errorf("start command: %w", err)
// 		return result
// 	}

// 	var stdout bytes.Buffer
// 	var stderr bytes.Buffer

// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	go func() {
// 		defer wg.Done()
// 		_, _ = io.Copy(&stdout, stdoutPipe)
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		_, _ = io.Copy(&stderr, stderrPipe)
// 	}()

// 	done := make(chan error, 1)

// 	go func() {
// 		done <- s.session.Wait()
// 	}()

// 	select {

// 	case <-ctx.Done():

// 		_ = s.session.Close()

// 		result.Error = ctx.Err()

// 	case err := <-done:

// 		wg.Wait()

// 		result.Output = stdout.String()

// 		if err != nil {

// 			if stderr.Len() > 0 {

// 				result.Error = fmt.Errorf(
// 					"%w: %s",
// 					err,
// 					stderr.String(),
// 				)

// 			} else {

// 				result.Error = err
// 			}
// 		}
// 	}

// 	result.Duration = time.Since(start)

// 	return result
// }

func (s *Session) Close() error {

	if s.session == nil {
		return nil
	}

	return s.session.Close()
}
