package shared

// MessageQueue defines the interface for message queue implementations
type MessageQueue interface {
	Publish(topic string, body []byte) error
	Subscribe(handler func(topic string, body []byte, id string) error) error
	Close() error
}