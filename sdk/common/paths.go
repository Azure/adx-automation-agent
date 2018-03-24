package common

// Defines the well know paths on the image
const (
	PathMountArtifacts   = "/mnt/storage"
	PathMountSecrets     = "/mnt/secrets"
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

// Defines the path template for logs
const (
	// PathTemplateTaskLog defines the relative path of a task's log file in a file share.
	// It is <run_id>/task_<task_id>.log
	PathTemplateTaskLog = "%d/task_%d.log"
)
