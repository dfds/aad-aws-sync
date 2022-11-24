package kafkamsgs

import (
	"encoding/json"

	"github.com/segmentio/kafka-go"
)

type EventMetadata struct {
	Name    string
	Version string
}

// MessageMetadata represents some metadata provided within the event
// payload by some legacy services.
type MessageMetadata struct {
	Version   string `json:"version"`
	EventName string `json:"eventName"`
}

func GetEventMetadata(msg kafka.Message) *EventMetadata {
	var event EventMetadata

	// Check the headers of the message
	for _, header := range msg.Headers {
		switch header.Key {
		case HeaderKeyEventName:
			event.Name = string(header.Value)
		case HeaderKeyVersion:
			event.Version = string(header.Value)
		}
	}

	// Check for metadata within the message
	var md MessageMetadata
	err := json.Unmarshal(msg.Value, &md)
	if err == nil {
		if md.EventName != "" {
			event.Name = md.EventName
		}
		if md.Version != "" {
			event.Version = md.Version
		}
	}

	return &event
}
