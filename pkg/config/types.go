package config

import "time"

type Config struct {
	UUID           string        `json:"-"` // Remove field from json as is already present in Result
	Termination    string        `yaml:"termination" json:"termination"`
	Connections    int           `yaml:"connections" json:"connections"`
	Samples        int           `yaml:"samples" json:"samples"`
	Duration       time.Duration `yaml:"duration" json:"duration"`
	Path           string        `yaml:"path" json:"path"`
	Concurrency    int32         `yaml:"concurrency" json:"concurrency"`
	Tool           string        `yaml:"tool" json:"tool"`
	ServerReplicas int32         `yaml:"serverReplicas" json:"serverReplicas"`
	Tuning         struct {
		Routers     int `yaml:"routers" json:"routers"`
		ThreadCount int `yaml:"threadCount" json:"threadCount"`
	} `yaml:"tuning" json:"tuning"`
}
