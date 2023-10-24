// Copyright 2023 The ingress-perf Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"encoding/json"
	"fmt"

	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
)

type hLoader struct {
	cmd []string
	res PodResult
}

func init() {
	toolMap["hloader"] = HLoader
}

func HLoader(cfg config.Config, ep string) Tool {
	newHLoader := &hLoader{
		cmd: []string{"hloader", "-u", ep,
			"-c", fmt.Sprint(cfg.Connections),
			"-d", fmt.Sprint(cfg.Duration),
			"-r", fmt.Sprint(cfg.RequestRate),
			"-t", fmt.Sprint(cfg.RequestTimeout),
			"-k", fmt.Sprint(cfg.Keepalive),
			"--http2", fmt.Sprint(cfg.HTTP2),
		},
		res: PodResult{},
	}
	return newHLoader
}

func (w *hLoader) Cmd() []string {
	return w.cmd
}

func (w *hLoader) ParseResult(stdout, _ string) (PodResult, error) {
	return w.res, json.Unmarshal([]byte(stdout), &w.res)
}
