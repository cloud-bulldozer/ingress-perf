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

package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloud-bulldozer/go-commons/prometheus"
	"github.com/cloud-bulldozer/go-commons/version"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
	"github.com/cloud-bulldozer/ingress-perf/pkg/runner/tools"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var lock = &sync.Mutex{}

func runBenchmark(
	cfg config.Config,
	clusterMetadata tools.ClusterMetadata,
	p *prometheus.Prometheus,
	podMetrics bool,
	isGatewayAPIEnabled bool,
) ([]tools.Result, error) {
	var aggAvgRps, aggAvgLatency, aggP95Latency float64
	var timeouts, httpErrors int64
	var benchmarkResult []tools.Result
	var clientPods []corev1.Pod
	var ep string
	var host string
	if isGatewayAPIEnabled {
		hr, err := hrClientSet.GatewayV1beta1().HTTPRoutes(routesNamespace).
			Get(context.TODO(), serverName, metav1.GetOptions{})
		if err != nil {
			return benchmarkResult, err
		}
		host = string(hr.Spec.Hostnames[0])
	} else {
		r, err := orClientSet.RouteV1().Routes(routesNamespace).
			Get(context.TODO(), fmt.Sprintf("%s", cfg.Termination), metav1.GetOptions{})
		if err != nil {
			return benchmarkResult, err
		}
		host = r.Spec.Host
	}
	protocol := "http"
	if cfg.Termination != "http" {
		protocol = "https"
	}

	ep = fmt.Sprintf("%s://%v%v", protocol, host, cfg.Path)
	allClientPods, err := clientSet.CoreV1().Pods(benchmarkNs.Name).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", clientName),
	})
	if err != nil {
		return benchmarkResult, err
	}
	// Filter out pods in terminating state from the list
	for _, p := range allClientPods.Items {
		if p.DeletionTimestamp == nil {
			clientPods = append(clientPods, p)
		}
		if len(clientPods) == int(cfg.Concurrency) {
			break
		}
	}
	ts := time.Now().UTC()
	for i := 1; i <= cfg.Samples; i++ {
		sampleTs := time.Now().UTC()
		result := tools.Result{
			UUID:            cfg.UUID,
			Sample:          i,
			Config:          cfg,
			Timestamp:       ts,
			ClusterMetadata: clusterMetadata,
			InfraMetrics:    make(map[string]float64),
		}
		result.Config.Tuning = currentTuning // It's useful to index the current tuning patch in the all benchmark's documents
		log.Infof("Running sample %d/%d: %v", i, cfg.Samples, cfg.Duration)
		errGroup := errgroup.Group{}
		for _, pod := range clientPods {
			for i := 0; i < cfg.Procs; i++ {
				func(p corev1.Pod) {
					errGroup.Go(func() error {
						tool, err := tools.New(cfg, ep)
						if err != nil {
							return err
						}
						log.Debugf("Running %v in client pods", tool.Cmd())
						return exec(context.TODO(), tool, p, &result)
					})
				}(pod)
			}
		}
		if err = errGroup.Wait(); err != nil {
			log.Errorf("Errors found during execution, skipping sample: %s", err)
			continue
		}
		normalizeResults(&result)
		if !podMetrics {
			result.Pods = nil
		}
		aggAvgRps += result.TotalAvgRps
		aggAvgLatency += result.AvgLatency
		aggP95Latency += result.P95Latency
		timeouts += result.Timeouts
		httpErrors += result.HTTPErrors
		elapsed := fmt.Sprintf("%ds", int(time.Since(sampleTs).Seconds()))
		for field, query := range config.PrometheusQueries {
			promQuery := strings.ReplaceAll(query, "ELAPSED", elapsed)
			log.Debugf("Running query: %s", promQuery)
			value, err := p.Query(promQuery, time.Time{}.UTC())
			if err != nil {
				log.Errorf("Query error: %v", err)
				continue
			}
			data, ok := value.(model.Vector)
			if !ok {
				log.Errorf("Unsupported result format: %s", value.Type().String())
				continue
			}
			for _, vector := range data {
				result.InfraMetrics[field] = float64(vector.Value)
			}
		}
		log.Infof("%s: Rps=%.0f avgLatency=%.0fms P95Latency=%.0fms", cfg.Termination, result.TotalAvgRps, result.AvgLatency/1e3, result.P95Latency/1e3)
		benchmarkResult = append(benchmarkResult, result)
		if cfg.Delay != 0 {
			log.Info("Sleeping for ", cfg.Delay)
			time.Sleep(cfg.Delay)
		}
	}
	validSamples := float64(len(benchmarkResult))
	log.Infof("Scenario summary %s: Rps=%.0f avgLatency=%.0fms P95Latency=%.0fms timeouts=%d http_errors=%d",
		cfg.Termination,
		aggAvgRps/validSamples,
		aggAvgLatency/validSamples/1e3,
		aggP95Latency/validSamples/1e3,
		timeouts,
		httpErrors,
	)
	return benchmarkResult, nil
}

func exec(ctx context.Context, tool tools.Tool, pod corev1.Pod, result *tools.Result) error {
	var stdout, stderr bytes.Buffer
	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(benchmarkNs.Name).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: clientName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		Command:   tool.Cmd(),
		TTY:       false,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		log.Error(err.Error())
		return err
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		log.Errorf("Exec failed in pod %s: %v, stderr: %v", pod.Name, err.Error(), stderr.String())
		return err
	}
	podResult, err := tool.ParseResult(stdout.String(), stderr.String())
	if err != nil {
		log.Errorf("Result parsing failed: %v", err.Error())
		log.Errorf("Stdout: %v", stdout.String())
		log.Errorf("Stderr: %v", stderr.String())
		return err
	}
	podResult.Name = pod.Name
	podResult.Node = pod.Spec.NodeName
	node, err := clientSet.CoreV1().Nodes().Get(context.TODO(), podResult.Node, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Couldn't fetch node: %v", err.Error())
		return err
	}
	if d, ok := node.Labels["node.kubernetes.io/instance-type"]; ok {
		podResult.InstanceType = d
	}
	lock.Lock()
	result.Pods = append(result.Pods, podResult)
	lock.Unlock()
	log.Debugf("%s: avgRps: %.0f avgLatency: %.0f ms", podResult.Name, podResult.AvgRps, podResult.AvgLatency/1000)
	return nil
}

func normalizeResults(result *tools.Result) {
	result.StatusCodes = make(map[int]int64)
	for _, pod := range result.Pods {
		result.TotalAvgRps += pod.AvgRps
		result.StdevRps += pod.StdevRps
		result.AvgLatency += pod.AvgLatency
		result.StdevLatency += pod.StdevLatency
		result.HTTPErrors += pod.HTTPErrors
		result.ReadErrors += pod.ReadErrors
		result.WriteErrors += pod.WriteErrors
		result.Requests += pod.Requests
		result.Timeouts += pod.Timeouts
		if pod.MaxLatency > result.MaxLatency {
			result.MaxLatency = pod.MaxLatency
		}
		result.P90Latency += pod.P90Latency
		result.P95Latency += pod.P95Latency
		result.P99Latency += pod.P99Latency
	}
	pods := float64(len(result.Pods))
	result.StdevRps = result.StdevRps / pods
	result.AvgLatency = result.AvgLatency / pods
	result.StdevLatency = result.StdevLatency / pods
	result.P90Latency = result.P90Latency / pods
	result.P95Latency = result.P95Latency / pods
	result.P99Latency = result.P99Latency / pods
	result.Version = fmt.Sprintf("%v@%v", version.Version, version.GitCommit)
}
