package kafkamsgs

// CapabilityCreatedMessage is a message emitted by the capability services
// when a new capability is created.
//
// Example:
//
// {
//   "version": "1",
//   "eventName": "capability_created",
//   "x-correlationId": "e2c2dbf6-0318-4aa2-8765-15ef75c5def3",
//   "x-sender": "CapabilityService.WebApi, Version=1.0.0.0, Culture=neutral, PublicKeyToken=null",
//   "payload": {
//     "capabilityId": "2335d768-add3-4022-89ba-3d5c2b5cdba3",
//     "capabilityName": "Sandbox-samolak"
//   }
// }
type CapabilityCreatedMessage struct {
	Version        string                          `json:"version"`
	EventName      string                          `json:"eventName"`
	XCorellationID string                          `json:"x-corellationId"`
	XSender        string                          `json:"x-sender"`
	Payload        CapabilityCreatedMessagePayload `json:"payload"`
}

type CapabilityCreatedMessagePayload struct {
	CapabilityID   string `json:"capabilityId"`
	CapabilityName string `json:"capabilityName"`
}

// AzureADGroupCreatedMessage is a message emitted when an Azure AD group was
// created for a capability.
type AzureADGroupCreatedMessage struct {
	CapabilityName string `json:"capabilityName"`
	AzureADGroupID string `json:"azureAdGroupId"`
}
