package main

import (
	"crypto/rand"
	"encoding/base32"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/adx-automation-agent/sdk/common"
	"github.com/Azure/adx-automation-agent/sdk/kubeutils"
	"github.com/Azure/adx-automation-agent/sdk/models"
	"github.com/Azure/adx-automation-agent/sdk/monitor"
	"github.com/Azure/adx-automation-agent/sdk/reportutils"
	"github.com/Azure/adx-automation-agent/sdk/schedule"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	taskBroker    = schedule.CreateInClusterTaskBroker()
	namespace     = common.GetCurrentNamespace("a01-prod")
	droidMetadata = models.ReadDroidMetadata(common.PathMetadataYml)
	clientset     = kubeutils.TryCreateKubeClientset()
	version       = "Unknown"
	sourceCommit  = "Unknown"
)

// main defines the logic of A01 dispatcher
// Dispatcher coordinates a test job. There is one instance of dispatcher for each test run. The dispatcher will parse
// the test index and submit tasks to the task queue. Once the task queue is set up, the dispatcher begin monitoring the
// status of the queue. When it determines all the tasks are completed, the dispatcher will trigger a reporting and then
// exit.
func main() {
	logrus.Infof("A01 Droid Dispatcher.\nVersion: %s.\nCommit: %s.\n", version, sourceCommit)
	logrus.Infof("Pod name: %s", os.Getenv(common.EnvPodName))

	var pRunID *int
	pRunID = flag.Int("run", -1, "The run ID")
	flag.Parse()

	if *pRunID == -1 {
		log.Fatal("Missing runID")
	}

	// query the run and then update the product name in the details
	run, err := models.QueryRun(*pRunID)
	if err != nil {
		logrus.Fatal("fail to query the run")
	}

	if run.Status == common.RunStatusInitialized || len(run.Status) == 0 {
		run.Details[common.KeyProduct] = droidMetadata.Product
		run, err = run.SubmitChange()
		if err != nil {
			logrus.Fatal("fail to update the run: ", err)
		}

		logrus.Info(run)

		// generate a job name. the name will be used through out the remaining
		// session to identify the group of operations and resources
		jobName := fmt.Sprintf("%s-%d-%s", droidMetadata.Product, run.ID, getRandomString())

		// publish tasks to the task broker which will establish a worker queue
		err = taskBroker.PublishTasks(jobName, run.QueryTests())
		if err != nil {
			logrus.Fatal("Fail to publish tasks to the task broker:", err)
		}
		defer taskBroker.Close()

		// update the run status and add job name
		run.Status = common.RunStatusPublished
		run.Details[common.KeyJobName] = jobName
		run, err = run.SubmitChange()
		if err != nil {
			logrus.Fatal("fail to update the run: ", err)
		}
	}

	if run.Status == common.RunStatusPublished {
		jobName := run.Details[common.KeyJobName]

		// creates a kubernete job to manage test droid
		jobDef, err := createTaskJob(run, jobName)
		if err != nil {
			log.Fatal(err.Error())
		}

		// ignore this error for now. This API's latest version seems to sending
		// inaccurate error
		clientset.BatchV1().Jobs(namespace).Create(jobDef)
		_, err = clientset.BatchV1().Jobs(namespace).Get(jobDef.Name, metav1.GetOptions{})
		if err != nil {
			log.Fatal(err.Error())
		}

		run.Status = common.RunStatusRunning
		run, err = run.SubmitChange()
		if err != nil {
			logrus.Fatal("fail to update the run: ", err)
		}
	}

	if run.Status == common.RunStatusRunning {
		// begin monitoring the job status till the end
		monitor.WaitTasks(taskBroker, run)

		secret, err := kubeutils.TryCreateKubeClientset().
			CoreV1().
			Secrets(namespace).
			Get(run.GetSecretName(droidMetadata), metav1.GetOptions{})
		if err != nil {
			logrus.Fatal("Failed to get the kubernetes secret: ", err)
		}

		reportutils.RefreshPowerBI(run, run.GetSecretName(droidMetadata))

		owners := string(secret.Data["owners"])
		templateURL, ok := secret.Data["email.path.template"]
		if ok {
			reportutils.Report(run, strings.Split(owners, ","), string(templateURL))
		} else {
			logrus.Warn("Failed to get the `email.path.template` value from the kubernetes secret. A generic template will be used instead")
			reportutils.Report(run, strings.Split(owners, ","), "")
		}

		run.Status = common.RunStatusCompleted
		run, err = run.SubmitChange()
		if err != nil {
			logrus.Fatal("fail to update the run: ", err)
		}
	}

	if run.Status == common.RunStatusCompleted {
		logrus.Info(run)
		logrus.Infof("The run %d was already completed.", run.ID)
		os.Exit(0)
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////////
// Kubernetes JOB

func createTaskJob(run *models.Run, jobName string) (job *batchv1.Job, err error) {
	client, err := kubeutils.CreateKubeClientset()
	if err != nil {
		return nil, err
	}

	parallelism := int32(run.Settings[common.KeyInitParallelism].(float64))
	var backoff int32 = 5

	definition := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: getLabels(run),
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &parallelism,
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   jobName,
					Labels: getLabels(run),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "test-runner-robot",
					Containers:         getContainerSpecs(run, jobName),
					ImagePullSecrets:   getImagePullSource(run),
					Volumes:            getVolumes(run),
					RestartPolicy:      corev1.RestartPolicyNever,
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
		{
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

	for _, file := range droidMetadata.SecretFiles {
		volumes = append(volumes,
			corev1.Volume{
				Name: common.StorageVolumeNameSecrets,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: run.GetSecretName(droidMetadata),
						Items: []corev1.KeyToPath{
							{
								Key:  file.SecretKey,
								Path: file.Path,
							},
						},
					},
				},
			})
	}
	return
}

func getImagePullSource(run *models.Run) []corev1.LocalObjectReference {
	return []corev1.LocalObjectReference{{Name: run.Settings[common.KeyImagePullSecret].(string)}}
}

func getContainerSpecs(run *models.Run, jobName string) (containers []corev1.Container) {
	c := corev1.Container{
		Name:    "main",
		Image:   run.Settings[common.KeyImageName].(string),
		Env:     getEnvironmentVariableDef(run, jobName),
		Command: []string{common.PathMountTools + "/a01droid"},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			MountPath: common.PathMountTools,
			Name:      common.StorageVolumeNameTools,
		},
	}

	if droidMetadata.Storage {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: common.PathMountArtifacts,
			Name:      common.StorageVolumeNameArtifacts,
		})
	}

	if len(droidMetadata.SecretFiles) > 0 {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: common.PathMountSecrets,
			Name:      common.StorageVolumeNameSecrets,
		})
	}

	c.VolumeMounts = volumeMounts

	return []corev1.Container{c}
}

func getEnvironmentVariableDef(run *models.Run, jobName string) []corev1.EnvVar {
	result := []corev1.EnvVar{
		{
			Name: common.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		{
			Name: common.EnvNodeName,
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.nodeName"},
			},
		},
		{
			Name:  common.EnvJobName,
			Value: jobName,
		},
		{
			Name: common.EnvKeyInternalCommunicationKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "store-secrets"},
					Key:                  "comkey",
				},
			},
		},
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
