package runner

import (
	"fmt"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	benchmarkNs = "ingress-perf"
	serverImage = "quay.io/cloud-bulldozer/nginx:latest"
	serverName  = "nginx"
	//clientImage = "quay.io/cloud-bulldozer/ingress-toolbox:latest"
	clientImage = "quay.io/cloud-bulldozer/wrk:latest"
	clientName  = "ingress-perf-client"
	waitPeriod  = 10 * time.Second
)

var workerAffinity = &corev1.Affinity{
	NodeAffinity: &corev1.NodeAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
			{
				Weight: 100,
				Preference: corev1.NodeSelectorTerm{
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
		Name:      serverName,
		Namespace: benchmarkNs,
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
			},
			Spec: corev1.PodSpec{
				Affinity:                      workerAffinity,
				TerminationGracePeriodSeconds: pointer.Int64(0), // Let's kill the pod inmediatly
				Containers: []corev1.Container{
					{
						Name:            serverName,
						Image:           serverImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: pointer.Bool(false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							RunAsNonRoot:             pointer.Bool(true),
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
		Name:      serverName,
		Namespace: benchmarkNs,
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
		Name:      clientName,
		Namespace: benchmarkNs,
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
			},
			Spec: corev1.PodSpec{
				Affinity:                      workerAffinity,
				TerminationGracePeriodSeconds: pointer.Int64(0),
				HostNetwork:                   true, // Enable hostNetwork in client pods
				Containers: []corev1.Container{
					{
						Command:         []string{"sleep", "inf"},
						Name:            clientName,
						Image:           clientImage,
						ImagePullPolicy: corev1.PullAlways,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: pointer.Bool(false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							RunAsNonRoot:             pointer.Bool(true),
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
		Name:      clientName,
		Namespace: benchmarkNs,
	},
	Subjects: []rbac.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "default",
			Namespace: benchmarkNs,
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
			Namespace: benchmarkNs,
			Name:      fmt.Sprintf("%s-edge", serverName),
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
			Namespace: benchmarkNs,
			Name:      fmt.Sprintf("%s-reencrypt", serverName),
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("https")},
			To: routev1.RouteTargetReference{
				Name: service.Name,
			},
			TLS: &routev1.TLSConfig{
				Termination:              routev1.TLSTerminationReencrypt,
				DestinationCACertificate: "-----BEGIN CERTIFICATE-----\nMIIDbTCCAlWgAwIBAgIJAJR/jN0Oa+/rMA0GCSqGSIb3DQEBCwUAME0xCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMQswCQYDVQQHDAJOWTEcMBoGA1UE\nCgwTRGVmYXVsdCBDb21wYW55IEx0ZDAeFw0xNzAxMjQwODExMDJaFw0yNzAxMjIw\nODExMDJaME0xCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMQswCQYD\nVQQHDAJOWTEcMBoGA1UECgwTRGVmYXVsdCBDb21wYW55IEx0ZDCCASIwDQYJKoZI\nhvcNAQEBBQADggEPADCCAQoCggEBAMItGS9sSafyqBuOcQcQ5j7OQ0EwF9qOckhl\nfT8VzUbcOy8/L/w654MpLEa4O4Fiek3keE7SDWGVtGZWDvT9y1QUxPhkDWq1Y3rr\nyMelv1xRIyPVD7EEicga50flKe8CKd1U3D6iDQzq0uxZZ6I/VArXW/BZ4LfPauzN\n9EpCYyKq0fY7WRFIGouO9Wu800nxcHptzhLAgSpO97aaZ+V+jeM7n7fchRSNrpIR\nzPBl/lIBgCPJgkax0tcm4EIKIwlG+jXWc5mvV8sbT8rAv32HVuaP6NafyWXXP3H1\noBf2CQCcwuM0sM9ZeZ5JEDF/7x3eNtqSt1X9HjzVpQjiVBXY+E0CAwEAAaNQME4w\nHQYDVR0OBBYEFOXxMHAA1qaKWlP+gx8tKO2rQ81WMB8GA1UdIwQYMBaAFOXxMHAA\n1qaKWlP+gx8tKO2rQ81WMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEB\nAJAri7Pd0eSY/rvIIvAvjhDPvKt6gI5hJEUp+M3nWTWA/IhQFYutb9kkZGhbBeLj\nqneJa6XYKaCcUx6/N6Vvr3AFqVsbbubbejRpdpXldJC33QkwaWtTumudejxSon24\nW/ANN/3ILNJVMouspLRGkFfOYp3lq0oKAlNZ5G3YKsG0znAfqhAVtqCTG9RU24Or\nxzkEaCw8IY5N4wbjCS9FPLm7zpzdg/M3A/f/vrIoGdns62hzjzcp0QVTiWku74M8\nv7/XlUYYvXOvPQCCHgVjnAZlnjcxMTBbwtdwfxjAmdNTmFFpASnf0s3b287zQwVd\nIeSydalVtLm7rBRZ59/2DYo=\n-----END CERTIFICATE-----",
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: benchmarkNs,
			Name:      fmt.Sprintf("%s-passthrough", serverName),
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
			Namespace: benchmarkNs,
			Name:      fmt.Sprintf("%s-http", serverName),
		},
		Spec: routev1.RouteSpec{
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("http")},
			To: routev1.RouteTargetReference{
				Name: service.Name,
			},
		},
	},
}
