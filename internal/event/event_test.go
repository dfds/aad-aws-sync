package event

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetEventFromMsg(t *testing.T) {
	envelope, err := GetEventFromMsg([]byte(""))
	assert.Error(t, err)
	assert.Nil(t, envelope)

	envelope, err = GetEventFromMsg([]byte("{}"))
	assert.NoError(t, err)
	assert.NotNil(t, envelope)
	assert.Equal(t, envelope.Name, "")
	assert.Equal(t, envelope.Version, "")

	envelope, err = GetEventFromMsg([]byte("{\"eventName\":\"capability_created\",\"version\":\"0.1\"}"))
	assert.NoError(t, err)
	assert.NotNil(t, envelope)
	assert.Equal(t, envelope.Name, "capability_created")
	assert.Equal(t, envelope.Version, "0.1")
}
