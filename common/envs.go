package common

const (
	// EnvKeyStoreName stores store service's host name and path prefix
	EnvKeyStoreName = "A01_STORE_NAME"

	// EnvKeyInternalCommunicationKey stores the internal communication authorization header value
	EnvKeyInternalCommunicationKey = "A01_INTERNAL_COMKEY"

	// EnvKeyRunID stores the current run ID
	EnvKeyRunID = "A01_DROID_RUN_ID"

	// EnvPodName stores the current pod name
	EnvPodName = "ENV_POD_NAME"

	// EnvNodeName stores the current node name
	EnvNodeName = "ENV_NODE_NAME"

	// EnvJobName stores the parent job name if a pod is created in a job
	EnvJobName = "ENV_JOB_NAME"
)
