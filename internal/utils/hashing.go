package utils

import (
	"EverythingSuckz/fsb/internal/token"
	"EverythingSuckz/fsb/internal/types"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const streamTokenKind = "stream"

type StreamHashPayload struct {
	MessageID  int    `json:"message_id"`
	FileDigest string `json:"file_digest"`
}

func GenerateStreamHash(messageID int, file *types.File) string {
	payload := StreamHashPayload{
		MessageID:  messageID,
		FileDigest: FileDigest(file),
	}
	encoded, err := token.Encode(streamTokenKind, payload)
	if err != nil {
		return ""
	}
	return encoded
}

func VerifyStreamHash(inputHash string, messageID int) (StreamHashPayload, error) {
	var payload StreamHashPayload
	if err := token.Decode(inputHash, streamTokenKind, &payload); err != nil {
		return StreamHashPayload{}, errors.New("invalid hash payload")
	}
	if payload.MessageID != messageID {
		return StreamHashPayload{}, errors.New("invalid hash payload")
	}
	return payload, nil
}

func (p StreamHashPayload) MatchesFile(file *types.File) bool {
	return p.FileDigest != "" && p.FileDigest == FileDigest(file)
}

func FileDigest(file *types.File) string {
	if file == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%d", file.FileName, file.FileSize, file.MimeType, file.ID)))
	return hex.EncodeToString(sum[:])
}

func CheckStreamHash(inputHash string, messageID int, file *types.File) bool {
	payload, err := VerifyStreamHash(inputHash, messageID)
	if err != nil {
		return false
	}
	return payload.MatchesFile(file)
}
