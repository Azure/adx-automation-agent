package models

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/httputils"
)

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

// Patch submit a patch
func (run *Run) Patch() (*Run, error) {
	body, err := json.Marshal(run)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal in JSON: %s", err.Error())
	}

	req, err := httputils.CreateRequest(http.MethodPatch, fmt.Sprintf("run/%d", run.ID), body)
	if err != nil {
		return nil, err
	}

	respContent, err := httputils.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var updated Run
	err = json.Unmarshal(respContent, &updated)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal JSON: %s", err.Error())
	}

	return &updated, nil
}

// QueryRun returns the run of the runID
func QueryRun(runID int) (*Run, error) {
	req, err := httputils.CreateRequest(http.MethodGet, fmt.Sprintf("run/%d", runID), nil)
	if err != nil {
		return nil, err
	}

	respContent, err := httputils.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var run Run
	err = json.Unmarshal(respContent, &run)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal JSON: %s", err.Error())
	}

	return &run, nil
}
