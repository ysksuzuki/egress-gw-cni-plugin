# Makefile for egress-gw-cni-plugin

PROTOC_VERSION=22.3
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_GO_GRPC_VERSON=1.3.0
PROTOC_GEN_DOC_VERSION=1.5.1

## DON'T EDIT BELOW THIS LINE
SUDO=sudo
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
test: simple-test
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
	go mod tidy
	git diff --exit-code --name-only

# Generate code
.PHONY: generate
generate: $(CONTROLLER_GEN)
	$(MAKE) $(PROTOC_OUTPUTS)

pkg/cnirpc/cni.pb.go: pkg/cnirpc/cni.proto
	$(PROTOC) --go_out=module=github.com/ysksuzuki/egress-gw-cni-plugin:. $<

pkg/cnirpc/cni_grpc.pb.go: pkg/cnirpc/cni.proto
	$(PROTOC) --go-grpc_out=module=github.com/ysksuzuki/egress-gw-cni-plugin:. $<

docs/cni-grpc.md: pkg/cnirpc/cni.proto
	$(PROTOC) --doc_out=docs --doc_opt=markdown,$@ $<

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

.PHONY: test-tools
test-tools: staticcheck

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		env GOFLAGS= go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
