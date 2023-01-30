package router

import (
	"context"
	"errors"
	"io"

	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
	"go.uber.org/zap"
)

// ErrUnsupportedEventVersion is returned when the detected version of the
// event is not supported by any of the existing handlers.
var ErrUnsupportedEventVersion = errors.New("unsupported event version")

// ConsumeMessages fetches messages from a Kafka topic, detects events, and
// routes the events to the appropriate handler.
// The handlers are expected to continue retrying on temporary errors,
// while permanent errors are expected to be written to a dead letter
// queue.
func ConsumeMessages(ctx context.Context) {
	log := GetLogger(ctx)
	consumer := GetKafkaConsumer(ctx)
	eventHandlers := GetEventHandlers(ctx)

	log.Info("Begin consuming messages")
	defer log.Info("Stopped consuming messages")

	for {
		log.Debug("Fetch message")
		msg, err := consumer.FetchMessage(ctx)
		if err == io.EOF {
			log.Info("Connection closed")
			break
		} else if err == context.Canceled {
			log.Info("Processing canceled")
			break
		} else if err != nil {
			log.Error("Error fetching message", zap.Error(err))
			break
		}
		msgLog := log.With(zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.String("key", string(msg.Key)),
			zap.String("value", string(msg.Value)))
		msgLog.Debug("Message fetched")

		// Parse event metadata from the message to determine if we are expected to handle it
		event := kafkamsgs.NewEventFromMessage(msg)
		var eventLog *zap.Logger
		var eventCtx context.Context
		if event == nil || event.Name == "" {
			// Handle undetermined event name
			eventHandlers.PermanentErrorHandler(context.WithValue(ctx, event_handlers.ContextKeyLogger, msgLog),
				*event, errors.New("unable to determine an event name"))
			goto CommitOffset
		}

		// Initiate the event logger and context
		eventLog = msgLog.With(zap.String("name", event.Name), zap.String("version", event.Version))
		eventCtx = context.WithValue(ctx, event_handlers.ContextKeyLogger, eventLog)
		eventLog.Info("Event detected")

		// Route the event to the appropriate handler based on event name and version
		switch event.Name {
		case kafkamsgs.EventNameCapabilityCreated:
			switch event.Version {
			case kafkamsgs.Version1:
				// Handle the event
				eventLog.Debug("Handling event")
				eventHandlers.CapabilityCreatedHandler(eventCtx, *event)
			default:
				eventLog.Error("Unsupported version of the capability created event")
				eventHandlers.PermanentErrorHandler(eventCtx, *event,
					ErrUnsupportedEventVersion)
			}
		default:
			// Skip unhandled event
			eventLog.Info("Skip processing unhandled event")
		}

		// Commit offset to the consumer group
	CommitOffset:
		if err := consumer.CommitMessages(ctx, msg); err != nil {
			log.Error("Failed to commit offset", zap.Error(err))
		}
	}
}
