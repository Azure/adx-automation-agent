package models

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Azure/adx-automation-agent/httputils"
)

// TaskResult is the data model of a task in A01 system
type TaskResult struct {
	Annotation    string                 `json:"annotation,omitempty"`
	Duration      int                    `json:"duration,omitempty"`
	ID            int                    `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Result        string                 `json:"result,omitempty"`
	ResultDetails map[string]interface{} `json:"result_details,omitempty"`
	RunID         int                    `json:"run_id,omitempty"`
	Settings      TaskSetting            `json:"settings,omitempty"`
	Status        string                 `json:"status,omitempty"`
}

// CommitNew save an uncommited Task to the database
func (task *TaskResult) CommitNew() (*TaskResult, error) {
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal JSON: %s", err.Error())
	}

	path := fmt.Sprintf("run/%d/task", task.RunID)
	req, err := httputils.CreateRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %s", err.Error())
	}

	httpClient := http.Client{CheckRedirect: nil}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %s", err.Error())
	}

	defer resp.Body.Close()
	respContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %s", err.Error())
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP Status %d: %s", resp.StatusCode, string(respContent))
	}

	var result TaskResult
	err = json.Unmarshal(respContent, &result)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal JSON: %s", err.Error())
	}

	return &result, nil
}
