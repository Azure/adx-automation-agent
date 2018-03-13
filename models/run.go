package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/httputils"
)

// Run is the data structure of A01 run
type Run struct {
	ID       int                    `json:"id,omitempty"`
	Name     string                 `json:"name"`
	Settings map[string]interface{} `json:"settings"`
	Details  map[string]string      `json:"details"`
	Status   string                 `json:"status"`
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

// SubmitChange POST the changes in current Run instance to task store
func (run *Run) SubmitChange() (*Run, error) {
	body, err := json.Marshal(run)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal in JSON: %s", err.Error())
	}

	req, err := httputils.CreateRequest(http.MethodPost, fmt.Sprintf("run/%d", run.ID), body)
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

// QueryTests returns the list of test tasks based on the query string
func (run *Run) QueryTests() []TaskSetting {
	common.LogInfo(fmt.Sprintf("Expecting script %s.", common.PathScriptGetIndex))
	content, err := exec.Command(common.PathScriptGetIndex).Output()
	if err != nil {
		panic(err.Error())
	}

	var input []TaskSetting
	err = json.Unmarshal(content, &input)
	if err != nil {
		panic(err.Error())
	}

	if query, ok := run.Settings[common.KeyTestQuery]; ok {
		common.LogInfo(fmt.Sprintf("Query string is '%s'", query))
		result := make([]TaskSetting, 0, len(input))
		for _, test := range input {
			matched, regerr := regexp.MatchString(query.(string), test.Classifier["identifier"])
			if matched && regerr == nil {
				result = append(result, test)
			}
		}

		return result
	}

	return input
}

// PrintInfo prints the run's detailed information to stdout
func (run *Run) PrintInfo() {
	common.LogInfo(fmt.Sprintf("Find run %d: %s.", run.ID, run.Name))
	if run.Details != nil {
		common.LogInfo("  Details")
		for key, value := range run.Details {
			common.LogInfo(fmt.Sprintf("    %s = %s", key, value))
		}
	}
	if run.Settings != nil {
		common.LogInfo("  Settings")
		for key, value := range run.Settings {
			common.LogInfo(fmt.Sprintf("    %s = %s", key, value))
		}
	}
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
