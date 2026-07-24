package config

import "time"

type Config struct {
	SSH       SSHConfig       `yaml:"ssh"`
	Executor  ExecutorConfig  `yaml:"executor"`
	Output    OutputConfig    `yaml:"output"`
	Inventory InventoryConfig `yaml:"inventory"`
	Commands  CommandConfig   `yaml:"commands"`
}
type CommandConfig struct {
	File string `yaml:"file"`
}

type SSHConfig struct {
	User string `yaml:"user" env:"SSH_USER" env-required:"true"`

	Timeout time.Duration `yaml:"timeout" env-default:"10s"`

	KnownHosts string `yaml:"known_hosts" env-default:"~/.ssh/known_hosts"`

	StrictHostKeyChecking bool `yaml:"strict_host_key_checking" env-default:"true"`

	Auth AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
	Type string `yaml:"type" env-default:"key"`

	PrivateKey string `yaml:"private_key"`

	Passphrase string `yaml:"passphrase"`

	Password string `yaml:"password"`
}

type ExecutorConfig struct {
	Workers int `yaml:"workers" env-default:"10"`

	Retry RetryConfig `yaml:"retry"`
}

type RetryConfig struct {
	Attempts int `yaml:"attempts" env-default:"3"`

	Delay time.Duration `yaml:"delay" env-default:"1s"`
}

type OutputConfig struct {
	Format string `yaml:"format" env-default:"text"`

	File string `yaml:"file" env-default:"output/result.txt"`
}

type InventoryConfig struct {
	File string `yaml:"file" env-default:"inventory/hosts.yaml"`
}
