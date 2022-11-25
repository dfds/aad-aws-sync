package handlers

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
)

// PermanentErorrHandler writes the original message along with the error to
// the dead letter queue to be examined later.
func PermanentErrorHandler(ctx context.Context, event kafkamsgs.Event, oerr error) {
	errorProducer := GetKafkaErrorProducer(ctx)
	err := errorProducer.WriteMessages(ctx, kafka.Message{
		Key:   event.Message.Key,
		Value: event.Message.Value,
		Headers: append(event.Message.Headers, protocol.Header{
			Key:   kafkamsgs.HeaderKeyError,
			Value: []byte(oerr.Error()),
		}),
	})
	if err != nil {
		log.Fatal("error writing message to dead letter queue", err)
	}
	log.Println("error while processing message, message with error written to dead letter queue", err)
}
