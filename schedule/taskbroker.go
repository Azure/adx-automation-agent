package schedule

import (
	"fmt"

	"github.com/Azure/adx-automation-agent/common"
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
	return &TaskBroker{
		ConnectionName: fmt.Sprintf("amqp://%s:5672", common.DNSNameTaskBroker),
	}
}
