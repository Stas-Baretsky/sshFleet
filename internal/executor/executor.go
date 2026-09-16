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
	client      *sshclient.Client
	workers     int
	retry       config.RetryConfig
	pty         bool
	idleTimeout time.Duration

	//
	// OnDeviceDone, если задан, вызывается ровно один раз на устройство,
	// когда для него больше не будет результатов (успех, ошибка,
	// отмена контекста - неважно). Нужен вызывающему коду (например GUI),
	// чтобы узнать о завершении устройства, не полагаясь на подсчет
	// количества полученных результатов - их может быть меньше, чем
	// команд, если выполнение прервалось на середине (см. RunShell)
	//
	OnDeviceDone func(device inventory.Device)
}

func New(
	client *sshclient.Client,
	cfg config.ExecutorConfig,
	sshCfg config.SSHConfig,
) *Executor {
	workers := cfg.Workers

	if workers <= 0 {
		workers = 1
	}

	return &Executor{
		client:      client,
		workers:     workers,
		retry:       cfg.Retry,
		pty:         sshCfg.PTY,
		idleTimeout: sshCfg.IdleTimeout,
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

	defer e.notifyDone(device)

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

	//
	// Интерактивный режим: одна PTY-сессия на устройство,
	// команды выполняются последовательно в общем контексте
	// (нужно, например, чтобы пройти в configure terminal)
	//
	if e.pty {

		e.executeDeviceShell(
			ctx,
			conn,
			device,
			commands,
			results,
		)

		return
	}

	for _, command := range commands {

		select {
		case <-ctx.Done():
			return
		default:
		}
		session, err := conn.NewSession(
			sshclient.SessionOptions{
				RequestPTY: false,
			},
		)

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

func (e *Executor) executeDeviceShell(
	ctx context.Context,
	conn *sshclient.Connection,
	device inventory.Device,
	commands []string,
	results chan<- sshclient.Result,
) {

	session, err := conn.NewSession(
		sshclient.SessionOptions{
			RequestPTY: true,
		},
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
	defer session.Close()

	for _, result := range session.RunShell(
		ctx,
		commands,
		e.idleTimeout,
	) {

		sendResult(
			ctx,
			results,
			result,
		)
	}
}

func (e *Executor) notifyDone(
	device inventory.Device,
) {

	if e.OnDeviceDone != nil {
		e.OnDeviceDone(device)
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
