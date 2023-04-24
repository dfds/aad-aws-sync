package util

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var dummyToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJkdW1teSIsImlhdCI6MTY3MjkxMTg0NiwiZXhwIjoxNzA0MzYxNDQ2LCJhdWQiOiJkZmRzLmNsb3VkIiwic3ViIjoiZHVtbXlAZGZkcy5jbG91ZCIsIkdpdmVuTmFtZSI6ImR1bW15IiwiU3VybmFtZSI6IndlZWUiLCJFbWFpbCI6ImR1bW15QGRmZHMuY2xvdWQiLCJSb2xlIjoiVGVzdGVyIGJ5IGRheSwgZGVzdHJveWVyIG9mIGNsdXN0ZXJzIGF0IG5pZ2h0In0.l5nt0qWmeGyAsOzM6B-ipb0UpBQSunlt7VxFDv53rwI"

func TestBearerToken_GetToken(t *testing.T) {
	type fields struct {
		token     string
		expiresIn int64
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BearerToken{
				token:     tt.fields.token,
				expiresIn: tt.fields.expiresIn,
			}
			if got := b.GetToken(); got != tt.want {
				t.Errorf("GetToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBearerToken_IsExpired(t *testing.T) {
	obj := NewBearerToken("")
	obj.expiresIn = 1672482052
	assert.True(t, obj.IsExpired())

	obj = NewBearerToken(dummyToken)
	obj.expiresIn = 1672482052
	assert.True(t, obj.IsExpired())
}

func TestNewBearerToken(t *testing.T) {
	val := "weee"
	obj := NewBearerToken(val)
	assert.Equal(t, obj.GetToken(), val)
}

func TestNewTokenClient(t *testing.T) {
	tc := NewTokenClient(func() (*RefreshAuthResponse, error) {
		return nil, nil
	})

	assert.NotNil(t, tc)
}

func TestTokenClient_RefreshAuth(t *testing.T) {
	resp := &RefreshAuthResponse{
		TokenType:    "",
		ExpiresIn:    1672482052,
		ExtExpiresIn: 1672482052,
		AccessToken:  dummyToken,
	}

	tc := NewTokenClient(func() (*RefreshAuthResponse, error) {
		return resp, nil
	})

	err := tc.RefreshAuth()
	assert.NoError(t, err)

	resp.ExpiresIn = time.Now().Add(time.Minute * 100).Unix()
	resp.ExtExpiresIn = time.Now().Add(time.Minute * 100).Unix()

	err = tc.RefreshAuth()
	assert.NoError(t, err)

	tc = NewTokenClient(func() (*RefreshAuthResponse, error) {
		return resp, errors.New("i'm an error, reee")
	})

	err = tc.RefreshAuth()
	assert.Error(t, err)

	tc = NewTokenClient(func() (*RefreshAuthResponse, error) {
		return nil, nil
	})

	err = tc.RefreshAuth()
	assert.Error(t, err)
}
