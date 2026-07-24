package output

import (
	"context"
	"fmt"
	"os"

	"ssh/internal/config"
	sshclient "ssh/internal/ssh"
)

type Writer struct {
	file *os.File

	formatter Formatter
}

type Formatter interface {
	Write(
		result sshclient.Result,
	) ([]byte, error)
}

func NewWriter(
	cfg config.OutputConfig,
) (*Writer, error) {

	file, err := os.OpenFile(
		cfg.File,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"open output file: %w",
			err,
		)
	}

	formatter, err := getFormatter(
		cfg.Format,
	)

	if err != nil {

		file.Close()

		return nil, err
	}

	return &Writer{

		file: file,

		formatter: formatter,
	}, nil
}

func (w *Writer) Write(
	ctx context.Context,
	results <-chan sshclient.Result,
) error {

	for {

		select {

		case <-ctx.Done():

			return ctx.Err()

		case result, ok := <-results:

			if !ok {

				return nil

			}

			data, err := w.formatter.Write(
				result,
			)

			if err != nil {

				return err

			}

			_, err = w.file.Write(
				data,
			)

			if err != nil {

				return fmt.Errorf(
					"write result: %w",
					err,
				)
			}

		}

	}

}

func (w *Writer) Close() error {

	return w.file.Close()

}

func getFormatter(
	format string,
) (Formatter, error) {

	switch format {

	case "text":

		return NewTextFormatter(), nil

	// case "json":

	// 	return NewJSONFormatter(), nil

	// case "csv":

	// 	return NewCSVFormatter(), nil

	default:

		return nil, fmt.Errorf(
			"unsupported output format: %s",
			format,
		)

	}

}
