
## DON'T EDIT BELOW THIS LINE
SUDO=sudo
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

.PHONY: test-founat
test-founat:
	go test -c ./pkg/founat
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns delete $$i 2>/dev/null || true; done
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns add $$i; done
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns exec $$i ip link set lo up; done
	$(SUDO) ./founat.test -test.v
	for i in $(NATNSLIST) $(OTHERNSLIST); do $(SUDO) ip netns delete $$i; done
	rm -f founat.test
