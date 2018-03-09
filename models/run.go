package models

import "github.com/Azure/adx-automation-agent/common"

// Run is the data structure of A01 run
type Run struct {
	ID       int                    `json:"id,omitempty"`
	Name     string                 `json:"name"`
	Settings map[string]interface{} `json:"settings"`
	Details  map[string]string      `json:"details"`
}

// GetSecretName returns the secret mapping to this run.
// It first tries to find the secret name in the run settings. If the run's settings do not contain the property,
// falls back to the prodcut name in the metadata.
func (run *Run) GetSecretName(metadata *DroidMetadata) string {
	if v, ok := run.Settings[common.KeySecrectName]; ok && len(v.(string)) >= 0 {
		return v.(string)
	}

	return metadata.Product
}
