package model

type Envelope struct {
	Name    string `json:"eventName"`
	Version string `json:"version"`
}

type EnvelopeWithPayload[T any] struct {
	Name    string `json:"eventName"`
	Version string `json:"version"`
	Payload T      `json:"payload"`
}

type HandlerContext struct {
	Event *Envelope
	Msg   []byte
}
