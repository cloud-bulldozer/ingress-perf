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
	"time"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/ocp-metadata"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
)

// We need to embed ClusterMetadata in order to add extra fields to it
type ClusterMetadata struct {
	ocpmetadata.ClusterMetadata
	HAProxyVersion string `json:"haproxyVersion,omitempty"`
}

type Tool interface {
	ParseResult(string, string) (PodResult, error)
	Cmd() []string
}

type PodResult struct {
	Name         string      `json:"pod"`
	Node         string      `json:"node"`
	InstanceType string      `json:"instanceType"`
	AvgRps       float64     `json:"rps"`
	StdevRps     float64     `json:"rps_stdev"`
	StdevLatency float64     `json:"stdev_lat"`
	AvgLatency   float64     `json:"avg_lat_us"`
	MaxLatency   float64     `json:"max_lat_us"`
	P90Latency   int64       `json:"p90_lat_us"`
	P95Latency   int64       `json:"p95_lat_us"`
	P99Latency   int64       `json:"p99_lat_us"`
	HTTPErrors   int64       `json:"http_errors"`
	ReadErrors   int64       `json:"read_errors"`
	WriteErrors  int64       `json:"write_errors"`
	Requests     int64       `json:"requests"`
	Timeouts     int64       `json:"timeouts"`
	StatusCodes  map[int]int `json:"status_codes"`
}

type Result struct {
	UUID         string        `json:"uuid"`
	Sample       int           `json:"sample"`
	Config       config.Config `json:"config"`
	Pods         []PodResult   `json:"pods"`
	Timestamp    time.Time     `json:"timestamp"`
	TotalAvgRps  float64       `json:"total_avg_rps"`
	StdevRps     float64       `json:"rps_stdev"`
	StdevLatency float64       `json:"stdev_lat"`
	AvgLatency   float64       `json:"avg_lat_us"`
	MaxLatency   float64       `json:"max_lat_us"`
	P90Latency   float64       `json:"p90_lat_us"`
	P95Latency   float64       `json:"p95_lat_us"`
	P99Latency   float64       `json:"p99_lat_us"`
	HTTPErrors   int64         `json:"http_errors"`
	ReadErrors   int64         `json:"read_errors"`
	WriteErrors  int64         `json:"write_errors"`
	Requests     int64         `json:"requests"`
	Timeouts     int64         `json:"timeouts"`
	Version      string        `json:"version"`
	StatusCodes  map[int]int   `json:"status_codes"`
	ClusterMetadata
}

type VegetaResult struct {
	Latencies struct {
		Total      float64 `json:"total"`
		AvgLatency float64 `json:"mean"`
		P50Latency float64 `json:"50th"`
		P90Latency float64 `json:"90th"`
		P95Latency float64 `json:"95th"`
		P99Latency float64 `json:"99th"`
		MaxLatency float64 `json:"max"`
		MinLatency float64 `json:"min"`
	} `json:"latencies"`
	BytesIn struct {
		Total float64 `json:"total"`
		Mean  float64 `json:"mean"`
	} `json:"bytes_in"`
	BytesOut struct {
		Total float64 `json:"total"`
		Mean  float64 `json:"mean"`
	} `json:"bytes_out"`
	Requests    int64       `json:"requests"`
	Throughput  float64     `json:"throughput"`
	StatusCodes map[int]int `json:"status_codes"`
}
