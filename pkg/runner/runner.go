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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cloud-bulldozer/go-commons/comparison"
	"github.com/cloud-bulldozer/go-commons/indexers"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/ocp-metadata"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
	"github.com/cloud-bulldozer/ingress-perf/pkg/runner/tools"
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
var currentTuning string

func Start(uuid, baseUUID, baseIndex string, tolerancy int, indexer *indexers.Indexer, cleanupAssets bool) error {
	var err error
	var kubeconfig string
	var benchmarkResult []tools.Result
	var comparator comparison.Comparator
	var clusterMetadata tools.ClusterMetadata
	passed := true
	if os.Getenv("KUBECONFIG") != "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); kubeconfig == "" && !os.IsNotExist(err) {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	restConfig.QPS = 200
	restConfig.Burst = 200
	clientSet = kubernetes.NewForConfigOrDie(restConfig)
	orClientSet = openshiftrouteclientset.NewForConfigOrDie(restConfig)
	dynamicClient = dynamic.NewForConfigOrDie(restConfig)
	ocpMetadata, err := ocpmetadata.NewMetadata(restConfig)
	if err != nil {
		return err
	}
	clusterMetadata.ClusterMetadata, err = ocpMetadata.GetClusterMetadata()
	if err != nil {
		return err
	}
	clusterMetadata.HAProxyVersion, err = getHAProxyVersion()
	log.Infof("HAProxy version: %s", clusterMetadata.HAProxyVersion)
	if err != nil {
		return err
	}
	if indexer != nil {
		if _, ok := (*indexer).(*indexers.Elastic); ok {
			comparator = comparison.NewComparator(*indexers.ESClient, baseIndex)
		}
	}
	if err := deployAssets(); err != nil {
		return err
	}
	for i, cfg := range config.Cfg {
		cfg.UUID = uuid
		log.Infof("Running test %d/%d", i+1, len(config.Cfg))
		log.Infof("Tool:%s termination:%v servers:%d concurrency:%d procs:%d connections:%d duration:%v",
			cfg.Tool,
			cfg.Termination,
			cfg.ServerReplicas,
			cfg.Concurrency,
			cfg.Procs,
			cfg.Connections,
			cfg.Duration,
		)
		if err := reconcileNs(cfg); err != nil {
			return err
		}
		if cfg.Tuning != "" {
			currentTuning = cfg.Tuning
			if err = applyTunning(cfg.Tuning); err != nil {
				return err
			}
		}
		if benchmarkResult, err = runBenchmark(cfg, clusterMetadata); err != nil {
			return err
		}
		if indexer != nil {
			if !cfg.Warmup {
				var benchmarkResultDocuments []interface{}
				for _, res := range benchmarkResult {
					benchmarkResultDocuments = append(benchmarkResultDocuments, res)
				}
				msg, err := (*indexer).Index(benchmarkResultDocuments, indexers.IndexingOpts{
					MetricName: uuid,
				})
				if err != nil {
					return err
				}
				log.Info(msg)
				if baseUUID != "" {
					log.Infof("Comparing total_avg_rps with baseline: %v in index %s", baseUUID, baseIndex)
					var totalAvgRps float64
					query := fmt.Sprintf("uuid.keyword: %s AND config.termination.keyword: %s AND config.concurrency: %d AND config.connections: %d AND config.serverReplicas: %d AND config.path.keyword: \\%s",
						baseUUID, cfg.Termination, cfg.Concurrency, cfg.Connections, cfg.ServerReplicas, cfg.Path)
					log.Debugf("Query: %s", query)
					for _, b := range benchmarkResult {
						totalAvgRps += b.TotalAvgRps
					}
					totalAvgRps = totalAvgRps / float64(len(benchmarkResult))
					msg, err := comparator.Compare("total_avg_rps", query, comparison.Avg, totalAvgRps, tolerancy)
					if err != nil {
						log.Error(err.Error())
						passed = false
					} else {
						log.Info(msg)
					}
				}
			} else {
				log.Info("Warmup is enabled, skipping indexing")
			}
		}
	}
	if cleanupAssets {
		if cleanup(10*time.Minute) != nil {
			return err
		}
	}
	if passed {
		return nil
	}
	return fmt.Errorf("some benchmark comparisons failed")
}

func cleanup(timeout time.Duration) error {
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
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: benchmarkNs, Labels: map[string]string{
		"pod-security.kubernetes.io/warn":                "privileged",
		"pod-security.kubernetes.io/audit":               "privileged",
		"pod-security.kubernetes.io/enforce":             "privileged",
		"security.openshift.io/scc.podSecurityLabelSync": "false",
	}}}
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
		return waitForDeployment(benchmarkNs, deployment.Name, time.Minute)
	}
	if err := f(server, cfg.ServerReplicas); err != nil {
		return err
	}
	return f(client, cfg.Concurrency)
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
