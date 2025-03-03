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

	"github.com/cloud-bulldozer/go-commons/indexers"
	"github.com/cloud-bulldozer/go-commons/prometheus"

	ocpmetadata "github.com/cloud-bulldozer/go-commons/ocp-metadata"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
	"github.com/cloud-bulldozer/ingress-perf/pkg/runner/tools"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	v1 "github.com/openshift/api/route/v1"
	openshiftrouteclientset "github.com/openshift/client-go/route/clientset/versioned"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/tools/clientcmd"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	gatewayApiClientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

var restConfig *rest.Config
var clientSet *kubernetes.Clientset
var istioClient *istioclient.Clientset
var dynamicClient *dynamic.DynamicClient
var orClientSet *openshiftrouteclientset.Clientset
var hrClientSet *gatewayApiClientset.Clientset
var currentTuning string

func New(uuid string, cleanup bool, opts ...OptsFunctions) *Runner {
	r := &Runner{
		uuid:    uuid,
		cleanup: cleanup,
	}
	for _, opts := range opts {
		opts(r)
	}
	return r
}

func WithIndexer(esServer, esIndex, resultsDir string, podMetrics bool, esInsecureSkipVerify bool) OptsFunctions {
	return func(r *Runner) {
		if esServer != "" || resultsDir != "" {
			var indexerCfg indexers.IndexerConfig
			if esServer != "" {
				indexerCfg = indexers.IndexerConfig{
					Type:    indexers.ElasticIndexer,
					Servers: []string{esServer},
					Index:   esIndex,
					InsecureSkipVerify: esInsecureSkipVerify,
				}
			} else if resultsDir != "" {
				indexerCfg = indexers.IndexerConfig{
					Type:             indexers.LocalIndexer,
					MetricsDirectory: resultsDir,
				}
			}
			log.Infof("Creating %s indexer", indexerCfg.Type)
			indexer, err := indexers.NewIndexer(indexerCfg)
			if err != nil {
				log.Fatal(err)
			}
			r.indexer = indexer
			r.podMetrics = podMetrics
		}
	}
}

func WithServiceMesh(enable bool, igNamespace string) OptsFunctions {
	return func(r *Runner) {
		r.serviceMesh = enable
		r.igNamespace = igNamespace
		config.PrometheusQueries["avg_cpu_usage_ingress_gateway_pods"] =
			fmt.Sprintf("avg(avg_over_time(sum(irate(container_cpu_usage_seconds_total{name!='', namespace='%s', pod=~'istio-ingressgateway.+'}[2m])) by (pod)[ELAPSED:]))", igNamespace)
		config.PrometheusQueries["avg_memory_usage_ingress_gateway_pods_bytes"] =
			fmt.Sprintf("avg(avg_over_time(sum(container_memory_working_set_bytes{name!='', namespace='%s', pod=~'istio-ingressgateway.+'}) by (pod)[ELAPSED:]))", igNamespace)
	}
}

func WithGatewayAPI(enable bool) OptsFunctions {
	return func(r *Runner) {
		r.gatewayAPI = enable
	}
}

func (r *Runner) Start() error {
	var err error
	var kubeconfig string
	var benchmarkResult []tools.Result
	var clusterMetadata tools.ClusterMetadata
	var benchmarkResultDocuments []interface{}
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
	istioClient = istioclient.NewForConfigOrDie(restConfig)
	orClientSet = openshiftrouteclientset.NewForConfigOrDie(restConfig)
	hrClientSet = gatewayApiClientset.NewForConfigOrDie(restConfig)
	dynamicClient = dynamic.NewForConfigOrDie(restConfig)
	ocpMetadata, err := ocpmetadata.NewMetadata(restConfig)
	if err != nil {
		return err
	}
	clusterMetadata.ClusterMetadata, err = ocpMetadata.GetClusterMetadata()
	if err != nil {
		return err
	}
	promURL, promToken, err := ocpMetadata.GetPrometheus()
	if err != nil {
		log.Error("Error fetching prometheus information")
		return err
	}
	p, err := prometheus.NewClient(promURL, promToken, "", "", true)
	if err != nil {
		log.Error("Error creating prometheus client")
		return err
	}
	clusterMetadata.HAProxyVersion, err = getHAProxyVersion()
	if err != nil {
		log.Errorf("Couldn't fetch haproxy version: %v", err)
	} else {
		log.Infof("HAProxy version: %s", clusterMetadata.HAProxyVersion)
	}
	if err := r.deployAssets(); err != nil {
		return err
	}
	for i, cfg := range config.Cfg {
		cfg.UUID = r.uuid
		log.Infof("Running test %d/%d", i+1, len(config.Cfg))
		log.Infof("Tool:%s termination:%v servers:%d concurrency:%d procs:%d connections:%d duration:%v http2:%v",
			cfg.Tool,
			cfg.Termination,
			cfg.ServerReplicas,
			cfg.Concurrency,
			cfg.Procs,
			cfg.Connections,
			cfg.Duration,
			cfg.HTTP2,
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
		if benchmarkResult, err = runBenchmark(cfg, clusterMetadata, p, r.podMetrics, r.gatewayAPI); err != nil {
			return err
		}
		if r.indexer != nil && !cfg.Warmup {
			for _, res := range benchmarkResult {
				benchmarkResultDocuments = append(benchmarkResultDocuments, res)
			}
			// When not using local indexer, empty the documents array when all documents after indexing them
			if _, ok := (*r.indexer).(*indexers.Local); !ok {
				if indexDocuments(*r.indexer, benchmarkResultDocuments, indexers.IndexingOpts{}) != nil {
					log.Errorf("Indexing error: %v", err.Error())
				}
				benchmarkResultDocuments = []interface{}{}
			}
		}
	}
	if _, ok := (*r.indexer).(*indexers.Local); r.indexer != nil && ok {
		if err := indexDocuments(*r.indexer, benchmarkResultDocuments, indexers.IndexingOpts{MetricName: r.uuid}); err != nil {
			log.Errorf("Indexing error: %v", err.Error())
		}
	}
	if r.cleanup {
		if cleanup(10*time.Minute) != nil {
			return err
		}
	}
	if passed {
		return nil
	}
	return fmt.Errorf("some benchmark comparisons failed")
}

func indexDocuments(indexer indexers.Indexer, documents []interface{}, indexingOpts indexers.IndexingOpts) error {
	msg, err := indexer.Index(documents, indexingOpts)
	if err != nil {
		return err
	}
	log.Info(msg)
	return nil
}

func cleanup(timeout time.Duration) error {
	log.Info("Cleaning up resources")
	if err := clientSet.CoreV1().Namespaces().Delete(context.TODO(), benchmarkNs.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}
	err := wait.PollUntilContextTimeout(context.TODO(), time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), benchmarkNs.Name, metav1.GetOptions{})
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

//nolint:gocyclo
func (r *Runner) deployAssets() error {
	log.Infof("Deploying benchmark assets")
	if r.serviceMesh {
		log.Info("Service mesh mode enabled")
		benchmarkNs.Labels["istio-injection"] = "enabled"
	} else if r.gatewayAPI {
		log.Info("Gateway API mode enabled")
	}
	_, err := clientSet.CoreV1().Namespaces().Create(context.TODO(), &benchmarkNs, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.AppsV1().Deployments(benchmarkNs.Name).Create(context.TODO(), &server, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), &clientCRB, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.AppsV1().Deployments(benchmarkNs.Name).Create(context.TODO(), &client, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	_, err = clientSet.CoreV1().Services(benchmarkNs.Name).Create(context.TODO(), &service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if !r.gatewayAPI {
		for _, route := range routes {
			if r.serviceMesh {
				route.Spec.To = v1.RouteTargetReference{
					Name: "istio-ingressgateway",
				}
				route.Spec.Port.TargetPort = intstr.FromString("http2")
				routesNamespace = r.igNamespace
			}
			_, err := orClientSet.RouteV1().Routes(routesNamespace).Create(context.TODO(), &route, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	if r.serviceMesh {
		routes, _ := orClientSet.RouteV1().Routes(routesNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=ingress-perf"})
		for _, r := range routes.Items {
			ingressGateway.Spec.Servers[0].Hosts = append(ingressGateway.Spec.Servers[0].Hosts, r.Spec.Host)
		}
		_, err = istioClient.NetworkingV1beta1().Gateways(benchmarkNs.Name).Create(context.TODO(), &ingressGateway, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		_, err = istioClient.NetworkingV1beta1().VirtualServices(benchmarkNs.Name).Create(context.TODO(), &virtualService, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	if r.gatewayAPI {
		ocpMetadata, _ := ocpmetadata.NewMetadata(restConfig)
		ingressDomain, err = ocpMetadata.GetDefaultIngressDomain()
		if err != nil {
			return err
		}
		listenerHostName = gatewayv1beta1.Hostname("*.gwapi." + ingressDomain)
		httproutes.Spec.Hostnames = append(httproutes.Spec.Hostnames, gatewayv1beta1.Hostname("nginx.gwapi."+ingressDomain))
		log.Debugf("Creating GatewayClass...")
		_, err = hrClientSet.GatewayV1beta1().GatewayClasses().Create(context.TODO(), gatewayClass, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		time.Sleep(5 * time.Second) // wait for ServiceMeshControlPlane to be ready
		log.Debugf("Creating Gateway...")
		_, err = hrClientSet.GatewayV1beta1().Gateways(string(gatewayNamespace)).Create(context.TODO(), &gateway, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		log.Debugf("Waiting 4 minutes for Gateway to be ready...")
		time.Sleep(4 * time.Minute)
		log.Debugf("Creating HTTPRoute...")
		_, err := hrClientSet.GatewayV1beta1().HTTPRoutes(routesNamespace).Create(context.TODO(), &httproutes, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func reconcileNs(cfg config.Config) error {
	f := func(deployment appsv1.Deployment, replicas int32) error {
		d, err := clientSet.AppsV1().Deployments(benchmarkNs.Name).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if d.Status.ReadyReplicas == replicas {
			return nil
		}
		deployment.Spec.Replicas = &replicas
		_, err = clientSet.AppsV1().Deployments(benchmarkNs.Name).Update(context.TODO(), &deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return waitForDeployment(benchmarkNs.Name, deployment.Name, time.Minute)
	}
	if err := f(server, cfg.ServerReplicas); err != nil {
		return err
	}
	return f(client, cfg.Concurrency)
}

func waitForDeployment(ns, deployment string, maxWaitTimeout time.Duration) error {
	var errMsg string
	var dep *appsv1.Deployment
	var err error
	log.Infof("Waiting for replicas from deployment %s in ns %s to be ready", deployment, ns)
	err = wait.PollUntilContextTimeout(context.TODO(), time.Second, maxWaitTimeout, true, func(ctx context.Context) (bool, error) {
		dep, err = clientSet.AppsV1().Deployments(ns).Get(context.TODO(), deployment, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if *dep.Spec.Replicas != dep.Status.ReadyReplicas || *dep.Spec.Replicas != dep.Status.AvailableReplicas {
			errMsg = fmt.Sprintf("%d/%d replicas ready", dep.Status.AvailableReplicas, *dep.Spec.Replicas)
			log.Debug(errMsg)
			return false, nil
		}
		log.Debugf("%d replicas from deployment %s ready", dep.Status.UpdatedReplicas, deployment)
		return true, nil
	})
	if err != nil && errMsg != "" {
		log.Error(errMsg)
		failedPods, _ := clientSet.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
			FieldSelector: "status.phase=Pending",
			LabelSelector: labels.SelectorFromSet(dep.Spec.Selector.MatchLabels).String(),
		})
		for _, pod := range failedPods.Items {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Waiting != nil {
					log.Errorf("%v@%v: %v", pod.Name, pod.Spec.NodeName, cs.State.Waiting.Message)
				}
			}
		}
	}
	return err
}
