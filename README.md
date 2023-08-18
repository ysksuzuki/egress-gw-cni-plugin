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
$ make test

$ kubectl -n internet get po -o wide
NAME                      READY   STATUS    RESTARTS   AGE   IP            NODE                      NOMINATED NODE   READINESS GATES
egress-6684b6fb7f-f4w4d   1/1     Running   0          58s   10.20.0.214   egress-gw-control-plane   <none>           <none>
egress-6684b6fb7f-pk8jm   1/1     Running   0          58s   10.20.0.141   egress-gw-control-plane   <none>           <none>

$ kubectl get po -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP           NODE               NOMINATED NODE   READINESS GATES
nat-client   1/1     Running   0          61s   10.10.0.12   egress-gw-worker   <none>           <none>

# nat-client -> egress-6684b6fb7f-f4w4d (SNAT) -> echo-server
$ kubectl exec nat-client -- curl -sf http://9.9.9.9/source
source: 10.20.0.214:50416
```
