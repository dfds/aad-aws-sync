package azure

import (
	"github.com/stretchr/testify/assert"
	"go.dfds.cloud/aad-aws-sync/internal/util"
	"net/http"
	"testing"
	"time"
)

var azureTestConfig = Config{
	TenantId:     "00000-00000-00000-00000-00000",
	ClientId:     "dummy",
	ClientSecret: "secretdummy",
}

func TestClient_HasTokenExpired(t *testing.T) {
	az := NewAzureClient(Config{})
	az.tokenClient = util.NewTokenClient(func() (*util.RefreshAuthResponse, error) {
		return &util.RefreshAuthResponse{
			TokenType:    "",
			ExpiresIn:    time.Now().Add(time.Minute * 100).Unix(),
			ExtExpiresIn: time.Now().Add(time.Minute * 100).Unix(),
			AccessToken:  "dummy",
		}, nil
	})

	assert.True(t, az.HasTokenExpired())

	err := az.RefreshAuth()
	assert.NoError(t, err)

	assert.False(t, az.HasTokenExpired())
}

func TestClient_RefreshAuth(t *testing.T) {
	az := NewAzureClient(Config{})
	az.tokenClient = util.NewTokenClient(func() (*util.RefreshAuthResponse, error) {
		return &util.RefreshAuthResponse{
			TokenType:    "",
			ExpiresIn:    time.Now().Add(time.Minute * 100).Unix(),
			ExtExpiresIn: time.Now().Add(time.Minute * 100).Unix(),
			AccessToken:  "dummy",
		}, nil
	})

	err := az.RefreshAuth()
	assert.NoError(t, err)
}

func TestClient_prepareHttpRequest(t *testing.T) {
	az := NewAzureClient(Config{})
	az.tokenClient = util.NewTokenClient(func() (*util.RefreshAuthResponse, error) {
		return &util.RefreshAuthResponse{
			TokenType:    "",
			ExpiresIn:    time.Now().Add(time.Minute * 100).Unix(),
			ExtExpiresIn: time.Now().Add(time.Minute * 100).Unix(),
			AccessToken:  "dummy",
		}, nil
	})

	req, err := http.NewRequest("GET", "http://dummy", nil)
	assert.NoError(t, err)

	err = az.prepareHttpRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, req.Header.Get("User-Agent"), "aad-aws-sync - github.com/dfds/aad-aws-sync")
	assert.Equal(t, req.Header.Get("Authorization"), "Bearer dummy")

}

func TestClient_prepareJsonRequest(t *testing.T) {
	az := NewAzureClient(Config{})
	az.tokenClient = util.NewTokenClient(func() (*util.RefreshAuthResponse, error) {
		return &util.RefreshAuthResponse{
			TokenType:    "",
			ExpiresIn:    time.Now().Add(time.Minute * 100).Unix(),
			ExtExpiresIn: time.Now().Add(time.Minute * 100).Unix(),
			AccessToken:  "dummy",
		}, nil
	})

	req, err := http.NewRequest("GET", "http://dummy", nil)
	assert.NoError(t, err)

	err = az.prepareJsonRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, req.Header.Get("Content-Type"), "application/json")
}

func TestGenerateAzureGroupDisplayName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateAzureGroupDisplayName(tt.args.name); got != tt.want {
				t.Errorf("GenerateAzureGroupDisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateAzureGroupMailPrefix(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateAzureGroupMailPrefix(tt.args.name); got != tt.want {
				t.Errorf("GenerateAzureGroupMailPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewAzureClient(t *testing.T) {
	az := NewAzureClient(Config{})
	assert.NotNil(t, az)
}
