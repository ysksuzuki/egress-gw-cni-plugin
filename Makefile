# Makefile for egress-gw-cni-plugin

CONTROLLER_RUNTIME_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' go.mod)
CONTROLLER_TOOLS_VERSION=0.11.4
PROTOC_VERSION=22.3
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_GO_GRPC_VERSON=1.3.0
PROTOC_GEN_DOC_VERSION=1.5.1

## DON'T EDIT BELOW THIS LINE
SUDO=sudo
CONTROLLER_GEN := $(shell pwd)/bin/controller-gen
SETUP_ENVTEST := $(shell pwd)/bin/setup-envtest
CRD_OPTIONS = "crd:crdVersions=v1"
PROTOC_OUTPUTS = pkg/cnirpc/cni.pb.go pkg/cnirpc/cni_grpc.pb.go docs/cni-grpc.md
PROTOC := PATH=$(PWD)/bin:'$(PATH)' $(PWD)/bin/protoc -I=$(PWD)/include:.
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
PODNSLIST = pod1 pod2 pod3
NATNSLIST = nat-client nat-router nat-egress nat-target
OTHERNSLIST = test-egress-dual test-egress-v4 test-egress-v6 \
	test-client-dual test-client-v4 test-client-v6 test-client-custom \
	test-fou-dual test-fou-v4 test-fou-v6

# Set the shell used to bash for better error handling.
SHELL = /bin/bash
.SHELLFLAGS = -e -o pipefail -c

# Run tests, and set up envtest if not done already.
.PHONY: test
test: simple-test setup-envtest
	source <($(SETUP_ENVTEST) use -p env); \
		go test -race -v -count 1 ./...

.PHONY: simple-test
simple-test: test-tools
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	staticcheck ./...
	go install ./...
	go vet ./...

.PHONY: test-founat
test-founat:
	go test -c ./pkg/founat
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns delete $$i 2>/dev/null || true; done
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns add $$i; done
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns exec $$i ip link set lo up; done
	$(SUDO) ./founat.test -test.v
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns delete $$i; done
	rm -f founat.test

.PHONY: check-generate
check-generate:
	-rm $(ROLES) $(PROTOC_OUTPUTS)
	$(MAKE) generate
	$(MAKE) manifests
	go mod tidy
	git diff --exit-code --name-only

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) $(CRD_OPTIONS) webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate code
.PHONY: generate
generate: $(CONTROLLER_GEN)
	$(MAKE) $(PROTOC_OUTPUTS)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

$(CONTROLLER_GEN):
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION))

pkg/cnirpc/cni.pb.go: pkg/cnirpc/cni.proto
	$(PROTOC) --go_out=module=github.com/ysksuzuki/egress-gw-cni-plugin:. $<

pkg/cnirpc/cni_grpc.pb.go: pkg/cnirpc/cni.proto
	$(PROTOC) --go-grpc_out=module=github.com/ysksuzuki/egress-gw-cni-plugin:. $<

docs/cni-grpc.md: pkg/cnirpc/cni.proto
	$(PROTOC) --doc_out=docs --doc_opt=markdown,$@ $<

.PHONY: build
build:
	GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o work/egress-gw-cni -ldflags="-s -w" cmd/egress-gw-cni/*.go
	GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o work/egress-controller -ldflags="-s -w" cmd/egress-controller/*.go
	GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o work/egress-gw -ldflags="-s -w" cmd/egress-gw/*.go
	GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o work/egress-gw-agent -ldflags="-s -w" cmd/egress-gw-agent/*.go

work/LICENSE:
	mkdir -p work
	cp LICENSE work

.PHONY: image
image: work/LICENSE
	docker buildx build --no-cache --load -t egress-gw:dev .

.PHONY: setup
setup:
	$(SUDO) apt-get update
	$(SUDO) apt-get -y install --no-install-recommends rsync unzip

	curl -sfL -o protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-linux-x86_64.zip
	unzip -o protoc.zip bin/protoc 'include/*'
	rm -f protoc.zip
	GOBIN=$(PWD)/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(PWD)/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GRPC_VERSON)
	GOBIN=$(PWD)/bin go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v$(PROTOC_GEN_DOC_VERSION)

.PHONY: setup-envtest
setup-envtest: ## Download setup-envtest locally if necessary
	# see https://github.com/kubernetes-sigs/controller-runtime/tree/master/tools/setup-envtest
	GOBIN=$(shell pwd)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
}
endef

.PHONY: test-tools
test-tools: staticcheck

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		env GOFLAGS= go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
