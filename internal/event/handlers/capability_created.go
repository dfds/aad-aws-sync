package handlers

import (
	"context"
	"errors"
	"go.dfds.cloud/aad-aws-sync/internal/event/model"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"go.uber.org/zap"
)

type capabilityCreated struct {
	CapabilityID   string `json:"capabilityId"`
	CapabilityName string `json:"capabilityName"`
}

func CapabilityCreatedHandler(ctx context.Context, event model.HandlerContext) error {
	util.Logger.Info("New Capability discovered. Creating entries in AAD")
	msg, err := GetEventWithPayloadFromMsg[capabilityCreated](event.Msg)
	if err != nil {
		return err
	}

	util.Logger.Info("Payload", zap.Any("payload", msg))

	return errors.New("triggering error on purpose")
}
