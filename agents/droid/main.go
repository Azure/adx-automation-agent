package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/Azure/adx-automation-agent/sdk/common"
	"github.com/Azure/adx-automation-agent/sdk/kubeutils"
	"github.com/Azure/adx-automation-agent/sdk/models"
	"github.com/Azure/adx-automation-agent/sdk/schedule"
	"github.com/sirupsen/logrus"
)

var (
	taskBroker      = schedule.CreateInClusterTaskBroker()
	jobName         = os.Getenv(common.EnvJobName)
	podName         = os.Getenv(common.EnvPodName)
	runID           = strings.Split(jobName, "-")[1] // the job name MUST follows the <product>-<runID>-<random ID>
	productName     = strings.Split(jobName, "-")[0] // the job name MUST follows the <product>-<runID>-<random ID>
	logPathTemplate = ""
	version         = "Unknown"
	sourceCommit    = "Unknown"
)

func ckEnvironment() {
	required := []string{common.EnvKeyInternalCommunicationKey, common.EnvJobName}

	for _, r := range required {
		_, exists := os.LookupEnv(r)
		if !exists {
			logrus.Fatalf("Missing environment variable %s.\n", r)
		}
	}
}

func preparePod() {
	_, statErr := os.Stat(common.PathScriptPreparePod)
	if statErr != nil && os.IsNotExist(statErr) {
		logrus.Infof("Executable %s doesn't exist. Skip preparing the pod.\n", common.PathScriptPreparePod)
		return
	}

	output, err := exec.Command(common.PathScriptPreparePod).CombinedOutput()
	if err != nil {
		logrus.Fatalf("Fail to prepare the pod: %s.\n%s\n", err, string(output))
	}
	logrus.Infof("Preparing Pod: \n%s\n", string(output))
}

func afterTask(taskResult *models.TaskResult) error {
	_, err := os.Stat(common.PathScriptAfterTest)
	if err != nil && os.IsNotExist(err) {
		// Missing after task executable is not considered an error.
		return nil
	}

	logrus.Infof("Executing after task %s.", common.PathScriptAfterTest)

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

	logrus.Infof("After task executed. %s.", string(output))
	return nil
}

func main() {
	logrus.Infof("A01 Droid Engine.\nVersion: %s.\nCommit: %s.\n", version, sourceCommit)
	logrus.Infof("Run ID: %s", runID)

	ckEnvironment()

	queue, ch, err := taskBroker.QueueDeclare(jobName)
	if err != nil {
		logrus.Fatal("Failed to connect to the task broker.")
	}

	if bLogPathTemplate, exists := kubeutils.TryGetSecretInBytes(
		productName,
		common.ProductSecretKeyLogPathTemplate); exists {
		logPathTemplate = string(bLogPathTemplate)
	}

	preparePod()

	for {
		delivery, ok, err := ch.Get(queue.Name, false /* autoAck*/)
		if err != nil {
			logrus.Fatal("Failed to get a delivery: ", err)
		}

		if !ok {
			logrus.Info("No more task in the queue. Exiting successfully.")
			break
		}

		var output []byte
		var taskResult *models.TaskResult
		var setting models.TaskSetting
		err = json.Unmarshal(delivery.Body, &setting)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to unmarshel a delivery's body in JSON: %s", err.Error())
			logrus.Error(errorMsg)

			taskResult = setting.CreateUncompletedTask(podName, runID, errorMsg)
		} else {
			logrus.Infof("Run task %s", setting.GetIdentifier())

			result, duration, executeOutput := setting.Execute()
			taskResult = setting.CreateCompletedTask(result, duration, podName, runID)
			output = executeOutput
		}

		taskResult, err = taskResult.CommitNew()
		if err != nil {
			logrus.Errorf("Failed to commit a new task: %s.", err.Error())
		} else {
			taskLogPath, err := taskResult.SaveTaskLog(output)
			if err != nil {
				logrus.Error(err)
			}

			err = afterTask(taskResult)
			if err != nil {
				logrus.Errorf("Failed in after task: %s.", err.Error())
			}

			if len(logPathTemplate) > 0 {
				taskResult.ResultDetails[common.KeyTaskLogPath] = strings.Replace(
					logPathTemplate,
					"{}",
					taskLogPath,
					1)

				taskResult.ResultDetails[common.KeyTaskRecordPath] = strings.Replace(
					logPathTemplate,
					"{}",
					path.Join(strconv.Itoa(taskResult.RunID), fmt.Sprintf("recording_%d.yaml", taskResult.ID)),
					1)

				_, err := taskResult.CommitChanges()
				if err != nil {
					logrus.Error(err)
				}
			}
		}

		err = delivery.Ack(false)
		if err != nil {
			logrus.Errorf("Failed to ack delivery: %s", err.Error())
		} else {
			logrus.Info("ACK")
		}
	}
}
