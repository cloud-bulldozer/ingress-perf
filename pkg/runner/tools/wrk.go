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

func (w *wrk) ParseResult(_, stderr string) (PodResult, error) {
	return w.res, json.Unmarshal([]byte(stderr), &w.res)
}
