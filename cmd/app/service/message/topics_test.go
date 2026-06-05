package message

import "testing"

func TestInstanceRoutingNames(t *testing.T) {
	if got, want := DeliverInstanceTopic("pod-a"), "message.deliver.instance.pod-a"; got != want {
		t.Fatalf("DeliverInstanceTopic() = %q, want %q", got, want)
	}
	if got, want := InstanceQueueName("im.message.dispatch", "pod-a"), "im.message.dispatch.pod-a"; got != want {
		t.Fatalf("InstanceQueueName() = %q, want %q", got, want)
	}
}
