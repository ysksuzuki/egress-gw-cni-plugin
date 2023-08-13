/*
Copyright 2023.

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

package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeEgress() *Egress {
	return &Egress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: EgressSpec{
			Destinations: []string{"10.2.0.0/16"},
			Replicas:     1,
		},
	}
}

var _ = Describe("Egress Webhook", func() {
	ctx := context.TODO()

	BeforeEach(func() {
		r := &Egress{}
		r.Name = "test"
		r.Namespace = "default"
		err := k8sClient.Delete(ctx, r)
		if err == nil {
			return
		}
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should create egress with the sane defaults", func() {
		r := makeEgress()
		r.Spec.Destinations = append(r.Spec.Destinations, "fd02::/120")
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Spec.Replicas).To(BeNumerically("==", 1))
		Expect(string(r.Spec.SessionAffinity)).To(Equal("ClientIP"))
	})

	It("should deny empty destinations", func() {
		r := makeEgress()
		r.Spec.Destinations = nil
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should deny bad subnets", func() {
		r := makeEgress()
		r.Spec.Destinations = []string{"127.0.0.1"}
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())

		r = makeEgress()
		r.Spec.Destinations = append(r.Spec.Destinations, "a.b.c.d/20")
		err = k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should deny invalid replicas", func() {
		r := makeEgress()
		r.Spec.Replicas = -1
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should deny invalid deployment strategy", func() {
		r := makeEgress()
		r.Spec.Strategy = &appsv1.DeploymentStrategy{
			Type: appsv1.DeploymentStrategyType("hoge"),
		}
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should allow valid deployment strategy", func() {
		r := makeEgress()
		r.Spec.Strategy = &appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		}
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny bad pod templates", func() {
		r := makeEgress()
		r.Spec.Template = &EgressPodTemplate{
			Metadata: Metadata{
				Annotations: map[string]string{"bad bad": "aaa"},
			},
		}
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())

		r = makeEgress()
		r.Spec.Template = &EgressPodTemplate{
			Metadata: Metadata{
				Labels: map[string]string{"good": "bad value", "name": "foo"},
			},
		}
		err = k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should allow valid pod templates", func() {
		r := makeEgress()
		r.Spec.Template = &EgressPodTemplate{
			Metadata: Metadata{
				Annotations: map[string]string{"good": "aaa bbb"},
				Labels:      map[string]string{"good": "good"},
			},
			Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Operator: corev1.TolerationOpExists,
					},
				},
			},
		}
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny invalid session affinity", func() {
		r := makeEgress()
		r.Spec.SessionAffinity = corev1.ServiceAffinity("hoge")
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should allow valid session affinity", func() {
		r := makeEgress()
		r.Spec.SessionAffinity = corev1.ServiceAffinityNone
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny updating destinations", func() {
		r := makeEgress()
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())

		r.Spec.Destinations = append(r.Spec.Destinations, "10.10.0.0/24")
		err = k8sClient.Update(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should deny invalid fields on update", func() {
		r := makeEgress()
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())

		r.Spec.Replicas = -1
		err = k8sClient.Update(ctx, r)
		Expect(err).To(HaveOccurred())
	})

	It("should allow updating other fields", func() {
		r := makeEgress()
		err := k8sClient.Create(ctx, r)
		Expect(err).NotTo(HaveOccurred())

		r.Spec.Replicas = 10
		r.Spec.SessionAffinity = corev1.ServiceAffinityNone
		err = k8sClient.Update(ctx, r)
		Expect(err).NotTo(HaveOccurred())
	})
})
