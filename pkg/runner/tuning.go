package runner

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ApplyTunning applies the given json merge patch to the default ingresscontroller CR
// and then waits for the ingres-controller deployment reconciliation to take place
func ApplyTunning(tuningPatch string) error {
	log.Infof("Applying tuning patch to ingress controller: %v", tuningPatch)
	_, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1",
		Resource: "ingresscontrollers",
	}).Namespace("openshift-ingress-operator").Patch(context.TODO(), "default", types.MergePatchType, []byte(tuningPatch), v1.PatchOptions{})
	if err != nil {
		return err
	}
	time.Sleep(5 * time.Second) // ingress-controller operator takes some time to reconcile the deployment
	return waitForDeployment("openshift-ingress", "router-default", 5*time.Minute)
}
