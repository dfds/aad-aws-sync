package capsvc

import (
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/utils/env"
	"net/http"
)

type Client struct {
	httpClient *http.Client
	host       string
	token      string
}

func (c *Client) prepareHttpRequest(h *http.Request) {
	h.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
}

func (c *Client) GetCapabilities() (*GetCapabilitiesResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/capabilities", c.host), nil)
	if err != nil {
		return nil, err
	}
	c.prepareHttpRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload *GetCapabilitiesResponse

	err = json.Unmarshal(rawData, &payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func NewCapSvcClient(host string) *Client {
	payload := &Client{
		httpClient: http.DefaultClient,
		token:      env.GetString("AAS_CAPSVC_TOKEN", ""),
		host:       host,
	}
	return payload
}
