package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

const (
	envKeyStoreName                = "A01_STORE_NAME"
	envKeyInternalCommunicationKey = "A01_INTERNAL_COMKEY"
	envKeyRunID                    = "A01_DROID_RUN_ID"
	pathMountStorage               = "/mnt/storage"
	scriptPreparePod               = "/app/prepare_pod"
	scriptAfterTest                = "/app/after_test"
)

var (
	httpClient = &http.Client{CheckRedirect: nil}
	runID      = os.Getenv(envKeyRunID)
	endpoint   = "http://" + os.Getenv(envKeyStoreName)
)

type a01TaskSetting struct {
	Version     string            `json:"ver,omitempty"`
	Execution   map[string]string `json:"execution,omitempty"`
	Classifier  map[string]string `json:"classifier,omitempty"`
	Miscellanea map[string]string `json:"msic,omitempty"`
}

type a01Task struct {
	Annotation    string                 `json:"annotation,omitempty"`
	Duration      int                    `json:"duration,omitempty"`
	ID            int                    `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Result        string                 `json:"result,omitempty"`
	ResultDetails map[string]interface{} `json:"result_details,omitempty"`
	RunID         int                    `json:"run_id,omitempty"`
	Settings      a01TaskSetting         `json:"settings,omitempty"`
	Status        string                 `json:"status,omitempty"`
}

func ckEndpoint() {
	resp, err := http.Get(endpoint + "/healthy")
	if err != nil {
		log.Fatalf("Fail to get response from the endpoint %s. Error %s.\n", endpoint, err)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("The endpoint is not healthy. Status code: %d.\n", resp.StatusCode)
	}
}

func ckEnvironment() {
	required := []string{envKeyInternalCommunicationKey, envKeyRunID, envKeyStoreName}

	for _, r := range required {
		_, exists := os.LookupEnv(r)
		if !exists {
			log.Fatalf("Missing environment variable %s.\n", r)
		}
	}
}

func preparePod() {
	_, statErr := os.Stat(scriptPreparePod)
	if statErr != nil && os.IsNotExist(statErr) {
		log.Printf("Executable %s doesn't exist. Skip preparing the pod.\n", scriptPreparePod)
		return
	}

	output, err := exec.Command(scriptPreparePod).CombinedOutput()
	if err != nil {
		log.Fatalf("Fail to prepare the pod: %s.\n%s\n", err, string(output))
	}
	log.Printf("Preparing Pod: \n%s\n", string(output))
}

func createNewRequest(method string, path string, jsonBody interface{}) (result *http.Request, err error) {
	templateError := "failed to create request"
	auth := os.Getenv(envKeyInternalCommunicationKey)

	var body io.Reader
	if jsonBody != nil {
		content, err := json.Marshal(jsonBody)
		if err != nil {
			return result, fmt.Errorf("%s Failed to marshal JSON: %s", templateError, err)
		}
		body = bytes.NewBuffer(content)
	}

	result, err = http.NewRequest(method, fmt.Sprintf("%s/%s", endpoint, path), body)
	if err != nil {
		return result, fmt.Errorf("%s. Error: %s", templateError, err)
	}
	result.Header.Set("Authorization", auth)
	if body != nil {
		result.Header.Set("Content-Type", "application/json")
	}

	return result, nil
}

// checkoutTask finds a new task to run and updates in which pod it will run (this pod!)
func checkoutTask(runID string) (id int, err error) {
	templateError := fmt.Sprintf("failed /ruin/%s/checkout", runID)
	request, err := createNewRequest(http.MethodPost, fmt.Sprintf("run/%s/checkout", runID), nil)
	if err != nil {
		return id, fmt.Errorf("%s. %s", templateError, err)
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return id, fmt.Errorf("%s. Request failed with error: %s", templateError, err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNoContent {
			log.Println("No more tasks. This droid's work is done.")
			os.Exit(0)
		} else {
			return id, fmt.Errorf("%s. Status code: %d", templateError, resp.StatusCode)
		}
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return id, fmt.Errorf("%s. Failed reading response body: %v", templateError, err)
	}

	var task a01Task
	err = json.Unmarshal(b, &task)
	if err != nil {
		return id, fmt.Errorf("%s. JSON unmarshaling failed: %s", templateError, err)
	}

	// update task
	if task.ResultDetails == nil {
		task.ResultDetails = make(map[string]interface{})
	}
	task.ResultDetails["agent"] = fmt.Sprintf("%s@%s", os.Getenv("ENV_POD_NAME"), os.Getenv("ENV_NODE_NAME"))

	err = patchTask(task)
	if err != nil {
		return id, fmt.Errorf("%s. Failed updating task: %s", templateError, err)
	}

	log.Printf("Checked out task %d.\n", task.ID)
	return task.ID, nil
}

func patchTask(task a01Task) error {
	templateError := fmt.Sprintf("failed PATCH run '%s' on task '%d'", runID, task.ID)
	path := fmt.Sprintf("task/%d", task.ID)

	req, err := createNewRequest(http.MethodPatch, path, task)
	if err != nil {
		return fmt.Errorf("%s. %s", templateError, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s. Error: %s", templateError, err)
	}

	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s. Status code: %d. Failed reading response body: %s", templateError, resp.StatusCode, err)
		}
		return fmt.Errorf("%s. Status code: %d. Response: %s", templateError, resp.StatusCode, b)
	}
	return nil
}

func getTask(taskID int) (task a01Task, err error) {
	path := fmt.Sprintf("task/%d", taskID)
	templateError := fmt.Sprintf("failed GET %s", path)

	request, err := createNewRequest(http.MethodGet, path, nil)
	if err != nil {
		return task, fmt.Errorf("%s. %s", templateError, err)
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return task, fmt.Errorf("%s with error: %s", templateError, err)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("%s. Status code: %d. Failed reading response body: %s", templateError, resp.StatusCode, err)
	}

	if resp.StatusCode >= 300 {
		if err != nil {
			return
		}
		return task, fmt.Errorf("%s. Status code: %d. Response: %s", templateError, resp.StatusCode, b)
	}

	err = json.Unmarshal(b, &task)
	if err != nil {
		return task, fmt.Errorf("%s. JSON decoding failed: %s", templateError, err)
	}

	return
}

func saveTaskLog(runID string, taskID int, output []byte) {
	stat, err := os.Stat(pathMountStorage)

	if err == nil && stat.IsDir() {
		runLogFolder := path.Join(pathMountStorage, runID)
		os.Mkdir(runLogFolder, os.ModeDir)

		taskLogFile := path.Join(runLogFolder, fmt.Sprintf("task_%d.log", taskID))
		err = ioutil.WriteFile(taskLogFile, output, 0644)
		if err == nil {
			return
		}
	}

	// the mount directory doesn't exist, output the log to stdout and let the pod logs handle it.
	log.Println("Storage volume is not mount for logging. Print the task output to the stdout instead.")
	log.Println("\n" + string(output))
}

func afterTask(taskID int) {
	templateError := "after task error"
	_, err := os.Stat(scriptAfterTest)
	if err != nil && os.IsNotExist(err) {
		log.Printf("%s. Executable %s doesn't exist. Skip after task action.\n", templateError, scriptAfterTest)
		return
	}

	task, err := getTask(taskID)
	if err != nil {
		log.Printf("%s. Failed to get task: %s\n", templateError, err)
		return
	}
	taskInBytes, err := json.Marshal(task)
	if err != nil {
		log.Printf("%s. Fail to encode task to JSON. Error: %s.\n", taskInBytes, err)
		return
	}

	output, err := exec.Command(scriptAfterTest, pathMountStorage, string(taskInBytes)).CombinedOutput()
	if err != nil {
		log.Printf("Fail to execute after task action. Error: %s.\n", err)
	}
	log.Println(string(output))
}

func runTask(taskID int) error {
	templateError := "failed to run task"
	task, err := getTask(taskID)
	if err != nil {
		return fmt.Errorf("%s. Failed to get task: %s", templateError, err)
	}

	execution := strings.Fields(task.Settings.Execution["command"])

	var cmd *exec.Cmd
	if len(execution) < 2 {
		cmd = exec.Command(execution[0])
	} else {
		cmd = exec.Command(execution[0], execution[1:]...)
	}

	begin := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Now().Sub(begin) / time.Millisecond

	if err == nil {
		task.Result = "Passed"
	} else {
		task.Result = "Failed"
	}
	task.Status = "completed"

	if task.ResultDetails == nil {
		task.ResultDetails = make(map[string]interface{})
	}
	task.ResultDetails["duration"] = int(duration)
	err = patchTask(task)
	if err != nil {
		return fmt.Errorf("%s. Failed to update task: %s", templateError, err)
	}

	saveTaskLog(runID, taskID, output)

	log.Printf("[%s] Task %s", task.Result, task.Name)
	return nil
}

func main() {
	ckEnvironment()
	ckEndpoint()
	preparePod()

	for {
		taskID, err := checkoutTask(runID)
		if err != nil {
			log.Fatalf(err.Error())
		}

		err = runTask(taskID)
		if err != nil {
			log.Fatalf(err.Error())
		}

		afterTask(taskID)
	}
}
