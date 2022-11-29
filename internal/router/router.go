package router

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"go.dfds.cloud/aad-aws-sync/internal/kafkamsgs"
)

// TODO write unit tests for this router

// ConsumeMessages fetches messages from a Kafka topic, detects events, and
// routes the events to the appropriate handler.
// The handlers are expected to continue retrying on temporary errors,
// while permanent errors are expected to be written to a dead letter
// queue.
func ConsumeMessages(ctx context.Context) {
	consumer := GetKafkaConsumer(ctx)
	handlers := GetEventHandlers(ctx)

	for {
		log.Println("fetch messages")
		msg, err := consumer.FetchMessage(ctx)
		if err == io.EOF {
			log.Println("connection closed")
			break
		} else if err == context.Canceled {
			log.Println("processing canceled")
			break
		} else if err != nil {
			log.Fatal("error fetching message:", err)
		}
		log.Printf("message at topic/partition/offset %v/%v/%v: %s = %s : %v\n", msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value), msg.Headers)

		// Parse event metadata from the message to determine if we are expected to handle it
		event := kafkamsgs.NewEventFromMessage(msg)
		if event == nil || event.Name == "" {
			// Handle undetermined event name
			handlers.PermanentErrorHandler(ctx, *event, errors.New("unable to detect an event name"))
			goto CommitOffset
		}

		// Route the event to the appropriate handler based on event name and version
		switch event.Name {
		case kafkamsgs.EventNameCapabilityCreated:
			switch event.Version {
			case kafkamsgs.Version1:
				// Handle the event
				handlers.CapabilityCreatedHandler(ctx, *event)
			default:
				handlers.PermanentErrorHandler(ctx, *event, fmt.Errorf("unsupported version of the capability created event: %q\n", event.Version))
			}
		default:
			// Skip unhandled event
			log.Printf("skip processing unhandled event: %q\n", event.Name)
		}

		// Commit offset to the consumer group
	CommitOffset:
		if err := consumer.CommitMessages(ctx, msg); err != nil {
			log.Fatal("failed to commit offset:", err)
		}
	}
}
