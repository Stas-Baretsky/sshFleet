package inventory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Inventory struct {
	Devices []Device `yaml:"devices"`
}

type Device struct {
	Name string `yaml:"name"`

	Address string `yaml:"address"`

	Username string `yaml:"username"`

	Port int `yaml:"port"`

	Tags []string `yaml:"tags"`
}

func Load(
	path string,
) (*Inventory, error) {

	data, err := os.ReadFile(
		path,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"read inventory file: %w",
			err,
		)
	}

	var inv Inventory

	err = yaml.Unmarshal(
		data,
		&inv,
	)

	if err != nil {

		return nil, fmt.Errorf(
			"parse inventory yaml: %w",
			err,
		)
	}

	if err := validate(
		&inv,
	); err != nil {

		return nil, err
	}

	return &inv, nil
}

func validate(
	inv *Inventory,
) error {

	if len(inv.Devices) == 0 {

		return fmt.Errorf(
			"inventory is empty",
		)
	}

	for i, device := range inv.Devices {

		if device.Name == "" {

			return fmt.Errorf(
				"device[%d]: name is empty",
				i,
			)
		}

		if device.Address == "" {

			return fmt.Errorf(
				"device[%d]: address is empty",
				i,
			)
		}

		if device.Port < 0 ||
			device.Port > 65535 {

			return fmt.Errorf(
				"device[%d]: invalid port %d",
				i,
				device.Port,
			)
		}

	}

	return nil
}
