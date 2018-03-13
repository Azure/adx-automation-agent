package common

import (
	"io/ioutil"
)

// Defins well-known names in the A01 system
const (
	StorageVolumeNameArtifacts = "artifacts-storage"
	StorageVolumeNameTools     = "tools-storage"
	DNSNameTaskStore           = "store-internal-svc"
	DNSNameEmailService        = "email-internal-svc"
	DNSNameTaskBroker          = "taskbroker-internal-svc"
	SecretNameAgents           = "agent-secrets"
)

const (
	// RunStatusInitialized is set when a run is just created
	RunStatusInitialized = "Initialized"

	// RunStatusPublished is set when tasks are added to the task broker queue
	RunStatusPublished = "Published"

	// RunStatusRunning is set when test job is created and start running
	RunStatusRunning = "Running"

	// RunStatusCompleted is set when all tasks are accomplished
	RunStatusCompleted = "Completed"
)

// GetCurrentNamespace returns the namespace this Pod belongs to. If it fails
// to resolve the name, it uses the fallback name.
func GetCurrentNamespace(fallback string) string {
	if content, err := ioutil.ReadFile(PathKubeNamespace); err == nil {
		return string(content)
	}

	return fallback
}
