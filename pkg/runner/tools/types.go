package tools

import (
	"time"

	"github.com/rsevilla87/ingress-perf/pkg/config"
)

type Tool interface {
	ParseResult(string, string) (PodResult, error)
	Cmd() []string
}

type PodResult struct {
	Name         string  `json:"pod"`
	Node         string  `json:"node"`
	AvgRps       float64 `json:"rps"`
	StdevRps     float64 `json:"rps_stdev"`
	StdevLatency float64 `json:"stdev_lat"`
	AvgLatency   float64 `json:"avg_lat_us"`
	MaxLatency   float64 `json:"max_lat_us"`
	P90Latency   int64   `json:"p90_lat_us"`
	P95Latency   int64   `json:"p95_lat_us"`
	P99Latency   int64   `json:"p99_lat_us"`
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
}
