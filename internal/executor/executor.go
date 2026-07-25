package executor

import (
	"context"
	"sync"
	"time"

	"ssh/internal/config"
	"ssh/internal/inventory"

	sshclient "ssh/internal/ssh"
)

type Executor struct {
	client  *sshclient.Client
	workers int
	retry   config.RetryConfig
}

func New(
	client *sshclient.Client,
	cfg config.ExecutorConfig,
) *Executor {
	workers := cfg.Workers

	if workers <= 0 {
		workers = 1
	}

	return &Executor{
		client:  client,
		workers: workers,
		retry:   cfg.Retry,
	}
}

func (e *Executor) Execute(
	ctx context.Context,
	devices []inventory.Device,
	commands []string,
) <-chan sshclient.Result {

	results := make(chan sshclient.Result)
	jobs := make(chan inventory.Device)

	go func() {

		defer close(results)
		var wg sync.WaitGroup

		for i := 0; i < e.workers; i++ {
			wg.Add(1)

			go func() {

				defer wg.Done()
				for device := range jobs {
					e.executeDevice(
						ctx,
						device,
						commands,
						results,
					)
				}
			}()
		}
		for _, device := range devices {

			select {
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return
			case jobs <- device:
			}
		}
		close(jobs)
		wg.Wait()
	}()
	return results
}

func (e *Executor) executeDevice(
	ctx context.Context,
	device inventory.Device,
	commands []string,
	results chan<- sshclient.Result,
) {

	target := sshclient.Target{
		Name:    device.Name,
		Address: device.Address,
		Port:    device.Port,
		User:    device.Username,
	}

	conn, err := e.connectWithRetry(
		ctx,
		target,
	)

	if err != nil {
		sendResult(
			ctx,
			results,
			sshclient.Result{
				Host:  device.Name,
				Error: err,
			},
		)
		return
	}
	defer conn.Close()

	for _, command := range commands {

		select {
		case <-ctx.Done():
			return
		default:
		}
		session, err := conn.NewSession()

		if err != nil {
			sendResult(
				ctx,
				results,
				sshclient.Result{
					Host:    device.Name,
					Command: command,
					Error:   err,
				},
			)
			continue
		}
		result := session.Run(
			ctx,
			command,
		)
		session.Close()

		sendResult(
			ctx,
			results,
			result,
		)
	}
}

func (e *Executor) connectWithRetry(
	ctx context.Context,
	target sshclient.Target,
) (*sshclient.Connection, error) {

	var lastErr error

	for attempt := 0; attempt <= e.retry.Attempts; attempt++ {
		conn, err := e.client.Connect(
			ctx,
			target,
		)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if attempt == e.retry.Attempts {
			break
		}
		if !sleepContext(
			ctx,
			e.retry.Delay,
		) {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func sleepContext(
	ctx context.Context,
	delay time.Duration,
) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func sendResult(
	ctx context.Context,
	results chan<- sshclient.Result,
	result sshclient.Result,
) {
	select {
	case <-ctx.Done():
		return
	case results <- result:

	}

}
