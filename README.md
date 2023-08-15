# egress-gw-cni-plugin

**egress-gw-cni-plugin** is a Pod based egress gw implementation that is extracted from [coil](https://github.com/cybozu-go/coil), assuming to be setup in conjunction with Cilium CNI using the CNI chain.
This egress gw feature leverages the Cilium multi-pool IPAM.

## How to run

```bash
$ make setup
$ make certs
$ make image
$ cd e2e
$ make start
$ make install-cilium
$ make install-egress-gw
$ kubectl apply -f ./manifests/pool.yaml 
$ kubectl apply -f ./manifests/namespace.yaml 
$ kubectl apply -f ./manifests/egress.yaml 
$ kubectl apply -f ./manifests/nat-client.yaml 
```