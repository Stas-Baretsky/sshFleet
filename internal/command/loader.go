package command

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Commands struct {
	Commands []string `yaml:"commands"`
}

func Load(
	path string,
) ([]string, error) {

	data, err := os.ReadFile(
		path,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"read commands file: %w",
			err,
		)
	}

	var commands Commands

	err = yaml.Unmarshal(
		data,
		&commands,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"parse commands yaml: %w",
			err,
		)
	}

	if err := validate(
		commands.Commands,
	); err != nil {

		return nil, err
	}

	return commands.Commands, nil
}

func validate(
	commands []string,
) error {

	if len(commands) == 0 {

		return fmt.Errorf(
			"commands list is empty",
		)
	}

	for i, cmd := range commands {

		if cmd == "" {

			return fmt.Errorf(
				"command[%d] is empty",
				i,
			)
		}

	}

	return nil
}
