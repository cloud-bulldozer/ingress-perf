package runner

import (
	"bytes"
	"context"
	"fmt"

	"github.com/rsevilla87/ingress-perf/pkg/config"
	_ "github.com/rsevilla87/ingress-perf/pkg/log"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func runBenchmark(cfg config.Config, testIndex int) error {
	benchmarkOutput := make(chan (string))
	var ep string
	r, err := orClientSet.RouteV1().Routes(benchmarkNs).Get(context.TODO(), fmt.Sprintf("%s-%s", serverName, cfg.Termination), metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cfg.Termination == "http" {
		ep = fmt.Sprintf("http://%v%v", r.Spec.Host, cfg.Path)
	} else {
		ep = fmt.Sprintf("https://%v%v", r.Spec.Host, cfg.Path)
	}
	cmd := []string{"wrk", "-c", fmt.Sprint(cfg.Connections), "-d", fmt.Sprintf("%v", cfg.Duration), "--latency", ep}
	clientPods, err := clientSet.CoreV1().Pods(benchmarkNs).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", clientName),
	})
	if err != nil {
		return err
	}
	for _, pod := range clientPods.Items {
		fmt.Println(pod.Name)
		go exec(context.TODO(), pod.Name, cmd, benchmarkOutput)
	}
	for range clientPods.Items {
		fmt.Println(<-benchmarkOutput)
	}
	return nil
}

func exec(ctx context.Context, pod string, cmd []string, output chan (string)) {
	var stdout bytes.Buffer
	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(benchmarkNs).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: clientName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		Command:   cmd,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		log.Error(err.Error())
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stdout,
		Tty:    true,
	})
	if err != nil {
		log.Errorf("Exec failed, skipping: %v", err.Error())
		return
	}
	output <- stdout.String()
}
