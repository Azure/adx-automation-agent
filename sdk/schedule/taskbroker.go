package schedule

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"log"

	"github.com/Azure/adx-automation-agent/sdk/kubeutils"

	"github.com/Azure/adx-automation-agent/sdk/common"
	"github.com/Azure/adx-automation-agent/sdk/models"
	"github.com/streadway/amqp"
)

// TaskBroker represents an instance of message broker used in the A01 system
type TaskBroker struct {
	ConnectionName string
	connected      bool
	channel        *amqp.Channel
	connection     *amqp.Connection
	declaredQueues []string
}

// GetChannel returns the channel to this task broker. If a channel hasn't been
// establised, a new channel as well as a connection will be created.
func (broker *TaskBroker) GetChannel() (*amqp.Channel, error) {
	if broker.channel == nil {
		if broker.connection == nil {
			conn, err := amqp.Dial(broker.ConnectionName)
			if err != nil {
				broker.connection = nil
				return nil, err
			}
			broker.connection = conn
		}

		ch, err := broker.connection.Channel()
		if err != nil {
			broker.Close()
			return nil, err
		}

		// ensure fair fetch
		err = ch.Qos(
			1,     // perfetch count
			0,     // prefetch size
			false, // global
		)
		if err != nil {
			broker.Close()
			return nil, err
		}

		broker.channel = ch
	}

	return broker.channel, nil
}

// QueueDeclare declare a queue associated with the given name. It returns the
// queue as well as the channel associate with this connetion. If a channel has
// not been established, a new one will be created.
func (broker *TaskBroker) QueueDeclare(name string) (queue amqp.Queue, ch *amqp.Channel, err error) {
	ch, err = broker.GetChannel()
	if err != nil {
		return amqp.Queue{}, nil, err
	}

	queue, err = ch.QueueDeclare(
		name,  // queue name
		true,  // durable
		true,  // delete when used
		false, // exclusive
		false, // no-wait
		nil,   // argument
	)

	broker.declaredQueues = append(broker.declaredQueues, queue.Name)

	return
}

// PublishTasks publishes the tasks to the queue specified by the given name. The queue will be
// declared if it doesn't already exist.
func (broker *TaskBroker) PublishTasks(queueName string, settings []models.TaskSetting) (err error) {
	logrus.Info(fmt.Sprintf("To schedule %d tests.", len(settings)))

	_, ch, err := broker.QueueDeclare(queueName)
	if err != nil {
		// TODO: update run's status in DB to failed
		return fmt.Errorf("fail to decalre queue: %s", err.Error())
	}

	logrus.Info(fmt.Sprintf("Declared queue %s. Begin publishing tasks ...", queueName))
	for _, setting := range settings {
		body, err := json.Marshal(setting)
		if err != nil {
			logrus.Warnf("Fail to marshal task %s setting in JSON. Error %s. The task is skipped.", setting, err.Error())
			continue
		}

		err = ch.Publish(
			"",        // default exchange
			queueName, // routing key
			false,     // mandatory
			false,     // immediate
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         body,
			})

		if err != nil {
			logrus.Warnf("Fail to publish task %s. Error %s. The task is skipped.", setting, err.Error())
		}
	}

	logrus.Info("Finish publish tasks")

	return nil
}

// Close the channel and connection
func (broker *TaskBroker) Close() {
	for _, queueName := range broker.declaredQueues {
		broker.channel.QueueDelete(queueName, false, false, true)
	}

	if broker.channel != nil {
		defer broker.channel.Close()
	}

	if broker.connection != nil {
		defer broker.connection.Close()
	}
}

// CreateLocalTaskBroker returns a TaskBroker instance used in local testing.
// The instance expects a message broker running at local 5672 port.
func CreateLocalTaskBroker() *TaskBroker {
	return &TaskBroker{
		ConnectionName: "amqp://localhost:5672",
	}
}

// CreateInClusterTaskBroker returns a in-cluster task broker instance
func CreateInClusterTaskBroker() *TaskBroker {
	endpoint, exists := kubeutils.TryGetSystemConfig(common.ConfigKeyEndpointTaskBroker)
	if !exists {
		log.Fatalln("Fail to fetch taskbroker's endpoint from system config.")
	}

	username, exists := kubeutils.TryGetSystemConfig(common.ConfigKeyUsernameTaskBroker)
	if !exists {
		log.Fatalln("Fail to fetch taskbroker's user name from system config.")
	}

	secretname, exists := kubeutils.TryGetSystemConfig(common.ConfigKeySecretTaskBroker)
	if !exists {
		log.Fatalln("Fail to fetch taskbroker's secret name from system config.")
	}

	passwordKey, exists := kubeutils.TryGetSystemConfig(common.ConfigKeyPasswordKeyTaskBroker)
	if !exists {
		log.Fatalln("Fail to fetch taskbroker's password key name from system config.")
	}

	passwordInBytes, exists := kubeutils.TryGetSecretInBytes(secretname, passwordKey)
	if !exists {
		log.Fatalf("Fail to fetch taskbroker's password from secret %s using key %s.", secretname, passwordKey)
	}

	password := string(passwordInBytes)

	return &TaskBroker{
		ConnectionName: fmt.Sprintf("amqp://%s:%s@%s:5672", username, password, endpoint),
	}
}
