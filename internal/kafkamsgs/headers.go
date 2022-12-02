package kafkamsgs

import (
	"github.com/segmentio/kafka-go/protocol"
)

const (
	HeaderKeyVersion   string = "Version"
	HeaderKeyEventName        = "Event Name"
	HeaderKeyError            = "Error"
)

const (
	EventNameCapabilityCreated   string = "capability_created"
	EventNameAzureADGroupCreated string = "azure_ad_group_created"
)

const (
	Version1 string = "1"
)

func EventHeaders(name, version string) []protocol.Header {
	return []protocol.Header{
		{
			Key:   HeaderKeyVersion,
			Value: []byte(version),
		},
		{
			Key:   HeaderKeyEventName,
			Value: []byte(name),
		},
	}
}
