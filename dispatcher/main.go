package main

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/streadway/amqp"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/kubeutils"
	"github.com/Azure/adx-automation-agent/models"
	"github.com/Azure/adx-automation-agent/reportutils"
	"github.com/Azure/adx-automation-agent/schedule"
)

var (
	taskBroker    = schedule.CreateInClusterTaskBroker()
	namespace     = common.GetCurrentNamespace("a01-prod")
	droidMetadata = models.ReadDroidMetadata(common.PathMetadataYml)
	clientset     = kubeutils.TryCreateKubeClientset()
	version       = "Unknown"
	sourceCommit  = "Unknown"
)

func main() {
	common.LogInfo(fmt.Sprintf("A01 Droid Dispatcher.\nVersion: %s.\nCommit: %s.\n", version, sourceCommit))
	common.LogInfo(fmt.Sprintf("Pod name: %s", os.Getenv(common.EnvPodName)))

	var pRunID *int
	pRunID = flag.Int("run", -1, "The run ID")
	flag.Parse()

	if *pRunID == -1 {
		log.Fatal("Missing runID")
	}

	// query the run and then update the product name in the details
	run, err := models.QueryRun(*pRunID)
	common.ExitOnError(err, "fail to query the run")

	run.Details[common.KeyProduct] = droidMetadata.Product
	run, err = run.Patch()
	common.ExitOnError(err, "fail to update the run")

	printRunInfo(run)

	// generate a job name. the name will be used through out the remaining
	// session to identify the group of operations and resources
	jobName := fmt.Sprintf("%s-%d-%s", droidMetadata.Product, run.ID, getRandomString())

	// publish tasks to the task broker which will establish a worker queue
	err = publishTasks(run, jobName, queryTests(run))
	common.ExitOnError(err, "Fail to publish tasks to the task broker.")
	defer taskBroker.Close()

	// creates a kubernete job to manage test droid
	jobDef, err := createTaskJob(run, jobName)
	if err != nil {
		log.Fatal(err.Error())
	}

	// ignore this error for now. This API's latest version seems to sending
	// inaccurate error
	job, _ := clientset.BatchV1().Jobs(namespace).Create(jobDef)
	job, err = clientset.BatchV1().Jobs(namespace).Get(jobDef.Name, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}

	// begin monitoring the job status till the end
	monitor(run, job)
}

func monitor(run *models.Run, job *batchv1.Job) {
	common.LogInfo("Begin monitoring task execution ...")

	ch, err := taskBroker.GetChannel()
	common.PanicOnError(err, "Fail to establish channel to the task broker during monitoring.")

	podListOpt := metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", job.Name)}

	for {
		time.Sleep(time.Second * 10)

		queue, err := ch.QueueInspect(job.ObjectMeta.Name)
		if err != nil {
			common.LogWarning(fmt.Errorf("Fail to insepct the queue %s: %s", job.ObjectMeta.Name, err).Error())
			continue
		}
		common.LogInfo(fmt.Sprintf("Messages %d => Consumers %d.", queue.Messages, queue.Consumers))

		if queue.Messages != 0 {
			// there are tasks to be run
			continue
		}

		// the number of the message in the queue is zero. make sure all the
		// pods in this job have finished
		podList, err := clientset.CoreV1().Pods(namespace).List(podListOpt)
		if err != nil {
			common.LogWarning(fmt.Errorf("Fail to list pod of %s: %s", job.Name, err).Error())
			continue
		}

		runningPods := 0
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}

		if runningPods != 0 {
			common.LogInfo(fmt.Sprintf("%d pod are still running.", runningPods))
			continue
		}

		// zero task in the queue and all pod stop.
		break
	}

	reportutils.Report(run, droidMetadata.Receivers)
}

func queryTests(run *models.Run) []models.TaskSetting {
	common.LogInfo(fmt.Sprintf("Expecting script %s.", common.PathScriptGetIndex))
	content, err := exec.Command(common.PathScriptGetIndex).Output()
	if err != nil {
		panic(err.Error())
	}

	var input []models.TaskSetting
	err = json.Unmarshal(content, &input)
	if err != nil {
		panic(err.Error())
	}

	if query, ok := run.Settings[common.KeyTestQuery]; ok {
		common.LogInfo(fmt.Sprintf("Query string is '%s'", query))
		result := make([]models.TaskSetting, 0, len(input))
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

func printRunInfo(run *models.Run) {
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

func publishTasks(run *models.Run, jobName string, settings []models.TaskSetting) (err error) {
	common.LogInfo(fmt.Sprintf("To schedule %d tests.", len(settings)))

	_, ch, err := taskBroker.QueueDeclare(jobName)
	if err != nil {
		// TODO: update run's status in DB to failed
		common.ExitOnError(err, "Fail to declare queue in task broker.")
	}

	common.LogInfo(fmt.Sprintf("Declared queue %s. Begin publishing tasks ...", jobName))
	for _, setting := range settings {
		body, err := json.Marshal(setting)
		if err != nil {
			common.LogWarning(fmt.Sprintf("Fail to marshal task %s setting in JSON. Error %s. The task is skipped.", setting, err.Error()))
			continue
		}

		err = ch.Publish(
			"",      // default exchange
			jobName, // routing key
			false,   // mandatory
			false,   // immediate
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         body,
			})

		if err != nil {
			common.LogWarning(fmt.Sprintf("Fail to publish task %s. Error %s. The task is skipped.", setting, err.Error()))
		}
	}

	common.LogInfo("Finish publish tasks")

	return nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////
// Kubernete JOB

func createTaskJob(run *models.Run, jobName string) (job *batchv1.Job, err error) {
	client, err := kubeutils.CreateKubeClientset()
	if err != nil {
		return nil, err
	}

	parallelism := int32(run.Settings[common.KeyInitParallelism].(float64))
	var backoff int32 = 5

	definition := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &parallelism,
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: jobName,
				},
				Spec: corev1.PodSpec{
					Containers:       getContainerSpecs(run, jobName),
					ImagePullSecrets: getImagePullSource(run),
					Volumes:          getVolumes(run),
					RestartPolicy:    corev1.RestartPolicyNever,
				},
			},
		},
	}

	return client.BatchV1().Jobs(namespace).Create(&definition)
}

func getLabels(run *models.Run) map[string]string {
	labels := make(map[string]string)
	labels["run_id"] = strconv.Itoa(run.ID)
	labels["run_live"] = run.Settings[common.KeyLiveMode].(string)

	return labels
}

func getVolumes(run *models.Run) (volumes []corev1.Volume) {
	volumes = []corev1.Volume{
		corev1.Volume{
			Name: common.StorageVolumeNameTools,
			VolumeSource: corev1.VolumeSource{
				AzureFile: &corev1.AzureFileVolumeSource{
					SecretName: common.SecretNameAgents,
					ShareName:  fmt.Sprintf("linux-%s", run.Settings[common.KeyAgentVersion]),
				},
			},
		},
	}

	if !droidMetadata.Storage {
		return
	}

	volumes = append(volumes,
		corev1.Volume{
			Name: common.StorageVolumeNameArtifacts,
			VolumeSource: corev1.VolumeSource{
				AzureFile: &corev1.AzureFileVolumeSource{
					SecretName: run.GetSecretName(droidMetadata),
					ShareName:  run.Settings[common.KeyStorageShare].(string),
				},
			},
		})

	return
}

func getImagePullSource(run *models.Run) []corev1.LocalObjectReference {
	return []corev1.LocalObjectReference{corev1.LocalObjectReference{Name: run.Settings[common.KeyImagePullSecret].(string)}}
}

func getContainerSpecs(run *models.Run, jobName string) (containers []corev1.Container) {
	c := corev1.Container{
		Name:    "main",
		Image:   run.Settings[common.KeyImageName].(string),
		Env:     getEnvironmentVariableDef(run, jobName),
		Command: []string{common.PathMountTools + "/a01droid"},
	}

	volumeMounts := []corev1.VolumeMount{
		corev1.VolumeMount{
			MountPath: common.PathMountTools,
			Name:      common.StorageVolumeNameTools}}

	if droidMetadata.Storage {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: common.PathMountArtifacts,
			Name:      common.StorageVolumeNameArtifacts})
	}

	c.VolumeMounts = volumeMounts

	return []corev1.Container{c}
}

func getEnvironmentVariableDef(run *models.Run, jobName string) []corev1.EnvVar {
	result := []corev1.EnvVar{
		corev1.EnvVar{
			Name:      common.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		corev1.EnvVar{
			Name:      common.EnvNodeName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
		corev1.EnvVar{Name: common.EnvJobName, Value: jobName},
		corev1.EnvVar{
			Name: common.EnvKeyInternalCommunicationKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "store-secrets"},
					Key:                  "comkey"}}},
	}

	for _, def := range droidMetadata.Environments {
		var envVar *corev1.EnvVar
		if def.Type == "secret" {
			envVar = &corev1.EnvVar{
				Name: def.Name,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: run.GetSecretName(droidMetadata)},
						Key:                  def.Value,
					},
				},
			}
		} else if def.Type == "argument-switch-live" {
			if run.Settings[common.KeyLiveMode] == "True" {
				envVar = &corev1.EnvVar{Name: def.Name, Value: def.Value}
			}
		} else if def.Type == "argument-value-mode" {
			if v, ok := run.Settings[common.KeyTestModel]; ok {
				envVar = &corev1.EnvVar{Name: def.Name, Value: v.(string)}
			}
		}

		if envVar != nil {
			result = append(result, *envVar)
		}
	}

	return result
}

func getRandomString() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return strings.TrimRight(strings.ToLower(base32.StdEncoding.EncodeToString(bytes)), "=")
}
