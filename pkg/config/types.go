package config

import "time"

type Config struct {
	Termination    string        `yaml:"termination"`
	Connections    int           `yaml:"connections"`
	Samples        int           `yaml:"samples"`
	Duration       time.Duration `yaml:"duration"`
	Path           string        `yaml:"path"`
	Concurrency    int32         `yaml:"concurrency"`
	Tool           string        `yaml:"tool"`
	ServerReplicas int32         `yaml:"serverReplicas"`
	Tuning         struct {
		Routers     int `yaml:"routers"`
		ThreadCount int `yaml:"threadCount"`
	} `yaml:"tuning"`
}
