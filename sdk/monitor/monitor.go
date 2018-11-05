package monitor

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Azure/adx-automation-agent/sdk/common"
	"github.com/Azure/adx-automation-agent/sdk/kubeutils"
	"github.com/Azure/adx-automation-agent/sdk/models"
	"github.com/Azure/adx-automation-agent/sdk/schedule"
)

const (
	interval = time.Second * 30
)

var (
	namespace = common.GetCurrentNamespace("a01-prod")
	clientset = kubeutils.TryCreateKubeClientset()
)

// WaitTasks blocks the caller till the job finishes.
func WaitTasks(taskBroker *schedule.TaskBroker, run *models.Run) error {
	logrus.Info("Begin monitoring task execution ...")

	ch, err := taskBroker.GetChannel()
	if err != nil {
		return err
	}

	jobName := run.Details[common.KeyJobName]
	podListOpt := metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", jobName)}
	api := clientset.CoreV1()

	for {
		time.Sleep(interval)

		queue, err := ch.QueueInspect(jobName)
		if err != nil {
			logrus.Info("The queue doesn't exist. All tasks have been executed.")
			break
		}
		logrus.Infof("Queue: messages %d.", queue.Messages)

		if queue.Messages != 0 {
			// there are tasks to be run
			continue
		}

		// the number of the message in the queue is zero. make sure all the
		// pods in this job have finished
		podList, err := api.Pods(namespace).List(podListOpt)
		if err != nil {
			logrus.Warnf("Fail to list pod of %s: %s", jobName, err)
			continue
		}

		runningPods := 0
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}

		if runningPods != 0 {
			logrus.Infof("%d pod are still running.", runningPods)
			continue
		}

		// zero task in the queue and all pod stop.
		break
	}

	return nil
}
