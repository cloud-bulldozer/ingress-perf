package tools

import (
	"encoding/json"
	"fmt"

	"github.com/rsevilla87/ingress-perf/pkg/config"
)

type wrk struct {
	cmd []string
	res PodResult
}

func init() {
	toolMap["wrk"] = Wrk
}

func Wrk(cfg config.Config, ep string) Tool {
	newWrk := &wrk{
		cmd: []string{"wrk", "-s", "json.lua", "-c", fmt.Sprint(cfg.Connections), "-d", fmt.Sprintf("%v", cfg.Duration.Seconds()), "--latency", ep, "--timeout=1s"},
		res: PodResult{},
	}
	return newWrk
}

func (w *wrk) Cmd() []string {
	return w.cmd
}

func (w *wrk) ParseResult(stdout, stderr string) (PodResult, error) {
	return w.res, json.Unmarshal([]byte(stderr), &w.res)
}
