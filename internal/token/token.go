package token

import (
	"EverythingSuckz/fsb/config"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const Version = 1

type Envelope struct {
	Version int             `json:"v"`
	Kind    string          `json:"kind"`
	Data    json.RawMessage `json:"data"`
}

func Encode(kind string, payload any) (string, error) {
	if kind == "" {
		return "", errors.New("empty token kind")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	envelopeBytes, err := json.Marshal(Envelope{
		Version: Version,
		Kind:    kind,
		Data:    payloadBytes,
	})
	if err != nil {
		return "", err
	}
	signature := sign(envelopeBytes)
	return base64.RawURLEncoding.EncodeToString(envelopeBytes) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func Decode(input, expectedKind string, payload any) error {
	parts := strings.Split(input, ".")
	if len(parts) != 2 {
		return errors.New("invalid token")
	}
	envelopeBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("invalid token payload")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("invalid token signature")
	}
	if !hmac.Equal(signature, sign(envelopeBytes)) {
		return errors.New("invalid token signature")
	}

	var envelope Envelope
	if err := json.Unmarshal(envelopeBytes, &envelope); err != nil {
		return errors.New("invalid token payload")
	}
	if envelope.Version != Version {
		return errors.New("invalid token version")
	}
	if envelope.Kind != expectedKind {
		return fmt.Errorf("invalid token kind: %s", envelope.Kind)
	}
	if len(envelope.Data) == 0 {
		return errors.New("empty token payload")
	}
	if err := json.Unmarshal(envelope.Data, payload); err != nil {
		return errors.New("invalid token payload")
	}
	return nil
}

func sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, []byte(config.ValueOf.ApiHash))
	mac.Write([]byte(fmt.Sprintf("%d:", Version)))
	mac.Write(payload)
	return mac.Sum(nil)
}
