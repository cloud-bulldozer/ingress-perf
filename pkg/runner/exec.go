package runner

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/ocp-metadata"
	"github.com/rsevilla87/ingress-perf/pkg/config"
	"github.com/rsevilla87/ingress-perf/pkg/runner/tools"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var lock = &sync.Mutex{}

func runBenchmark(cfg config.Config, clusterMetadata ocpmetadata.ClusterMetadata) ([]interface{}, error) {
	var benchmarkResult []interface{}
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
	for i := 1; i <= cfg.Samples; i++ {
		result := tools.Result{
			UUID:            cfg.UUID,
			Sample:          i,
			Config:          cfg,
			Timestamp:       time.Now().UTC(),
			ClusterMetadata: clusterMetadata,
		}
		log.Infof("Running sample %d/%d: %v", i, cfg.Samples, cfg.Duration)
		for _, pod := range clientPods {
			wg.Add(1)
			go exec(context.TODO(), &wg, tool, pod, &result)
		}
		wg.Wait()
		genResultSummary(&result)
		log.Infof("Summary: Rps=%.2f req/s avgLatency=%.2f μs P99Latency=%.2f μs", result.TotalAvgRps, result.AvgLatency, result.P99Latency)
		benchmarkResult = append(benchmarkResult, result)
		if cfg.SampleDelay != 0 {
			log.Info("Sleeping for ", cfg.SampleDelay)
			time.Sleep(cfg.SampleDelay)
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
	log.Debugf("%s: avgRps: %.2f req/s avgLatency: %.2f μs", podResult.Name, podResult.AvgRps, podResult.AvgLatency)
}

func genResultSummary(result *tools.Result) {
	for _, pod := range result.Pods {
		result.TotalAvgRps += pod.AvgRps
		result.StdevRps += pod.StdevRps
		result.AvgLatency += pod.AvgLatency
		result.StdevLatency += pod.StdevLatency
		result.Errors += pod.Errors
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
