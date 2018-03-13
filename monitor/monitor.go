package monitor

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/kubeutils"
	"github.com/Azure/adx-automation-agent/models"
	"github.com/Azure/adx-automation-agent/schedule"
)

const (
	interval = time.Second * 30
)

var (
	namespace = common.GetCurrentNamespace("a01-prod")
	clientset = kubeutils.TryCreateKubeClientset()
)

// WaitTasks blocks the caller till the job finishes.
func WaitTasks(taskBroker *schedule.TaskBroker, run *models.Run) {
	common.LogInfo("Begin monitoring task execution ...")

	ch, err := taskBroker.GetChannel()
	common.PanicOnError(err, "Fail to establish channel to the task broker during monitoring.")

	jobName := run.Details[common.KeyJobName]
	podListOpt := metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", jobName)}
	api := clientset.CoreV1()

	for {
		time.Sleep(interval)

		queue, err := ch.QueueInspect(jobName)
		if err != nil {
			common.LogWarning(fmt.Errorf("Fail to insepct the queue %s: %s", jobName, err).Error())
			continue
		}
		common.LogInfo(fmt.Sprintf("Queue: messages %d.", queue.Messages))

		if queue.Messages != 0 {
			// there are tasks to be run
			continue
		}

		// the number of the message in the queue is zero. make sure all the
		// pods in this job have finished
		podList, err := api.Pods(namespace).List(podListOpt)
		if err != nil {
			common.LogWarning(fmt.Errorf("Fail to list pod of %s: %s", jobName, err).Error())
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
}
