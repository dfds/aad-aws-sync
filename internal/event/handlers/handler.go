package handlers

import (
	"encoding/json"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
)

func GetEventWithPayloadFromMsg[T any](data []byte) (*model.EnvelopeWithPayload[T], error) {
	var payload *model.EnvelopeWithPayload[T]
	err := json.Unmarshal(data, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}
