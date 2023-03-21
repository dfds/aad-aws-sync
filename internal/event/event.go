package event

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kelseyhightower/envconfig"
	"github.com/segmentio/kafka-go"
	"go.dfds.cloud/aad-aws-sync/internal/config"
	"go.dfds.cloud/aad-aws-sync/internal/event/handlers"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
	"go.dfds.cloud/aad-aws-sync/internal/kafkautil"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
	"io"
	"sync"
)

func GetEventFromMsg(data []byte) (*model.Envelope, error) {
	var payload *model.Envelope
	err := json.Unmarshal(data, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func commitMsg(ctx context.Context, msg kafka.Message, consumer *kafka.Reader) error {
	err := consumer.CommitMessages(ctx, msg)
	if err != nil {
		return err
	}

	msgLog := util.Logger.With(zap.String("topic", msg.Topic),
		zap.Int("partition", msg.Partition),
		zap.Int64("offset", msg.Offset))
	msgLog.Debug("Commit for consumer group updated")

	return nil
}

func StartEventHandlers(ctx context.Context, conf config.Config, wg *sync.WaitGroup) error {
	var authConfig kafkautil.AuthConfig
	err := envconfig.Process("AAS_KAFKA_AUTH", &authConfig)
	if err != nil {
		return err
	}

	var consumerConfig kafkautil.ConsumerConfig
	err = envconfig.Process("AAS_KAFKA_CONSUMER", &consumerConfig)
	if err != nil {
		return errors.New("failed to process consumer configurations")
	}

	var producerConfig kafkautil.ProducerConfig
	err = envconfig.Process("AAS_KAFKA_PRODUCER", &producerConfig)
	if err != nil {
		return errors.New("failed to process producer configurations")
	}

	var errorProducerConfig kafkautil.ProducerConfig
	err = envconfig.Process("AAS_KAFKA_ERROR_PRODUCER", &errorProducerConfig)
	if err != nil {
		return errors.New("failed to process error producer configurations")
	}

	registry := NewRegistry()
	//registry.Register("capability_created", handlers.CapabilityCreatedHandler)
	registry.Register("member_joined_capability", handlers.MemberJoinedCapabilityHandler)
	registry.Register("member_left_capability", handlers.MemberLeftCapabilityHandler)

	dialer, err := kafkautil.NewDialer(authConfig)
	if err != nil {
		return err
	}

	consumer := kafkautil.NewConsumer(consumerConfig, authConfig, dialer)
	var cleanupOnce sync.Once
	cleanup := func() {
		util.Logger.Debug("Closing Kafka consumer")
		if err := consumer.Close(); err != nil {
			util.Logger.Fatal("Failed to close Kafka consumer", zap.Error(err))
		}
		util.Logger.Debug("Kafka consumer has been closed")
	}
	defer cleanupOnce.Do(cleanup)

	dlqProducer := kafkautil.NewProducer(errorProducerConfig, authConfig, dialer)

	wg.Add(1)
	defer wg.Done()
	for {
		util.Logger.Debug("Awaiting new message from topic")
		msg, err := consumer.FetchMessage(ctx)
		if err == io.EOF {
			util.Logger.Info("Connection closed")
			break
		} else if err == context.Canceled {
			util.Logger.Info("Processing canceled")
			break
		} else if err != nil {
			util.Logger.Error("Error fetching message", zap.Error(err))
			break
		}
		msgLog := util.Logger.With(zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.String("key", string(msg.Key)),
			zap.String("value", string(msg.Value)))
		msgLog.Debug("Message fetched")

		// Convert msg to Event
		var handler HandlerFunc
		var eventLog *zap.Logger

		event, err := GetEventFromMsg(msg.Value)
		if err != nil {
			msgLog.Info("Unable to deserialise message payload. Quite likely the message is not valid JSON. Skipping message")
			goto CommitOffset
		}

		if event == nil || event.Name == "" {
			msgLog.Info("Unable to recognise event envelope, skipping message")
			goto CommitOffset
		}

		eventLog = util.Logger.With(zap.String("topic", msg.Topic),
			zap.Int("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
			zap.String("key", string(msg.Key)),
			zap.String("eventName", event.Name))

		handler = registry.GetHandler(event.Name)
		if handler == nil {
			eventLog.Info("No handler registered for event, skipping.")
			goto CommitOffset
		}

		err = handler(ctx, model.HandlerContext{
			Event: event,
			Msg:   msg.Value,
		})
		if err != nil {
			eventLog.Error("Handler for event failed. Forwarding event to DLQ", zap.Error(err))
			forwardedMsg := msg
			forwardedMsg.Topic = ""
			err = dlqProducer.WriteMessages(ctx, forwardedMsg)
			if err != nil {
				eventLog.Error("Unable to forward event to DLQ, stopping event loop.", zap.Error(err))
				cleanupOnce.Do(cleanup)
				return err
			}
		}

	CommitOffset:
		err = commitMsg(ctx, msg, consumer)
		if err != nil {
			msgLog.Error("Unable to update commit for consumer group", zap.Error(err)) // TODO: Trigger graceful shutdown
		}
	}

	cleanupOnce.Do(cleanup)

	return nil
}
