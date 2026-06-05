package message

import "fmt"

const (
	// DeliverInstanceTopicPrefix routes delivery events to one application instance.
	// DeliverInstanceTopicPrefix 将投递事件路由到单个应用实例。
	DeliverInstanceTopicPrefix = "message.deliver.instance."
)

// DeliverInstanceTopic returns the routing key for one application instance.
// DeliverInstanceTopic 返回单个应用实例的投递路由键。
func DeliverInstanceTopic(instanceID string) string {
	return fmt.Sprintf("%s%s", DeliverInstanceTopicPrefix, instanceID)
}

// InstanceQueueName returns the queue name owned by one application instance.
// InstanceQueueName 返回单个应用实例拥有的队列名称。
func InstanceQueueName(baseQueue, instanceID string) string {
	return fmt.Sprintf("%s.%s", baseQueue, instanceID)
}
