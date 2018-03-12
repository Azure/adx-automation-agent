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

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/models"
	"github.com/Azure/adx-automation-agent/schedule"
)

var (
	taskBroker   = schedule.CreateInClusterTaskBroker()
	jobName      = os.Getenv(common.EnvJobName)
	podName      = os.Getenv(common.EnvPodName)
	runID        = strings.Split(jobName, "-")[1] // the job name MUST follows the <product>-<runID>-<random ID>
	version      = "Unknown"
	sourceCommit = "Unknown"
)

func ckEndpoint() {
	url := fmt.Sprintf("http://%s/api/healthy", common.DNSNameTaskStore)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Fail to get response from %s. Error %s.\n", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("The endpoint is not healthy. Status code: %d.\n", resp.StatusCode)
	}
}

func ckEnvironment() {
	required := []string{common.EnvKeyInternalCommunicationKey, common.EnvJobName}

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

func saveTaskLog(taskID int, output []byte) error {
	stat, err := os.Stat(common.PathMountArtifacts)
	if err == nil && stat.IsDir() {
		runLogFolder := path.Join(common.PathMountArtifacts, runID)
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

func afterTask(taskResult *models.TaskResult) error {
	_, err := os.Stat(common.PathScriptAfterTest)
	if err != nil && os.IsNotExist(err) {
		// Missing after task execuable is not considerred an error.
		return nil
	}

	log.Printf("Executing after task %s.", common.PathScriptAfterTest)

	taskInBytes, err := json.Marshal(taskResult)
	if err != nil {
		return fmt.Errorf("unable to encode task to JSON: %s", err.Error())
	}

	output, err := exec.Command(
		common.PathScriptAfterTest,
		common.PathMountArtifacts,
		string(taskInBytes),
	).CombinedOutput()

	if err != nil {
		return fmt.Errorf("execution failed: %s", err.Error())
	}

	common.LogInfo(fmt.Sprintf("After task executed. %s.", string(output)))
	return nil
}

func main() {
	common.LogInfo(fmt.Sprintf("A01 Droid Engine.\nVersion: %s.\nCommit: %s.\n", version, sourceCommit))
	common.LogInfo(fmt.Sprintf("Run ID: %s", runID))

	ckEnvironment()
	ckEndpoint()

	queue, ch, err := taskBroker.QueueDeclare(jobName)
	common.ExitOnError(err, "Failed to connect to the task broker.")

	preparePod()

	for {
		delivery, ok, err := ch.Get(queue.Name, false /* autoAck*/)
		common.ExitOnError(err, "Failed to get a delivery.")

		if !ok {
			common.LogInfo("No more task in the queue. Exiting successfully.")
			break
		}

		var output []byte
		var taskResult *models.TaskResult
		var setting models.TaskSetting
		err = json.Unmarshal(delivery.Body, &setting)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to unmarshel a delivery's body in JSON: %s", err.Error())
			common.LogError(errorMsg)

			taskResult = setting.CreateIncompletedTask(podName, runID, errorMsg)
		} else {
			common.LogInfo(fmt.Sprintf("Run task %s", setting.GetIdentifier()))

			result, duration, executeOutput := setting.Execute()
			taskResult = setting.CreateCompletedTask(result, duration, podName, runID)
			output = executeOutput
		}

		taskResult, err = taskResult.CommitNew()
		if err != nil {
			common.LogError(fmt.Sprintf("Failed to commit a new task: %s.", err.Error()))
		} else {
			err = saveTaskLog(taskResult.ID, output)
			if err != nil {
				common.LogError(fmt.Sprintf("Failed to save task log a new task: %s.", err.Error()))
			}

			err = afterTask(taskResult)
			if err != nil {
				common.LogError(fmt.Sprintf("Failed in after task: %s.", err.Error()))
			}
		}

		err = delivery.Ack(false)
		if err != nil {
			common.LogError(fmt.Sprintf("Failed to ack delivery: %s", err.Error()))
		} else {
			common.LogInfo("ACK")
		}
	}
}
