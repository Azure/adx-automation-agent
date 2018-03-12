package models

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// TaskSetting is the setting data model of A01Task
type TaskSetting struct {
	Version     string            `json:"ver,omitempty"`
	Execution   map[string]string `json:"execution,omitempty"`
	Classifier  map[string]string `json:"classifier,omitempty"`
	Miscellanea map[string]string `json:"misc,omitempty"`
}

// GetIdentifier returns the unique identifier of the task setting
func (setting *TaskSetting) GetIdentifier() string {
	return setting.Classifier["identifier"]
}

// GetCommand returns the string slice of the command to execute
func (setting *TaskSetting) GetCommand() []string {
	return strings.Fields(setting.Execution["command"])
}

// Execute runs the command and returns the execution results
func (setting *TaskSetting) Execute() (result string, duration int, output []byte) {
	execution := setting.GetCommand()
	var cmd *exec.Cmd
	if len(execution) < 2 {
		cmd = exec.Command(execution[0])
	} else {
		cmd = exec.Command(execution[0], execution[1:]...)
	}

	begin := time.Now()
	output, err := cmd.CombinedOutput()
	duration = int(time.Now().Sub(begin) / time.Millisecond)

	if err == nil {
		result = "Passed"
	} else {
		result = "Failed"
	}

	return
}

// CreateCompletedTask returns a uncommited Task instance represents a completed task
func (setting *TaskSetting) CreateCompletedTask(result string, duration int, podName string, runID string) *TaskResult {
	nRunID, _ := strconv.Atoi(runID)

	task := TaskResult{
		Name:          fmt.Sprintf("Test: %s", setting.GetIdentifier()),
		Duration:      duration,
		Result:        result,
		ResultDetails: map[string]interface{}{"agent": podName},
		RunID:         nRunID,
		Settings:      *setting,
		Status:        "Completed",
	}

	return &task
}

// CreateIncompletedTask returns a uncommited Task instance represents an incompleted task
func (setting *TaskSetting) CreateIncompletedTask(podName string, runID string, errorMsg string) *TaskResult {
	nRunID, _ := strconv.Atoi(runID)

	task := TaskResult{
		Name:   fmt.Sprintf("Test: %s", setting.GetIdentifier()),
		Result: "Error",
		ResultDetails: map[string]interface{}{
			"agent": podName,
			"error": errorMsg,
		},
		RunID:    nRunID,
		Settings: *setting,
		Status:   "Error",
	}

	return &task
}
