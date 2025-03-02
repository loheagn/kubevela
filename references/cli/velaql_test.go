/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/oam-dev/kubevela/apis/core.oam.dev/common"
	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
	"github.com/oam-dev/kubevela/apis/types"
	helmapi "github.com/oam-dev/kubevela/pkg/appfile/helm/flux2apis"
	"github.com/oam-dev/kubevela/pkg/oam"
	common2 "github.com/oam-dev/kubevela/pkg/utils/common"
)

var _ = Describe("Test velaQL", func() {
	var appName = "test-velaql"
	var namespace = "default"
	It("Test GetServiceEndpoints", func() {
		arg := common2.Args{}
		arg.SetConfig(cfg)
		arg.SetClient(k8sClient)

		// prepare
		testApp := &v1beta1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: namespace,
			},
			Spec: v1beta1.ApplicationSpec{
				Components: []common.ApplicationComponent{
					{
						Name: "endpoints-test",
						Type: "webservice",
					},
				},
			},
			Status: common.AppStatus{
				AppliedResources: []common.ClusterObjectReference{
					{
						Cluster: "",
						ObjectReference: corev1.ObjectReference{
							Kind:       "Ingress",
							Namespace:  "default",
							Name:       "ingress-http",
							APIVersion: "networking.k8s.io/v1beta1",
						},
					},
					{
						Cluster: "",
						ObjectReference: corev1.ObjectReference{
							Kind:       "Ingress",
							Namespace:  "default",
							Name:       "ingress-https",
							APIVersion: "networking.k8s.io/v1",
						},
					},
					{
						Cluster: "",
						ObjectReference: corev1.ObjectReference{
							Kind:       "Ingress",
							Namespace:  "default",
							Name:       "ingress-paths",
							APIVersion: "networking.k8s.io/v1",
						},
					},
					{
						Cluster: "",
						ObjectReference: corev1.ObjectReference{
							Kind:      "Service",
							Namespace: "default",
							Name:      "nodeport",
						},
					},
					{
						Cluster: "",
						ObjectReference: corev1.ObjectReference{
							Kind:      "Service",
							Namespace: "default",
							Name:      "loadbalancer",
						},
					},
					{
						Cluster: "",
						ObjectReference: corev1.ObjectReference{
							Kind:      helmapi.HelmReleaseGVK.Kind,
							Namespace: "default",
							Name:      "helmRelease",
						},
					},
				},
			},
		}
		err := k8sClient.Create(context.TODO(), testApp)
		Expect(err).Should(BeNil())

		var mr []v1beta1.ManagedResource
		for i := range testApp.Status.AppliedResources {
			mr = append(mr, v1beta1.ManagedResource{
				ClusterObjectReference: testApp.Status.AppliedResources[i],
			})
		}
		rt := &v1beta1.ResourceTracker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appName,
				Namespace: namespace,
				Labels: map[string]string{
					oam.LabelAppName:      testApp.Name,
					oam.LabelAppNamespace: testApp.Namespace,
				},
			},
			Spec: v1beta1.ResourceTrackerSpec{
				Type:             v1beta1.ResourceTrackerTypeRoot,
				ManagedResources: mr,
			},
		}
		err = k8sClient.Create(context.TODO(), rt)
		Expect(err).Should(BeNil())

		testServicelist := []map[string]interface{}{
			{
				"name": "clusterip",
				"ports": []corev1.ServicePort{
					{Port: 80, TargetPort: intstr.FromInt(80), Name: "80port"},
					{Port: 81, TargetPort: intstr.FromInt(81), Name: "81port"},
				},
				"type": corev1.ServiceTypeClusterIP,
			},
			{
				"name": "nodeport",
				"ports": []corev1.ServicePort{
					{Port: 80, TargetPort: intstr.FromInt(80), NodePort: 30229},
				},
				"type": corev1.ServiceTypeNodePort,
			},
			{
				"name": "loadbalancer",
				"ports": []corev1.ServicePort{
					{Port: 80, TargetPort: intstr.FromInt(80), Name: "80port"},
					{Port: 81, TargetPort: intstr.FromInt(81), Name: "81port"},
				},
				"type": corev1.ServiceTypeLoadBalancer,
				"status": corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "10.10.10.10",
							},
							{
								Hostname: "text.example.com",
							},
						},
					},
				},
			},
			{
				"name": "helm1",
				"ports": []corev1.ServicePort{
					{Port: 80, NodePort: 30002, TargetPort: intstr.FromInt(80)},
				},
				"type": corev1.ServiceTypeNodePort,
				"labels": map[string]string{
					"helm.toolkit.fluxcd.io/name":      "helmRelease",
					"helm.toolkit.fluxcd.io/namespace": "default",
				},
			},
		}
		for _, s := range testServicelist {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      s["name"].(string),
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: s["ports"].([]corev1.ServicePort),
					Type:  s["type"].(corev1.ServiceType),
				},
			}

			if s["labels"] != nil {
				service.Labels = s["labels"].(map[string]string)
			}
			err := k8sClient.Create(context.TODO(), service)
			Expect(err).Should(BeNil())
			if s["status"] != nil {
				service.Status = s["status"].(corev1.ServiceStatus)
				err := k8sClient.Status().Update(context.TODO(), service)
				Expect(err).Should(BeNil())
			}
		}
		var prefixbeta = networkv1beta1.PathTypePrefix
		testIngress := []client.Object{
			&networkv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-http",
					Namespace: "default",
				},
				Spec: networkv1beta1.IngressSpec{
					Rules: []networkv1beta1.IngressRule{
						{
							Host: "ingress.domain",
							IngressRuleValue: networkv1beta1.IngressRuleValue{
								HTTP: &networkv1beta1.HTTPIngressRuleValue{
									Paths: []networkv1beta1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkv1beta1.IngressBackend{
												ServiceName: "clusterip",
												ServicePort: intstr.FromInt(80),
											},
											PathType: &prefixbeta,
										},
									},
								},
							},
						},
					},
				},
			},
			&networkv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-https",
					Namespace: "default",
				},
				Spec: networkv1beta1.IngressSpec{
					TLS: []networkv1beta1.IngressTLS{
						{
							SecretName: "https-secret",
						},
					},
					Rules: []networkv1beta1.IngressRule{
						{
							Host: "ingress.domain.https",
							IngressRuleValue: networkv1beta1.IngressRuleValue{
								HTTP: &networkv1beta1.HTTPIngressRuleValue{
									Paths: []networkv1beta1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkv1beta1.IngressBackend{
												ServiceName: "clusterip",
												ServicePort: intstr.FromInt(80),
											},
											PathType: &prefixbeta,
										},
									},
								},
							},
						},
					},
				},
			},
			&networkv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-paths",
					Namespace: "default",
				},
				Spec: networkv1beta1.IngressSpec{
					TLS: []networkv1beta1.IngressTLS{
						{
							SecretName: "https-secret",
						},
					},
					Rules: []networkv1beta1.IngressRule{
						{
							Host: "ingress.domain.path",
							IngressRuleValue: networkv1beta1.IngressRuleValue{
								HTTP: &networkv1beta1.HTTPIngressRuleValue{
									Paths: []networkv1beta1.HTTPIngressPath{
										{
											Path: "/test",
											Backend: networkv1beta1.IngressBackend{
												ServiceName: "clusterip",
												ServicePort: intstr.FromInt(80),
											},
											PathType: &prefixbeta,
										},
										{
											Path: "/test2",
											Backend: networkv1beta1.IngressBackend{
												ServiceName: "clusterip",
												ServicePort: intstr.FromInt(80),
											},
											PathType: &prefixbeta,
										},
									},
								},
							},
						},
					},
				},
			},
			&networkv1beta1.Ingress{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "networking.k8s.io/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-helm",
					Namespace: "default",
					Labels: map[string]string{
						"helm.toolkit.fluxcd.io/name":      "helmRelease",
						"helm.toolkit.fluxcd.io/namespace": "default",
					},
				},
				Spec: networkv1beta1.IngressSpec{
					Rules: []networkv1beta1.IngressRule{
						{
							Host: "ingress.domain.helm",
							IngressRuleValue: networkv1beta1.IngressRuleValue{
								HTTP: &networkv1beta1.HTTPIngressRuleValue{
									Paths: []networkv1beta1.HTTPIngressPath{
										{
											Path: "/",
											Backend: networkv1beta1.IngressBackend{
												ServiceName: "clusterip",
												ServicePort: intstr.FromInt(80),
											},
											PathType: &prefixbeta,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		for _, ing := range testIngress {
			err := k8sClient.Create(context.TODO(), ing)
			Expect(err).Should(BeNil())
		}
		var node corev1.NodeList
		err = k8sClient.List(context.TODO(), &node)
		Expect(err).Should(BeNil())
		var gatewayIP string
		if len(node.Items) > 0 {
			for _, address := range node.Items[0].Status.Addresses {
				if address.Type == corev1.NodeInternalIP {
					gatewayIP = address.Address
					break
				}
			}
		}
		velaQL, err := ioutil.ReadFile("../../charts/vela-core/templates/velaql/endpoints.yaml")
		Expect(err).Should(BeNil())
		velaQLYaml := strings.Replace(string(velaQL), "{{ include \"systemDefinitionNamespace\" . }}", types.DefaultKubeVelaNS, 1)
		var cm corev1.ConfigMap
		err = yaml.Unmarshal([]byte(velaQLYaml), &cm)
		Expect(err).Should(BeNil())
		err = k8sClient.Create(context.Background(), &cm)
		Expect(err).Should(BeNil())
		endpoints, err := GetServiceEndpoints(context.TODO(), k8sClient, appName, namespace, arg, Filter{})
		Expect(err).Should(BeNil())
		urls := []string{
			"http://ingress.domain",
			"https://ingress.domain.https",
			"https://ingress.domain.path/test",
			"https://ingress.domain.path/test2",
			fmt.Sprintf("http://%s:30229", gatewayIP),
			"http://10.10.10.10",
			"http://text.example.com",
			"tcp://10.10.10.10:81",
			"tcp://text.example.com:81",
			// helmRelease
			fmt.Sprintf("http://%s:30002", gatewayIP),
			"http://ingress.domain.helm",
		}
		for i, endpoint := range endpoints {
			Expect(endpoint.String()).Should(BeEquivalentTo(urls[i]))
		}
	})
})
