package output

import (
	"fmt"
	"strings"

	sshclient "ssh/internal/ssh"
)

type TextFormatter struct {
}

func NewTextFormatter() *TextFormatter {

	return &TextFormatter{}
}

func (f *TextFormatter) Write(
	result sshclient.Result,
) ([]byte, error) {

	var builder strings.Builder

	builder.WriteString(
		"================================\n",
	)

	builder.WriteString(
		fmt.Sprintf(
			"HOST: %s\n",
			result.Host,
		),
	)

	builder.WriteString(
		fmt.Sprintf(
			"COMMAND: %s\n",
			result.Command,
		),
	)

	builder.WriteString(
		fmt.Sprintf(
			"DURATION: %s\n",
			result.Duration,
		),
	)

	if result.Error != nil {

		builder.WriteString(
			"STATUS: FAILED\n",
		)

		builder.WriteString(
			fmt.Sprintf(
				"ERROR: %s\n",
				result.Error,
			),
		)

	} else {

		builder.WriteString(
			"STATUS: SUCCESS\n",
		)

		builder.WriteString(
			"OUTPUT:\n",
		)

		builder.WriteString(
			result.Output,
		)

		if !strings.HasSuffix(
			result.Output,
			"\n",
		) {

			builder.WriteString(
				"\n",
			)

		}

	}

	builder.WriteString(
		"================================\n\n",
	)

	return []byte(
		builder.String(),
	), nil
}
