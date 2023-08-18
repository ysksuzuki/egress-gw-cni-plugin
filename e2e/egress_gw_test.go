package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var testIPv6 = os.Getenv("TEST_IPV6") == "true"

var _ = Describe("egress-gw-cni-plugin", func() {
	It("should run health probe servers", func() {
		By("checking all pods get ready")
		Eventually(func() error {
			pods := &corev1.PodList{}
			err := getResource("kube-system", "pods", "", "app.kubernetes.io/name=egress-gw", pods)
			if err != nil {
				return err
			}

			if len(pods.Items) == 0 {
				return errors.New("bug")
			}

		OUTER:
			for _, pod := range pods.Items {
				for _, cond := range pod.Status.Conditions {
					if cond.Type != corev1.PodReady {
						continue
					}
					if cond.Status == corev1.ConditionTrue {
						continue OUTER
					}
				}
				return fmt.Errorf("pod is not ready: %s", pod.Name)
			}

			return nil
		}).Should(Succeed())
	})

	It("should export metrics", func() {
		By("checking port 9384 for egress-gw-agent")
		out, err := runOnNode("egress-gw-worker", "curl", "-sf", "http://localhost:9384/metrics")
		Expect(err).ShouldNot(HaveOccurred())

		mfs, err := (&expfmt.TextParser{}).TextToMetricFamilies(bytes.NewReader(out))
		Expect(err).NotTo(HaveOccurred())
		Expect(mfs).NotTo(BeEmpty())
	})

	It("should be able to run Egress pods", func() {
		By("creating address pool")
		kubectlSafe(nil, "apply", "-f", "manifests/pool.yaml")

		By("creating internet namespace")
		kubectlSafe(nil, "apply", "-f", "manifests/namespace.yaml")

		By("defining Egress in the internet namespace")
		kubectlSafe(nil, "apply", "-f", "manifests/egress.yaml")

		By("checking pod deployments")
		Eventually(func() int {
			depl := &appsv1.Deployment{}
			err := getResource("internet", "deployments", "egress", "", depl)
			if err != nil {
				return 0
			}
			return int(depl.Status.ReadyReplicas)
		}).Should(Equal(2))
	})

	It("should be able to run NAT client pods", func() {
		By("creating a NAT client pod")
		kubectlSafe(nil, "apply", "-f", "manifests/nat-client.yaml")

		By("checking the pod status")
		Eventually(func() error {
			pod := &corev1.Pod{}
			err := getResource("default", "pods", "nat-client", "", pod)
			if err != nil {
				return err
			}
			if len(pod.Status.ContainerStatuses) == 0 {
				return errors.New("no container status")
			}
			cst := pod.Status.ContainerStatuses[0]
			if !cst.Ready {
				return errors.New("container is not ready")
			}
			return nil
		}).Should(Succeed())
	})

	It("should allow NAT traffic over foo-over-udp tunnel", func() {
		var fakeIP, fakeURL string
		if testIPv6 {
			fakeIP = "2606:4700:4700::9999"
			fakeURL = fmt.Sprintf("http://[%s]", fakeIP)
		} else {
			fakeIP = "9.9.9.9"
			fakeURL = "http://" + fakeIP
		}

		By("setting a fake global address to egress-gw-control-plane")
		_, err := runOnNode("egress-gw-control-plane", "ip", "link", "add", "dummy-fake", "type", "dummy")
		Expect(err).NotTo(HaveOccurred())
		_, err = runOnNode("egress-gw-control-plane", "ip", "link", "set", "dummy-fake", "up")
		Expect(err).NotTo(HaveOccurred())
		if testIPv6 {
			_, err = runOnNode("egress-gw-control-plane", "ip", "address", "add", fakeIP+"/128", "dev", "dummy-fake", "nodad")
		} else {
			_, err = runOnNode("egress-gw-control-plane", "ip", "address", "add", fakeIP+"/32", "dev", "dummy-fake")
		}
		Expect(err).NotTo(HaveOccurred())

		By("running HTTP server on egress-gw-control-plane")
		go runOnNode("egress-gw-control-plane", "/usr/local/bin/echotest")
		time.Sleep(100 * time.Millisecond)

		By("sending and receiving HTTP request from nat-client")
		data := make([]byte, 1<<20) // 1 MiB
		resp := kubectlSafe(data, "exec", "-i", "nat-client", "--", "curl", "-sf", "-T", "-", fakeURL)
		Expect(resp).To(HaveLen(1 << 20))

		By("running the same test 100 times")
		for i := 0; i < 100; i++ {
			time.Sleep(1 * time.Millisecond)
			resp := kubectlSafe(data, "exec", "-i", "nat-client", "--", "curl", "-sf", "-T", "-", fakeURL)
			Expect(resp).To(HaveLen(1 << 20))
		}
	})
})
