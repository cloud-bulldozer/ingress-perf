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
	"sync"
	"time"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/ocp-metadata"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
	"github.com/cloud-bulldozer/ingress-perf/pkg/runner/tools"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var lock = &sync.Mutex{}

func runBenchmark(cfg config.Config, clusterMetadata ocpmetadata.ClusterMetadata) ([]tools.Result, error) {
	var benchmarkResult []tools.Result
	var clientPods []corev1.Pod
	var wg sync.WaitGroup
	var ep string
	var tool tools.Tool
	r, err := orClientSet.RouteV1().Routes(benchmarkNs).Get(context.TODO(), fmt.Sprintf("%s-%s", serverName, cfg.Termination), metav1.GetOptions{})
	if err != nil {
		return benchmarkResult, err
	}
	if cfg.Termination == "http" {
		ep = fmt.Sprintf("http://%v%v", r.Spec.Host, cfg.Path)
	} else {
		ep = fmt.Sprintf("https://%v%v", r.Spec.Host, cfg.Path)
	}
	tool, err = tools.New(cfg, ep)
	if err != nil {
		return benchmarkResult, err
	}
	allClientPods, err := clientSet.CoreV1().Pods(benchmarkNs).List(context.TODO(), metav1.ListOptions{
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
		result := tools.Result{
			UUID:            cfg.UUID,
			Sample:          i,
			Config:          cfg,
			Timestamp:       ts,
			ClusterMetadata: clusterMetadata,
		}
		log.Infof("Running sample %d/%d: %v", i, cfg.Samples, cfg.Duration)
		for _, pod := range clientPods {
			wg.Add(1)
			go exec(context.TODO(), &wg, tool, pod, &result)
		}
		wg.Wait()
		genResultSummary(&result)
		log.Infof("Summary for %s: Rps=%.0f avgLatency=%.0f μs P99Latency=%.0f μs", cfg.Termination, result.TotalAvgRps, result.AvgLatency, result.P99Latency)
		benchmarkResult = append(benchmarkResult, result)
		if cfg.Delay != 0 {
			log.Info("Sleeping for ", cfg.Delay)
			time.Sleep(cfg.Delay)
		}
	}
	return benchmarkResult, nil
}

func exec(ctx context.Context, wg *sync.WaitGroup, tool tools.Tool, pod corev1.Pod, result *tools.Result) {
	defer wg.Done()
	var stdout, stderr bytes.Buffer
	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(benchmarkNs).
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
		return
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		log.Errorf("Exec failed, skipping: %v", err.Error())
		return
	}
	podResult, err := tool.ParseResult(stdout.String(), stderr.String())
	if err != nil {
		log.Errorf("Result parsing failed, skipping: %v", err.Error())
		return
	}
	podResult.Name = pod.Name
	podResult.Node = pod.Spec.NodeName
	node, err := clientSet.CoreV1().Nodes().Get(context.TODO(), podResult.Node, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Couldn't fetch node: %v", err.Error())
		return
	}
	if d, ok := node.Labels["node.kubernetes.io/instance-type"]; ok {
		podResult.InstanceType = d
	}
	lock.Lock()
	result.Pods = append(result.Pods, podResult)
	lock.Unlock()
	log.Debugf("%s: avgRps: %.0f avgLatency: %.0f μs", podResult.Name, podResult.AvgRps, podResult.AvgLatency)
}

func genResultSummary(result *tools.Result) {
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
		result.P90Latency += float64(pod.P90Latency)
		result.P95Latency += float64(pod.P95Latency)
		result.P99Latency += float64(pod.P99Latency)
	}
	result.StdevRps = result.StdevRps / float64(len(result.Pods))
	result.AvgLatency = result.AvgLatency / float64(len(result.Pods))
	result.StdevLatency = result.StdevLatency / float64(len(result.Pods))
	result.P90Latency = result.P90Latency / float64(len(result.Pods))
	result.P95Latency = result.P95Latency / float64(len(result.Pods))
	result.P99Latency = result.P99Latency / float64(len(result.Pods))
}
