package util

import (
	"errors"
	"time"
)

type BearerToken struct {
	token     string
	expiresIn int64
}

type RefreshAuthResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	ExtExpiresIn int64  `json:"ext_expires_in"`
	AccessToken  string `json:"access_token"`
}

func (b *BearerToken) IsExpired() bool {
	if b.token == "" {
		return true
	}

	currentTime := time.Now()
	tokenExpirationTime := time.Unix(b.expiresIn, 0)
	return currentTime.After(tokenExpirationTime)
}

func (b *BearerToken) GetToken() string {
	return b.token
}

type TokenClient struct {
	Token           *BearerToken
	refreshAuthFunc func() (*RefreshAuthResponse, error)
}

func NewBearerToken(token string) *BearerToken {
	return &BearerToken{token: token}
}

func NewTokenClient(authFunc func() (*RefreshAuthResponse, error)) *TokenClient {
	return &TokenClient{
		Token:           &BearerToken{},
		refreshAuthFunc: authFunc,
	}
}

func (c *TokenClient) RefreshAuth() error {
	if c.Token != nil {
		if !c.Token.IsExpired() {
			//fmt.Println("Token has not expired, reusing token from cache")
			return nil
		}
	}

	resp, err := c.refreshAuthFunc()
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New("unable to refresh authentication")
	}

	currentTime := time.Now()
	c.Token = &BearerToken{}
	c.Token.expiresIn = currentTime.Unix() + resp.ExpiresIn
	c.Token.token = resp.AccessToken

	return nil
}
