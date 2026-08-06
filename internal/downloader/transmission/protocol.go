package transmission

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	maxResponseBytes = 1 << 20
	maxRequestBytes  = 1 << 20
)

type requestArguments interface{ requestArguments() }

type responseArguments interface {
	decodeArguments(json.RawMessage) error
}

type requestEnvelope struct {
	Method    string           `json:"method"`
	Arguments requestArguments `json:"arguments"`
}

type responseEnvelope struct {
	Result    *string         `json:"result"`
	Arguments json.RawMessage `json:"arguments"`
}

func marshalRequest(method string, arguments requestArguments) ([]byte, error) {
	body, err := json.Marshal(requestEnvelope{Method: method, Arguments: arguments})
	if err != nil {
		return nil, ErrTransport
	}
	if len(body) > maxRequestBytes {
		return nil, ErrRequestTooLarge
	}
	return body, nil
}

func decodeResponse(body []byte, target responseArguments) error {
	var envelope responseEnvelope
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&envelope); err != nil {
		return ErrMalformedResponse
	}
	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrMalformedResponse
	}
	if envelope.Result == nil || *envelope.Result != "success" {
		return ErrUpstreamResult
	}
	if envelope.Arguments == nil || target.decodeArguments(envelope.Arguments) != nil {
		return ErrMalformedResponse
	}
	return nil
}
