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

import (
	"os"
	"time"

	yaml "gopkg.in/yaml.v3"
)

// UnmarshalYAML implements YAML unmarshalleer to set default values in the config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type ConfigDefaulted Config
	defaultCfg := ConfigDefaulted{
		Warmup:            false, // Disable warmup by default
		RequestTimeout:    time.Second,
		Procs:             1,
		Keepalive:         true,
		PrometheusMetrics: prometheusQueries,
	}
	if err := unmarshal(&defaultCfg); err != nil {
		return err
	}
	*c = Config(defaultCfg)
	return nil
}

func Load(cfg string) error {
	f, err := os.Open(cfg)
	if err != nil {
		return err
	}
	data := yaml.NewDecoder(f)
	data.KnownFields(true)
	err = data.Decode(&Cfg)
	return err
}
