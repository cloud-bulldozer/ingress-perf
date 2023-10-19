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

type vegeta struct {
	cmd []string
	res VegetaResult
}

func init() {
	toolMap["vegeta"] = Vegeta
}

func Vegeta(cfg config.Config, ep string) Tool {
	endpoint := fmt.Sprintf("echo GET %v", ep)
	vegetaCmd := fmt.Sprintf("vegeta attack -insecure -max-connections=%d -duration=%v -timeout=%v -keepalive=%v -max-body=0",
		cfg.Connections,
		cfg.Duration,
		cfg.RequestTimeout,
		cfg.Keepalive,
	)
	if cfg.RequestRate > 0 {
		vegetaCmd += fmt.Sprintf(" -rate=%d", cfg.RequestRate)
	} else {
		vegetaCmd += fmt.Sprintf(" -rate=0 -workers=%d -max-workers=%d", cfg.Threads, cfg.Threads)
	}

	newWrk := &vegeta{
		cmd: []string{"bash", "-c", fmt.Sprintf("%v | %v | vegeta report -type json", endpoint, vegetaCmd)},
		res: VegetaResult{},
	}
	return newWrk
}

func (v *vegeta) Cmd() []string {
	return v.cmd
}

/* Example JSON output
{
  "latencies": {
    "total": 1256079085,
    "mean": 31401977,
    "50th": 24082627,
    "90th": 56335116,
    "95th": 66540881,
    "99th": 77088475,
    "max": 77088475,
    "min": 16256151
  },
  "bytes_in": {
    "total": 29211360,
    "mean": 730284
  },
  "bytes_out": {
    "total": 0,
    "mean": 0
  },
  "earliest": "2023-09-28T12:04:38.399615001+02:00",
  "latest": "2023-09-28T12:04:42.300039403+02:00",
  "end": "2023-09-28T12:04:42.364625089+02:00",
  "duration": 3900424402,
  "wait": 64585686,
  "requests": 40,
  "rate": 10.255294264770113,
  "throughput": 10.088246716208607,
  "success": 1,
  "status_codes": {
    "200": 40
  },
  "errors": []
}
*/

func (v *vegeta) ParseResult(stdout, _ string) (PodResult, error) {
	var podResult PodResult
	err := json.Unmarshal([]byte(stdout), &v.res)
	if err != nil {
		return podResult, err
	}
	podResult = PodResult{
		AvgRps:      v.res.Throughput,
		AvgLatency:  v.res.Latencies.AvgLatency / 1e3,
		MaxLatency:  v.res.Latencies.MaxLatency / 1e3,
		P90Latency:  int64(v.res.Latencies.P90Latency / 1e3),
		P95Latency:  int64(v.res.Latencies.P95Latency / 1e3),
		P99Latency:  int64(v.res.Latencies.P99Latency / 1e3),
		Requests:    v.res.Requests,
		StatusCodes: v.res.StatusCodes,
	}
	return podResult, nil
}
