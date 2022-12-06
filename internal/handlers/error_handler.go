package handlers

import (
	"context"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.uber.org/zap"
)

// PermanentErorrHandler writes the original message along with the error to
// the dead letter queue to be examined later.
func PermanentErrorHandler(ctx context.Context, event kafkamsgs.Event, oerr error) {
	log := GetLogger(ctx)
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
		log.Error("Error writing message to dead letter queue",
			zap.Error(err))
		return
	}
	log.Error("Permanent error while handling message", zap.Error(oerr))
}
