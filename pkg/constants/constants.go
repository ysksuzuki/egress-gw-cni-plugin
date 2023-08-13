package constants

// AnnEgressPrefix annotation keys
const (
	AnnEgressPrefix = "egress.ysksuzuki.com/"
)

// Keys in CNI_ARGS
const (
	PodNameKey      = "K8S_POD_NAME"
	PodNamespaceKey = "K8S_POD_NAMESPACE"
	PodContainerKey = "K8S_POD_INFRA_CONTAINER_ID"
)

// Label keys
const (
	LabelAppName      = "app.kubernetes.io/name"
	LabelAppInstance  = "app.kubernetes.io/instance"
	LabelAppComponent = "app.kubernetes.io/component"
)

// RBAC resource names
const (
	// SAEgress is the name of the ServiceAccount for egress
	SAEgress = "egress"

	// CRBEgress is the name of the ClusterRoleBinding for egress
	CRBEgress = "egress"
)

// Environment variables
const (
	EnvAddresses    = "EGRESS_POD_ADDRESSES"
	EnvPodNamespace = "EGRESS_POD_NAMESPACE"
	EnvPodName      = "EGRESS_POD_NAME"
	EnvEgressName   = "EGRESS_NAME"
)
const MetricsNS = "egressgw"
