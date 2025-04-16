package messaging

type KafkaEvent = string

const (
	ProductCreatedEvent = "product_created"
	ProductUpdatedEvent = "product_updated"
	ProductDeletedEvent = "product_deleted"
)
