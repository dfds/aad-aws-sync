package handlers

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetEventWithPayloadFromMsg(t *testing.T) {
	ep, err := GetEventWithPayloadFromMsg[memberJoinedCapability]([]byte(""))
	assert.Error(t, err)
	assert.Nil(t, ep)

	ep, err = GetEventWithPayloadFromMsg[memberJoinedCapability]([]byte("{}"))
	assert.NoError(t, err)
	assert.NotNil(t, ep)

	assert.Equal(t, ep.Payload.CapabilityId, "")
	assert.Equal(t, ep.Payload.MemberEmail, "")

	ep, err = GetEventWithPayloadFromMsg[memberJoinedCapability]([]byte("{\"eventName\":\"capability_created\",\"version\":\"0.1\",\"payload\":{\"capabilityId\":\"9999\", \"memberEmail\": \"dummy@dfds.cloud\"}}"))
	assert.NoError(t, err)
	assert.NotNil(t, ep)

	assert.Equal(t, ep.Payload.CapabilityId, "9999")
	assert.Equal(t, ep.Payload.MemberEmail, "dummy@dfds.cloud")
}
