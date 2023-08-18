KIND_VERSION=0.18.0
KUBERNETES_VERSION=1.26.3
KUSTOMIZE_VERSION = 5.0.1
CILIUM_VERSION = 1.14.1
CILIUM_CLI_VERSION = 0.15.5
BINDIR := $(abspath $(PWD)/../bin)

KIND := $(BINDIR)/kind
KUBECTL := $(BINDIR)/kubectl
CILIUM := $(BINDIR)/cilium
KUSTOMIZE := $(BINDIR)/kustomize
export KUBECTL

KIND_CONFIG = kind-config.yaml
ifeq ($(TEST_IPV6),true)
KIND_CONFIG = kind-config_v6.yaml
endif

.PHONY: start
start: $(KIND) $(KUBECTL) $(CILIUM) $(KUSTOMIZE)
	$(KIND) create cluster --image kindest/node:v$(KUBERNETES_VERSION) --name egress-gw --config $(KIND_CONFIG)

.PHONY: stop
stop: $(KIND)
	$(KIND) delete cluster --name egress-gw

.PHONY: install-cilium
install-cilium:
	CILIUM_CLI_MODE=helm $(CILIUM) install --version v$(CILIUM_VERSION) \
		--set=cni.customConf=true \
		--set=hubble.relay.enabled=true \
		--set=ipam.mode=multi-pool \
		--set=tunnel=disabled \
		--set=autoDirectNodeRoutes=true \
		--set=ipv4NativeRoutingCIDR=10.0.0.0/8 \
		--set=routingMode=native \
		--set=endpointRoutes.enabled=true \
		--set-string=extraConfig.enable-local-node-route=false \
		--set=kubeProxyReplacement=strict \
		--set=bpf.masquerade=true \
		--set=socketLB.hostNamespaceOnly=true \
		--set=ipam.operator.autoCreateCiliumPodIPPools.default.ipv4.cidrs='{10.10.0.0/16}' \
		--set=ipam.operator.autoCreateCiliumPodIPPools.default.ipv4.maskSize=24
	$(KUBECTL) -n kube-system wait --timeout=3m --for=condition=available deployment/cilium-operator
	$(KUBECTL) -n kube-system --timeout=3m rollout status daemonset cilium

.PHONY: install-egress-gw
install-egress-gw:
	$(KIND) load docker-image --name egress-gw egress-gw:dev
	$(KUSTOMIZE) build --load-restrictor=LoadRestrictionsNone . | $(KUBECTL) apply -f -
	$(KUBECTL) -n kube-system wait --timeout=3m --for=condition=available deployment/egress-gw-controller
	$(KUBECTL) -n kube-system --timeout=3m rollout status daemonset egress-gw-agent
	$(KUBECTL) taint nodes egress-gw-worker test:NoSchedule
	$(KUBECTL) taint nodes egress-gw-worker2 test:NoSchedule
	$(KUBECTL) label nodes egress-gw-worker test=egress-gw
	$(KUBECTL) label nodes egress-gw-worker2 test=egress-gw

.PHONY: test
test:
	go build -o echotest ./echo-server
	docker cp echotest egress-gw-control-plane:/usr/local/bin
	rm echotest
	go test -count 1 -v . -args -ginkgo.progress -ginkgo.v

.PHONY: logs
logs:
	rm -rf logs.tar.gz logs
	$(KIND) export logs --name=egress-gw ./logs
	tar czf logs.tar.gz logs
	rm -rf logs

$(KIND):
	mkdir -p $(dir $@)
	curl -sfL -o $@ https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-linux-amd64
	chmod a+x $@

$(KUBECTL):
	mkdir -p $(dir $@)
	curl -sfL -o $@ https://dl.k8s.io/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $@

$(CILIUM):
	mkdir -p $(dir $@)
	curl -sfL -o $(BINDIR)/cilium-linux-amd64.tar.gz https://github.com/cilium/cilium-cli/releases/download/v$(CILIUM_CLI_VERSION)/cilium-linux-amd64.tar.gz
	tar xzf $(BINDIR)/cilium-linux-amd64.tar.gz -C $(BINDIR) cilium
	chmod a+x $@

$(KUSTOMIZE):
	mkdir -p $(dir $@)
	curl -sfL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar -xz -C $(BINDIR)
	chmod a+x $@
