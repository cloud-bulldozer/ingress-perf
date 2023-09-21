package runner

import (
	"bytes"
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func getHAProxyVersion() (string, error) {
	var stdout, stderr bytes.Buffer
	podList, err := clientSet.CoreV1().Pods("openshift-ingress").List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "ingresscontroller.operator.openshift.io/deployment-ingresscontroller=default",
			FieldSelector: "status.phase=Running"},
	)
	if err != nil {
		return "", err
	}
	routerPod := podList.Items[0]
	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(routerPod.Name).
		Namespace(routerPod.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: "router",
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		Command:   []string{"bash", "-c", "rpm -qa | grep haproxy"},
		TTY:       false,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		log.Error(err.Error())
		return "", err
	}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(stdout.String(), "\n"), err
}
