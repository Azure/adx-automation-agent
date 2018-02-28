package common

// Defines the well know paths on the image
const (
	PathMountStorage     = "/mnt/storage"
	PathScriptPreparePod = "/app/prepare_pod"
	PathScriptAfterTest  = "/app/after_test"
)

// Defines the Kubernetes specifci paths
const (
	PathKubeNamespace = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)
