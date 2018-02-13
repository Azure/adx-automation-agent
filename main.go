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
	Version     string            `json:"ver"`
	Execution   map[string]string `json:"execution"`
	Classifier  map[string]string `json:"classifier"`
	Miscellanea map[string]string `json:"msic"`
}

type a01Task struct {
	Annotation    string                 `json:"annotation"`
	Duration      int                    `json:"duration"`
	ID            int                    `json:"id"`
	Name          string                 `json:"name"`
	Result        string                 `json:"result"`
	ResultDetails map[string]interface{} `json:"result_details"`
	RunID         int                    `json:"run_id"`
	Settings      a01TaskSetting         `json:"settings"`
	Status        string                 `json:"status"`
}

type a01TaskCollections []a01Task

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

	for _, each := range required {
		_, exists := os.LookupEnv(envKeyInternalCommunicationKey)
		if !exists {
			log.Fatalf("Missing environment variable %s.\n", each)
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

func createNewRequest(method string, path string, jsonBody interface{}) *http.Request {
	auth := os.Getenv(envKeyInternalCommunicationKey)

	var body io.Reader
	if jsonBody != nil {
		content, jsonErr := json.Marshal(jsonBody)
		if jsonErr != nil {
			log.Fatalf("Fail to marshal JSON. Error: %s\n", jsonErr)
		}
		body = bytes.NewBuffer(content)
	}

	result, err := http.NewRequest(method, fmt.Sprintf("%s/%s", endpoint, path), body)
	if err != nil {
		log.Fatalf("Fail to create new request. Error: %s\n", err)
	}
	result.Header.Set("Authorization", auth)
	if body != nil {
		result.Header.Set("Content-Type", "application/json")
	}

	return result
}

func checkoutTask(runID string) int {
	request := createNewRequest("POST", fmt.Sprintf("run/%s/checkout", runID), nil)
	resp, err := httpClient.Do(request)
	if err != nil {
		log.Fatalf("Fail /run/%s/checkout. Error: %s.\n", runID, err)
	}

	if resp.StatusCode == 200 {

	} else if resp.StatusCode == 204 {
		log.Print("No more task. This droid's work is done.")
		os.Exit(0)
	} else {
		log.Fatalf("Fail /run/%s/checkout. Status code: %d.\n", runID, resp.StatusCode)
	}

	var task a01Task
	err = json.NewDecoder(resp.Body).Decode(&task)
	if err != nil {
		log.Fatalf("Fail /run/%s/checkout. JSON decoding failed: %s.\n", runID, err)
	}

	if task.ResultDetails == nil {
		task.ResultDetails = make(map[string]interface{})
	}
	task.ResultDetails["agent"] = fmt.Sprintf("%s@%s", os.Getenv("ENV_POD_NAME"), os.Getenv("ENV_NODE_NAME"))

	patchTask(task.ID, task)

	log.Printf("Check out task %d.\n", task.ID)
	return task.ID
}

func patchTask(taskID int, patch interface{}) {
	path := fmt.Sprintf("task/%d", taskID)
	request := createNewRequest("PATCH", path, patch)
	resp, err := httpClient.Do(request)
	if err != nil {
		log.Fatalf("Fail PATCH %s. Error: %s.\n", path, err)
	} else if resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Fatalf("Fail PATCH %s. Status code: %d. Response: %s.\n", path, resp.StatusCode, body)
	}
}

func getTask(taskID int) *a01Task {
	path := fmt.Sprintf("task/%d", taskID)
	request := createNewRequest("GET", path, nil)
	resp, err := httpClient.Do(request)
	if err != nil {
		log.Fatalf("Fail GET %s. Error: %s.\n", path, err)
	} else if resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Fatalf("Fail PATCH %s. Status code: %d. Response: %s.\n", path, resp.StatusCode, body)
	}

	var result a01Task
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if decodeErr != nil {
		log.Fatalf("Fail GET %s. JSON decoding failed: %s.\n", path, err)
	}

	return &result
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
	_, statErr := os.Stat(scriptAfterTest)
	if statErr != nil && os.IsNotExist(statErr) {
		log.Printf("Executable %s doesn't exist. Skip after task action.\n", scriptAfterTest)
		return
	}

	task := getTask(taskID)
	taskInBytes, jsonErr := json.Marshal(task)
	if jsonErr != nil {
		log.Printf("Fail to encode task to JSON. Error: %s.\n", jsonErr)
		return
	}

	output, err := exec.Command(scriptAfterTest, pathMountStorage, string(taskInBytes)).CombinedOutput()
	if err != nil {
		log.Printf("Fail to execute after task action. Error: %s.\n", err)
	}
	log.Println(string(output))
}

func runTask(taskID int) {
	task := getTask(taskID)
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
	patchTask(taskID, *task)
	saveTaskLog(runID, taskID, output)

	log.Printf("[%s] Task %s", task.Result, task.Name)
}

func main() {
	ckEnvironment()
	ckEndpoint()
	preparePod()

	for {
		taskID := checkoutTask(runID)
		runTask(taskID)
		afterTask(taskID)
	}
}
