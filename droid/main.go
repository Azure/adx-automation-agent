package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/httputils"
	"github.com/Azure/adx-automation-agent/models"
)

var (
	httpClient   = &http.Client{CheckRedirect: nil}
	runID        = os.Getenv(common.EnvKeyRunID)
	endpoint     = "http://" + os.Getenv(common.EnvKeyStoreName)
	version      = "Unknown"
	sourceCommit = "Unknown"
)

func ckEndpoint() {
	resp, err := http.Get(endpoint + "/healthy")
	if err != nil {
		log.Fatalf("Fail to get response from the endpoint %s. Error %s.\n", endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("The endpoint is not healthy. Status code: %d.\n", resp.StatusCode)
	}
}

func ckEnvironment() {
	required := []string{
		common.EnvKeyInternalCommunicationKey,
		common.EnvKeyRunID,
		common.EnvKeyStoreName}

	for _, r := range required {
		_, exists := os.LookupEnv(r)
		if !exists {
			log.Fatalf("Missing environment variable %s.\n", r)
		}
	}
}

func preparePod() {
	_, statErr := os.Stat(common.PathScriptPreparePod)
	if statErr != nil && os.IsNotExist(statErr) {
		log.Printf("Executable %s doesn't exist. Skip preparing the pod.\n", common.PathScriptPreparePod)
		return
	}

	output, err := exec.Command(common.PathScriptPreparePod).CombinedOutput()
	if err != nil {
		log.Fatalf("Fail to prepare the pod: %s.\n%s\n", err, string(output))
	}
	log.Printf("Preparing Pod: \n%s\n", string(output))
}

// checkoutTask finds a new task to run and updates in which pod it will run (this pod!)
func checkoutTask(runID string) (id int, err error) {
	templateError := fmt.Sprintf("Fail to checkout task from run %s.", runID) + " Reason: %s. Exception: %s."
	request, err := httputils.CreateRequest(http.MethodPost, fmt.Sprintf("run/%s/checkout", runID), nil)
	if err != nil {
		return 0, fmt.Errorf(templateError, "unable to create new request", err)
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf(templateError, "http request failed.", err)
	}

	if resp.StatusCode == http.StatusOK {
		// continue
	} else if resp.StatusCode == http.StatusNoContent {
		log.Println("No more tasks. This droid's work is done.")
		os.Exit(0)
	} else {
		reason := fmt.Sprintf("status code: %d.", resp.StatusCode) + " Body %s"
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, fmt.Errorf(templateError, fmt.Sprintf(reason, "fail to read."), err)
		}
		return 0, fmt.Errorf(templateError, fmt.Sprintf(reason, string(b)), "N/A")
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf(templateError, "unable to read response body", err)
	}

	var task models.Task
	err = json.Unmarshal(b, &task)
	if err != nil {
		return 0, fmt.Errorf(templateError, "unable to parse body in JSON", err)
	}

	// update task
	if task.ResultDetails == nil {
		task.ResultDetails = make(map[string]interface{})
	}
	task.ResultDetails["agent"] = fmt.Sprintf("%s@%s", os.Getenv("ENV_POD_NAME"), os.Getenv("ENV_NODE_NAME"))

	err = patchTask(task)
	if err != nil {
		return 0, fmt.Errorf(templateError, "unable to update the task", err)
	}

	log.Printf("Checked out task %d.\n", task.ID)
	return task.ID, nil
}

func patchTask(task models.Task) error {
	templateError := fmt.Sprintf("Fail to path task %d.", task.ID) + " Reason: %s. Exception: %s."
	path := fmt.Sprintf("task/%d", task.ID)

	// Marshal the task
	content, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf(templateError, "unable to marshal body in JSON", err)
	}

	req, err := httputils.CreateRequest(http.MethodPatch, path, content)
	if err != nil {
		return fmt.Errorf(templateError, "unable to create new request", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(templateError, "http request failed.", err)
	}

	if resp.StatusCode >= 300 {
		reason := fmt.Sprintf("status code: %d.", resp.StatusCode) + " Body %s"
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf(templateError, fmt.Sprintf(reason, "fail to read."), err)
		}
		return fmt.Errorf(templateError, fmt.Sprintf(reason, string(b)), "N/A")
	}

	return nil
}

func getTask(taskID int) (task models.Task, err error) {
	templateError := fmt.Sprintf("Fail to get task %d.", taskID) + " Reason: %s. Exception: %s."
	path := fmt.Sprintf("task/%d", taskID)

	request, err := httputils.CreateRequest(http.MethodGet, path, nil)
	if err != nil {
		return task, fmt.Errorf(templateError, "unable to create new request", err)
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return task, fmt.Errorf(templateError, "http request failed.", err)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return task, fmt.Errorf(templateError, "unable to read response body.", err)
	}

	if resp.StatusCode >= 300 {
		reason := fmt.Sprintf("status code: %d. Body %s", resp.StatusCode, string(b))
		return task, fmt.Errorf(templateError, reason, "N/A")
	}

	err = json.Unmarshal(b, &task)
	if err != nil {
		return task, fmt.Errorf(templateError, "unable to parse body in JSON", err)
	}

	return
}

func saveTaskLog(runID string, taskID int, output []byte) error {
	stat, err := os.Stat(common.PathMountStorage)
	if err == nil && stat.IsDir() {
		runLogFolder := path.Join(common.PathMountStorage, runID)
		os.Mkdir(runLogFolder, os.ModeDir)

		taskLogFile := path.Join(runLogFolder, fmt.Sprintf("task_%d.log", taskID))
		err = ioutil.WriteFile(taskLogFile, output, 0644)
		if err != nil {
			return fmt.Errorf("Fail to save task log. Reason: unable to write file. Exception: %s", err)
		}
		return nil
	}

	// the mount directory doesn't exist, output the log to stdout and let the pod logs handle it.
	log.Println("Storage volume is not mount for logging. Print the task output to the stdout instead.")
	log.Println("\n" + string(output))
	return nil
}

func afterTask(taskID int) error {
	templateError := "Fail to exectue after task action. Reason: %s. Exception: %s."
	_, err := os.Stat(common.PathScriptAfterTest)
	if err != nil && os.IsNotExist(err) {
		// Missing after task execuable is not considerred an error.
		log.Printf("Skip the after task action because the executable %s doesn't exist.", common.PathScriptAfterTest)
		return nil
	}

	task, err := getTask(taskID)
	if err != nil {
		return fmt.Errorf(templateError, "unable to get the task", err)
	}
	taskInBytes, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf(templateError, "unable to encode task to JSON", err)
	}

	output, err := exec.Command(common.PathScriptAfterTest, common.PathMountStorage, string(taskInBytes)).CombinedOutput()
	if err != nil {
		return fmt.Errorf(templateError, "task executable failure", err)
	}

	log.Printf("After task executed.\n%s\n", string(output))
	return nil
}

func runTask(taskID int) error {
	templateError := fmt.Sprintf("Fail to run task %d.", taskID) + " Reason: %s. Exception: %s."
	task, err := getTask(taskID)
	if err != nil {
		return fmt.Errorf(templateError, "unable to get task", err)
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
		return fmt.Errorf(templateError, "unable to update task", err)
	}

	err = saveTaskLog(runID, taskID, output)
	if err != nil {
		log.Println(err.Error())
	}

	log.Printf("[%s] Task %s", task.Result, task.Name)
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("A01 Droid Engine.\nVersion: %s.\nCommit: %s.\n", version, sourceCommit)
		os.Exit(0)
	}

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

		err = afterTask(taskID)
		if err != nil {
			// after task action's failure is not fatal.
			log.Println(err.Error())
		}
	}
}
