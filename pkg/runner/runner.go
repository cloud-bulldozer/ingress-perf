package runner

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/cloud-bulldozer/go-commons/indexers"
	ocpmetadata "github.com/cloud-bulldozer/go-commons/ocp-metadata"
	"github.com/rsevilla87/ingress-perf/pkg/config"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	openshiftrouteclientset "github.com/openshift/client-go/route/clientset/versioned"
	"k8s.io/client-go/tools/clientcmd"
)

var restConfig *rest.Config
var clientSet *kubernetes.Clientset
var dynamicClient *dynamic.DynamicClient
var orClientSet *openshiftrouteclientset.Clientset

func Start(uuid string, indexer *indexers.Indexer) error {
	var err error
	var result []interface{}
	var kubeconfig string
	log.Info("Starting ingress-perf")
	if os.Getenv("KUBECONFIG") != "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); kubeconfig == "" && !os.IsNotExist(err) {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	restConfig.QPS = 50
	restConfig.Burst = 50
	clientSet = kubernetes.NewForConfigOrDie(restConfig)
	orClientSet = openshiftrouteclientset.NewForConfigOrDie(restConfig)
	dynamicClient = dynamic.NewForConfigOrDie(restConfig)
	ocpMetadata, err := ocpmetadata.NewMetadata(restConfig)
	if err != nil {
		return err
	}
	clusterMetadata, err := ocpMetadata.GetClusterMetadata()
	if err != nil {
		return err
	}
	if err := deployAssets(); err != nil {
		return err
	}
	for i, cfg := range config.Cfg {
		cfg.UUID = uuid
		log.Infof("Running test %d/%d: %v", i+1, len(config.Cfg), cfg.Termination)
		if err := reconcileNs(cfg); err != nil {
			return err
		}
		if cfg.Tuning != "" {
			if err = ApplyTunning(cfg.Tuning); err != nil {
				return err
			}
		}
		if result, err = runBenchmark(cfg, clusterMetadata); err != nil {
			return err
		}
		if indexer != nil {
			if !cfg.Warmup {
				msg, err := (*indexer).Index(result, indexers.IndexingOpts{})
				if err != nil {
					return err
				}
				log.Info(msg)
			} else {
				log.Info("Warmup is enabled, skipping indexing")
			}
		}
	}
	return nil
}

func Cleanup(timeout time.Duration) error {
	log.Info("Cleaning up resources")
	if err := clientSet.CoreV1().Namespaces().Delete(context.TODO(), benchmarkNs, metav1.DeleteOptions{}); err != nil {
		return err
	}
	err := wait.PollUntilContextTimeout(context.TODO(), time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), benchmarkNs, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	return clientSet.RbacV1().ClusterRoleBindings().Delete(context.Background(), clientCRB.Name, metav1.DeleteOptions{})
}

func deployAssets() error {
	log.Infof("Deploying benchmark assets")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: benchmarkNs}}
	_, err := clientSet.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.AppsV1().Deployments(benchmarkNs).Create(context.TODO(), &server, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), &clientCRB, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.AppsV1().Deployments(benchmarkNs).Create(context.TODO(), &client, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.CoreV1().Services(benchmarkNs).Create(context.TODO(), &service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	for _, route := range routes {
		_, err = orClientSet.RouteV1().Routes(benchmarkNs).Create(context.TODO(), &route, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func reconcileNs(cfg config.Config) error {
	f := func(deployment appsv1.Deployment, replicas int32) error {
		d, err := clientSet.AppsV1().Deployments(benchmarkNs).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if d.Spec.Replicas == &replicas {
			return nil
		}
		deployment.Spec.Replicas = &replicas
		_, err = clientSet.AppsV1().Deployments(benchmarkNs).Update(context.TODO(), &deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		if err := waitForDeployment(benchmarkNs, deployment.Name, time.Minute); err != nil {
			return err
		}
		return nil
	}
	if err := f(server, cfg.ServerReplicas); err != nil {
		return err
	}
	if err := f(client, cfg.Concurrency); err != nil {
		return err
	}
	return nil
}

func waitForDeployment(ns, deployment string, maxWaitTimeout time.Duration) error {
	return wait.PollUntilContextTimeout(context.TODO(), time.Second, maxWaitTimeout, true, func(ctx context.Context) (bool, error) {
		dep, err := clientSet.AppsV1().Deployments(ns).Get(context.TODO(), deployment, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if *dep.Spec.Replicas != dep.Status.ReadyReplicas || *dep.Spec.Replicas != dep.Status.AvailableReplicas {
			log.Debugf("Waiting for replicas from deployment %s in ns %s to be ready", deployment, ns)
			return false, nil
		}
		log.Debugf("%d replicas from deployment %s ready", dep.Status.UpdatedReplicas, deployment)
		return true, nil
	})
}
