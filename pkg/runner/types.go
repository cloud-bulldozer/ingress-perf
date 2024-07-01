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
	"fmt"

	"github.com/cloud-bulldozer/go-commons/indexers"
	routev1 "github.com/openshift/api/route/v1"
	"istio.io/api/networking/v1beta1"
	v1networking "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	serverImage = "quay.io/cloud-bulldozer/nginx:latest"
	serverName  = "nginx"
	clientImage = "quay.io/cloud-bulldozer/ingress-perf:latest"
	clientName  = "ingress-perf-client"
)

type Runner struct {
	uuid        string
	indexer     *indexers.Indexer
	podMetrics  bool
	cleanup     bool
	serviceMesh bool
	igNamespace string
}

type OptsFunctions func(r *Runner)

var routesNamespace = benchmarkNs.Name

var benchmarkNs = corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name: "ingress-perf",
		Labels: map[string]string{
			"pod-security.kubernetes.io/warn":                "privileged",
			"pod-security.kubernetes.io/audit":               "privileged",
			"pod-security.kubernetes.io/enforce":             "privileged",
			"security.openshift.io/scc.podSecurityLabelSync": "false",
		},
	},
}

var workerAffinity = &corev1.Affinity{
	NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "node-role.kubernetes.io/worker",
							Operator: corev1.NodeSelectorOpExists,
						},
						{
							Key:      "node-role.kubernetes.io/infra",
							Operator: corev1.NodeSelectorOpDoesNotExist,
						},
					},
				},
			},
		},
	},
}

var server = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: serverName,
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": serverName},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": serverName},
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
			},
			Spec: corev1.PodSpec{
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
					MaxSkew:           1,
					TopologyKey:       "kubernetes.io/hostname",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": serverName},
					},
				}},
				Affinity:                      workerAffinity,
				TerminationGracePeriodSeconds: ptr.To[int64](0), // It helps to kill the pod immediately on GC
				Containers: []corev1.Container{
					{
						Name:            serverName,
						Image:           serverImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To[bool](false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							RunAsNonRoot:             ptr.To[bool](true),
							SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
						},
						Ports: []corev1.ContainerPort{{Name: "http", Protocol: corev1.ProtocolTCP, ContainerPort: 8080}},
					},
				},
			},
		},
	},
}

var service = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: serverName,
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{"app": serverName},
		Type:     corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{
			{Name: "http", Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8080), Port: 8080},
			{Name: "https", Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8443), Port: 8443}},
	},
}

var client = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: clientName,
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": clientName},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": clientName},
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "true",
				},
			},
			Spec: corev1.PodSpec{
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
					MaxSkew:           1,
					TopologyKey:       "kubernetes.io/hostname",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": clientName},
					},
				}},
				Affinity:                      workerAffinity,
				TerminationGracePeriodSeconds: ptr.To[int64](0),
				HostNetwork:                   true, // Enable hostNetwork in client pods
				Containers: []corev1.Container{
					{
						Command:         []string{"sleep", "inf"},
						Name:            clientName,
						Image:           clientImage,
						ImagePullPolicy: corev1.PullAlways,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To[bool](false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							RunAsNonRoot:             ptr.To[bool](true),
							SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
						},
					},
				},
			},
		},
	},
}

var clientCRB = rbac.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: clientName,
	},
	Subjects: []rbac.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: benchmarkNs.Name,
		},
	},
	RoleRef: rbac.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Name:     "system:openshift:scc:hostnetwork-v2",
		Kind:     "ClusterRole",
	},
}

var routes = []routev1.Route{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-edge", serverName),
			Labels: map[string]string{
				"app": "ingress-perf",
			},
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("http")},
			To: routev1.RouteTargetReference{
				Name: service.Name,
			},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationEdge,
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-reencrypt", serverName),
			Labels: map[string]string{
				"app": "ingress-perf",
			},
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("https")},
			To: routev1.RouteTargetReference{
				Name: service.Name,
			},
			TLS: &routev1.TLSConfig{
				Termination:              routev1.TLSTerminationReencrypt,
				Certificate:              "-----BEGIN CERTIFICATE-----\nMIIDYzCCAkugAwIBAgIUf9Gp3JZWaHXqBhj4FAj/RHUsSjwwDQYJKoZIhvcNAQEL\nBQAwQTELMAkGA1UEBhMCRVMxDzANBgNVBAgMBk1hZHJpZDEPMA0GA1UEBwwGTWFk\ncmlkMRAwDgYDVQQKDAdSZWQgSGF0MB4XDTI0MDcwMTEyMDgyNVoXDTM0MDYyOTEy\nMDgyNVowQTELMAkGA1UEBhMCRVMxDzANBgNVBAgMBk1hZHJpZDEPMA0GA1UEBwwG\nTWFkcmlkMRAwDgYDVQQKDAdSZWQgSGF0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\nMIIBCgKCAQEAxm2CnuQ9xzzcorvLDiyFtYU2q21NTi3Nbu7X2U7hSKxTxXg7qkPh\nMJXVjV2kAGB5LBS/LYsEThdDq0M6Z+hEMjBQpYWTCTaR6rHJadXcO6V8SaavpkH6\nOVQCnIbYL0maJPn4dE/uAA05a4Lmd7Iu3MXMI4pIrQ/HuIsFBy/0SoGg37K+S4/Y\nHc9Cj6tfoNDMmGSGn7Dra8XpOiClKXesUevMrMFHR1Hm2V+eOZ+EEaNz5HlYYScR\n31YR5lkega8A10+2WujER9F5B4lyU2fvk1USCM/CgTjUCiqDs6SINBGSrx2pDKyx\niJsNOIkxsqS3qboGjpCKVXPvm7Ihh40zcwIDAQABo1MwUTAdBgNVHQ4EFgQUnTjL\nLrNBgnHQOrY1jsgZHsw+GzYwHwYDVR0jBBgwFoAUnTjLLrNBgnHQOrY1jsgZHsw+\nGzYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAqoT5DV40AYw7\ne0TBQxJqr+xMrQF/K5z7Qy2HNN2oTox4gMp580APxR619wbGX+bQ2WrdEGz+uRkS\nyX62hGqK+bIa7PTPRWHhoIfrbAxwYnsSLhExQ0aSbVuOzwRegtZTEh2KC6et+Rbb\n0JbLYhL0RhJQ3z6lbw3OEqRBkA59E2Myf0l7wnmM7QTyUVOVEJVkwQdb4juqjJrp\nX3JizQybbbWK7GTU7zXBPOQncraOg/rFygsiSgYjaPMmAgjMdOat3BbN5gkcwaa+\nYhuAsqFFAxUJKN+jdJPZ45FZ9O0Q6jMa0dCLprfkaGoNKxu6Bk072yAkJaaN4Srh\nQ1c2f3vQ+A==\n-----END CERTIFICATE-----",
				Key:                      "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDGbYKe5D3HPNyi\nu8sOLIW1hTarbU1OLc1u7tfZTuFIrFPFeDuqQ+EwldWNXaQAYHksFL8tiwROF0Or\nQzpn6EQyMFClhZMJNpHqsclp1dw7pXxJpq+mQfo5VAKchtgvSZok+fh0T+4ADTlr\nguZ3si7cxcwjikitD8e4iwUHL/RKgaDfsr5Lj9gdz0KPq1+g0MyYZIafsOtrxek6\nIKUpd6xR68yswUdHUebZX545n4QRo3PkeVhhJxHfVhHmWR6BrwDXT7Za6MRH0XkH\niXJTZ++TVRIIz8KBONQKKoOzpIg0EZKvHakMrLGImw04iTGypLepugaOkIpVc++b\nsiGHjTNzAgMBAAECggEAFPX+hWBg8koImJwz4ARt/3XaB5PdRbPq7pwGays8+xtR\nIhwIWX0gLay2SV+p8mw+gJyUcXAs/T7R5CaO5ekQQyj9a+7HUILWOS+csjRoNljN\n303fYjW4BBROzlAkSCKDSQKq70Z+Qr0i4quKQvysnJ+8Z8i00qtHEsSEvHShXrlR\nGbJY7snrMVGYlMpfZ/d6INg3DNQ8Qex6gCtPw+XDtZHrGTLQJGO/a+ELqgs8Tpg9\n5THLsh0nNIrrxeSzYCLBzhN/PfriliEoYHOR8/LQDFe/KGdeXEeZUX0sm4hnb+pk\n/wGSJFHrUCsNmba/NMdaJZtc+sJ3MD0HLsHvlOH2EQKBgQDwHSxalf0oHY1dAjfm\nNvpL4wn3BaEq+Nizi2ToV17w0/eB/JD5LCR2mjeBqcvdA3DLzXDnuV7dH4DnreZS\nmMWnwNuzH+t8NIsfe9joMBlkFVRb16+RHoSkQsyer++N0yVJZ3eBiBqrRfyNf2y/\nWxtyNGnvkTfAW9aD91vUgbjeCwKBgQDTjkuSJhpl7McYh/rhkYUSwqc4esR2AYY1\nshF+D5vVqQK5ursSV4esF1nTv5s1+4Dr2tgKJ7tv0JRBq+nuKFx2mjt2ESmPeslF\nVUJ6QU4G/lyVf2Oc5OfSs5Cy9PPEozGylDW8NhA+nJxJ7a5nC02zxAVC4NH36bTP\n5dIzxfwpOQKBgQDRXg7Dug8xQHt4Aywsz/l13DVfMIh0yOFjxQMq2KxPP/SMYmci\n7cupoxUNOboqMArVmoB9fZRrKUIkQe0YTl/iw3aOfSG/fq1kuX/MDczGCRcysiJD\nXqH3/Msg/3XZa6gSlmaNCtl+SJtL4jU3scB/15Ybr7s2Pp4RGcn3xVkmOwKBgCyy\n2QbFJDi64YouWPTGAV7oX9qQHehtBAVeXVHMTHnFXv0NFi+W7OSQfns99bZfMGsk\n7xaeNH04jcqeXyvhvT+V8NBOagCVyebrT8Ny/uB9Z+RyaQKiXtTm+cDF/qdib8Ky\ni/P7XE1OW7tHC91bxwwfc5kg1VxTNc/jWBl2UIQJAoGANSnfBkhK94rFVsW3HSYf\no8w85f/Xwea4kTwUVgzIV0DvlVQq4Pyk75pKjYLog+TMiKbTRKMNaQYxi/82FJcE\nOnoR0J2QymnLpsGdVBlvrR5x8sPXxnzoW6+T64aKXQVVUYq6I1iGCDxhRsm63sLD\nV0h3fmq3FZw95Y0bjPY4BZ4=\n-----END PRIVATE KEY-----",
				DestinationCACertificate: "-----BEGIN CERTIFICATE-----\nMIIDbTCCAlWgAwIBAgIJAJR/jN0Oa+/rMA0GCSqGSIb3DQEBCwUAME0xCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMQswCQYDVQQHDAJOWTEcMBoGA1UE\nCgwTRGVmYXVsdCBDb21wYW55IEx0ZDAeFw0xNzAxMjQwODExMDJaFw0yNzAxMjIw\nODExMDJaME0xCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMQswCQYD\nVQQHDAJOWTEcMBoGA1UECgwTRGVmYXVsdCBDb21wYW55IEx0ZDCCASIwDQYJKoZI\nhvcNAQEBBQADggEPADCCAQoCggEBAMItGS9sSafyqBuOcQcQ5j7OQ0EwF9qOckhl\nfT8VzUbcOy8/L/w654MpLEa4O4Fiek3keE7SDWGVtGZWDvT9y1QUxPhkDWq1Y3rr\nyMelv1xRIyPVD7EEicga50flKe8CKd1U3D6iDQzq0uxZZ6I/VArXW/BZ4LfPauzN\n9EpCYyKq0fY7WRFIGouO9Wu800nxcHptzhLAgSpO97aaZ+V+jeM7n7fchRSNrpIR\nzPBl/lIBgCPJgkax0tcm4EIKIwlG+jXWc5mvV8sbT8rAv32HVuaP6NafyWXXP3H1\noBf2CQCcwuM0sM9ZeZ5JEDF/7x3eNtqSt1X9HjzVpQjiVBXY+E0CAwEAAaNQME4w\nHQYDVR0OBBYEFOXxMHAA1qaKWlP+gx8tKO2rQ81WMB8GA1UdIwQYMBaAFOXxMHAA\n1qaKWlP+gx8tKO2rQ81WMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEB\nAJAri7Pd0eSY/rvIIvAvjhDPvKt6gI5hJEUp+M3nWTWA/IhQFYutb9kkZGhbBeLj\nqneJa6XYKaCcUx6/N6Vvr3AFqVsbbubbejRpdpXldJC33QkwaWtTumudejxSon24\nW/ANN/3ILNJVMouspLRGkFfOYp3lq0oKAlNZ5G3YKsG0znAfqhAVtqCTG9RU24Or\nxzkEaCw8IY5N4wbjCS9FPLm7zpzdg/M3A/f/vrIoGdns62hzjzcp0QVTiWku74M8\nv7/XlUYYvXOvPQCCHgVjnAZlnjcxMTBbwtdwfxjAmdNTmFFpASnf0s3b287zQwVd\nIeSydalVtLm7rBRZ59/2DYo=\n-----END CERTIFICATE-----",
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-passthrough", serverName),
			Annotations: map[string]string{
				// Passthrough terminations default balance strategy is source, this strategy is not suitable for
				// performance testing when concurrency is higher than 1, as all the requests will use the same source IP
				"haproxy.router.openshift.io/balance": "random",
			},
			Labels: map[string]string{
				"app": "ingress-perf",
			},
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("https")},
			To: routev1.RouteTargetReference{
				Name: service.Name,
			},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationPassthrough,
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-http", serverName),
			Labels: map[string]string{
				"app": "ingress-perf",
			},
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("http")},
			To: routev1.RouteTargetReference{
				Name: service.Name,
			},
		},
	},
}

var ingressGateway = v1networking.Gateway{
	ObjectMeta: metav1.ObjectMeta{
		Name: "gateway",
	},
	Spec: v1beta1.Gateway{
		Selector: map[string]string{
			"istio": "ingressgateway",
		},
		Servers: []*v1beta1.Server{
			{
				Port: &v1beta1.Port{
					Number:   80,
					Protocol: "HTTP",
					Name:     "http",
				},
			},
		},
	},
}

var virtualService = v1networking.VirtualService{
	ObjectMeta: metav1.ObjectMeta{
		Name: "http",
	},
	Spec: v1beta1.VirtualService{
		Hosts: []string{
			"*",
		},
		Gateways: []string{
			ingressGateway.Name,
		},
		Http: []*v1beta1.HTTPRoute{
			{
				Route: []*v1beta1.HTTPRouteDestination{
					{
						Destination: &v1beta1.Destination{
							Host: service.Name,
							Port: &v1beta1.PortSelector{
								Number: 8080,
							},
						},
					},
				},
			},
		},
	},
}
