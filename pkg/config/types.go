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

package config

import "time"

var Cfg []Config

type Config struct {
	UUID string `json:"-"` // Remove field from json as is already present in Result
	// Termination benchmark termination type: allowed values are http, edge, reencrypt and reencrypt
	Termination string `yaml:"termination" json:"termination"`
	// Connections number of connections per client
	Connections int `yaml:"connections" json:"connections"`
	// Samples number of samples per scenario
	Samples int `yaml:"samples" json:"samples"`
	// Duration of each sample
	Duration time.Duration `yaml:"duration" json:"duration"`
	// Path scenario endpoint. i.e: 1024.html, 2048.html
	Path string `yaml:"path" json:"path"`
	// Concurrency defined the number of clients
	Concurrency int32 `yaml:"concurrency" json:"concurrency"`
	// Tool defines the tool to run the benchmark scenario
	Tool string `yaml:"tool" json:"tool"`
	// ServerReplicas number of server (nginx) replicas backed by the routes. Example: wrk
	ServerReplicas int32 `yaml:"serverReplicas" json:"serverReplicas"`
	// Tuning defines a tuning patch for the default IngressController object
	Tuning string `yaml:"tuningPatch" json:"tuningPatch"`
	// Delay defines a delay between samples
	Delay time.Duration `yaml:"delay"`
	// Warmup enables warmup: Indexing will be disabled in this scenario. Default is false
	Warmup bool `yaml:"warmup" json:"-"`
}
