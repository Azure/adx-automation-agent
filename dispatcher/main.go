package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/httputils"
	"github.com/Azure/adx-automation-agent/kubeutils"
	"github.com/Azure/adx-automation-agent/models"
)

var (
	httpClient    = &http.Client{CheckRedirect: nil}
	namespace     = common.GetCurrentNamespace("a01-prod")
	droidMetadata = models.ReadDroidMetadata(common.PathMetadataYml)
	clientset     = kubeutils.TryCreateKubeClientset()
	version       = "Unknown"
	sourceCommit  = "Unknown"
)

func main() {
	info(fmt.Sprintf("A01 Droid Dispatcher.\nVersion: %s.\nCommit: %s.\n", version, sourceCommit))
	info(fmt.Sprintf("Pod name: %s", os.Getenv(common.EnvPodName)))

	var pRunID *int
	pRunID = flag.Int("run", -1, "The run ID")
	flag.Parse()

	if *pRunID == -1 {
		log.Fatal("Missing runID")
	}

	run := getRun(*pRunID)

	err := postTasks(run, queryTests(run))
	if err != nil {
		log.Fatal(err.Error())
	}

	jobDef, err := createTaskJob(run)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Ignore this error for now. This API's latest version seems to sending inaccurate
	// error
	job, _ := clientset.BatchV1().Jobs(namespace).Create(jobDef)
	job, err = clientset.BatchV1().Jobs(namespace).Get(jobDef.Name, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}

	info(fmt.Sprintf("Job %s started", job.Name))
	monitor(run, job)
}

func info(message string) {
	log.Printf("INFO: %s", message)
}

func monitor(run *models.Run, job *batchv1.Job) {
	for {
		content, err := sendRequest(http.MethodGet, fmt.Sprintf("run/%d/tasks", run.ID), nil, "Fail to query tests. Reason %s. Exception %s.")
		if err != nil {
			log.Println(err.Error())
			continue
		}

		var tasks []models.Task
		if err := json.Unmarshal(content, &tasks); err != nil {
			log.Println(err.Error())
			continue
		}

		statuses := make(map[string]int)
		for _, task := range tasks {
			statuses[task.Status]++
		}

		statusInfo := make([]string, 0, len(statuses))
		for name, count := range statuses {
			statusInfo = append(statusInfo, fmt.Sprintf("%s=%d", name, count))
		}

		info(strings.Join(statusInfo, "|"))

		lostTask := make([]int, 0, 10) // those tests where pod crashes during execution therfore entering limbo
		podList, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "job-name = " + job.Name})
		if err != nil {
			log.Println(err.Error())
			continue
		}

		for _, task := range tasks {
			if task.Status != "schedules" {
				continue
			}

			if agent, ok := task.ResultDetails["agent"]; ok {
				podName := strings.Split(agent.(string), "@")[0]
				if len(podName) > 0 {
					for _, pod := range podList.Items {
						if pod.ObjectMeta.Name == podName {
							if pod.Status.Phase != corev1.PodRunning {
								lostTask = append(lostTask, task.ID)
							}
						}
					}
				}
			}
		}

		if _, ok := statuses["initialized"]; !ok {
			if _, ok := statuses["scheduled"]; !ok {
				info(fmt.Sprintf("Run %d is finished", run.ID))
				report(run)
				os.Exit(0)
			} else if statuses["scheduled"]-len(lostTask) == 0 {
				info(fmt.Sprintf("Run %d is finished despite %d lost tasks.", run.ID, len(lostTask)))
				report(run)
				os.Exit(0)
			}
		}

		time.Sleep(time.Second * 30)
	}
}

func report(run *models.Run) {
	info("Sending report...")
	if email, ok := run.Settings[common.KeyUserEmail]; ok {
		content := make(map[string]string)
		content["run_id"] = strconv.Itoa(run.ID)
		content["receivers"] = email.(string)

		body, err := json.Marshal(content)
		if err != nil {
			info("Fail to marshal JSON during request sending email.")
			return
		}

		info(string(body))
		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("http://%s/report", common.DNSNameEmailService),
			bytes.NewBuffer(body))
		if err != nil {
			info("Fail to create request to requesting email.")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			info("Fail to send request to email service.")
			return
		}

		if resp.StatusCode != http.StatusOK {
			info("The request may have failed.")
		}
	} else {
		info("Skip sending report")
	}
}

func queryTests(run *models.Run) []models.TaskSetting {
	info(fmt.Sprintf("Expecting script %s.", common.PathScriptGetIndex))
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
		info(fmt.Sprintf("Query string is '%s'", query))
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

func sendRequest(method string, path string, body interface{}, templateError string) ([]byte, error) {
	var content []byte
	if body != nil {
		var err error
		content, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf(templateError, "unable to marshal the body in JSON.")
		}
	}

	req, err := httputils.CreateRequest(method, path, content)
	if err != nil {
		return nil, fmt.Errorf(templateError, "unable to create request", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(templateError, "http request failure", err)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(templateError, "unable to read respond body", err)
	}

	if resp.StatusCode >= 300 {
		reason := fmt.Sprintf("status code: %d.", resp.StatusCode) + " Body %s"
		return nil, fmt.Errorf(templateError, fmt.Sprintf(reason, string(b)), "N/A")
	}

	return b, nil
}

// Read run data and update the run details accordingly. exit the program as fatal if failed.
func getRun(runID int) (result *models.Run) {
	templateError := fmt.Sprintf("Fail to get the run %d.", runID) + " Reason %s. Exception %s."
	content, err := sendRequest(http.MethodGet, fmt.Sprintf("run/%d", runID), nil, templateError)
	if err != nil {
		log.Fatalf(fmt.Errorf(templateError, "http failure", err).Error())
	}

	var run models.Run
	err = json.Unmarshal(content, &run)
	if err != nil {
		log.Fatalf(fmt.Errorf(templateError, "json unmarshal failure", err).Error())
	}

	info(fmt.Sprintf("Find run %d: %s.", run.ID, run.Name))
	if run.Details != nil {
		info("  Details")
		for key, value := range run.Details {
			info(fmt.Sprintf("    %s = %s", key, value))
		}
	}
	if run.Settings != nil {
		info("  Settings")
		for key, value := range run.Settings {
			info(fmt.Sprintf("    %s = %s", key, value))
		}
	}

	info(fmt.Sprintf("Update run product in details to %s.", droidMetadata.Product))
	run.Details[common.KeyProduct] = droidMetadata.Product
	_, err = sendRequest(http.MethodPatch, fmt.Sprintf("run/%d", runID), run, templateError)
	if err != nil {
		log.Fatal(err.Error())
	}

	return &run
}

func postTasks(run *models.Run, settings []models.TaskSetting) (err error) {
	info(fmt.Sprintf("To schedule %d tests.", len(settings)))
	err = nil
	tasks := make([]models.Task, len(settings))

	for idx, setting := range settings {
		var task models.Task
		task.Name = fmt.Sprintf("Test: %s", setting.Classifier["identifier"])
		task.Settings = setting
		task.Annotation = run.Settings[common.KeyImageName].(string)

		tasks[idx] = task
	}

	templateError := fmt.Sprintf("Fail to create task for run %d.", run.ID) + " Reason %s. Exception %s."
	info("Posting tasks ...")
	_, err = sendRequest(http.MethodPost, fmt.Sprintf("run/%d/tasks", run.ID), tasks, templateError)
	info("Finish posting tasks ...")

	return
}

func createTaskJob(run *models.Run) (job *batchv1.Job, err error) {
	client, err := kubeutils.CreateKubeClientset()
	if err != nil {
		return nil, err
	}

	parallelism := int32(run.Settings[common.KeyInitParallelism].(float64))
	var backoff int32 = 5

	name := fmt.Sprintf("%s-%d-%s", droidMetadata.Product, run.ID, getRandomString())
	definition := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &parallelism,
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: corev1.PodSpec{
					Containers:       getContainerSpecs(run),
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
					ShareName:  "latest",
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

func getContainerSpecs(run *models.Run) (containers []corev1.Container) {
	c := corev1.Container{
		Name:    "main",
		Image:   run.Settings[common.KeyImageName].(string),
		Env:     getEnvironmentVariableDef(run),
		Command: []string{common.PathMountTools + "/a01droid", "-run", strconv.Itoa(run.ID)},
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

func getEnvironmentVariableDef(run *models.Run) []corev1.EnvVar {
	result := []corev1.EnvVar{
		corev1.EnvVar{
			Name:      common.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		corev1.EnvVar{
			Name:      common.EnvNodeName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
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
