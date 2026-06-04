package message

import "fmt"

const (
	// DeliverTopicPrefix routes per-user delivery events on the topic exchange.
	// DeliverTopicPrefix 在 Topic 交换机上按用户路由投递事件。
	DeliverTopicPrefix = "message.deliver.user."
	// DeliverTopicPattern binds the gateway dispatch queue.
	// DeliverTopicPattern 用于绑定网关分发队列。
	DeliverTopicPattern = "message.deliver.user.#"
)

// DeliverTopic returns the routing key for delivering to one user.
// DeliverTopic 返回投递给单个用户的路由键。
func DeliverTopic(userID uint64) string {
	return fmt.Sprintf("%s%d", DeliverTopicPrefix, userID)
}
