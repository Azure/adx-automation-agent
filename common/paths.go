package common

// Defines the well know paths on the image
const (
	PathMountArtifacts   = "/mnt/storage"
	PathMountTools       = "/mnt/tools"
	PathScriptPreparePod = "/app/prepare_pod"
	PathScriptAfterTest  = "/app/after_test"
	PathScriptGetIndex   = "/app/get_index"
	PathMetadataYml      = "/app/metadata.yml"
)

// Defines the Kubernetes specific paths
const (
	PathKubeNamespace = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)
