package tools

import (
	"fmt"

	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
)

var toolMap = make(map[string]func(config.Config, string) Tool)

func New(cfg config.Config, endpoint string) (Tool, error) {
	var tool Tool
	f, ok := toolMap[cfg.Tool]
	if !ok {
		return tool, fmt.Errorf("tool %v not supported", cfg.Tool)
	}
	return f(cfg, endpoint), nil
}
