package model

type Envelope struct {
	Type      string `json:"type"`
	MessageId string `json:"messageId"`
}

type EnvelopeWithPayload[T any] struct {
	Type      string `json:"type"`
	MessageId string `json:"messageId"`
	Payload   T      `json:"data"`
}

type HandlerContext struct {
	Event *Envelope
	Msg   []byte
}
