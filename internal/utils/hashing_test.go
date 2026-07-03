package utils

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/types"
	"testing"
)

func TestStreamHashBindsMessageAndFile(t *testing.T) {
	oldAPIHash := config.ValueOf.ApiHash
	t.Cleanup(func() {
		config.ValueOf.ApiHash = oldAPIHash
	})
	config.ValueOf.ApiHash = "secret"
	file := &types.File{
		FileName: "movie.mp4",
		FileSize: 1234,
		MimeType: "video/mp4",
		ID:       99,
	}

	hash := GenerateStreamHash(10, file)
	payload, err := VerifyStreamHash(hash, 10)
	if err != nil {
		t.Fatalf("expected generated hash to verify before fetching file: %v", err)
	}
	if payload.FileDigest == "" {
		t.Fatal("expected payload to include a file digest")
	}
	if !payload.MatchesFile(file) {
		t.Fatal("expected payload metadata to match file")
	}
	if !CheckStreamHash(hash, 10, file) {
		t.Fatal("expected generated hash to validate")
	}
	if _, err := VerifyStreamHash(hash, 11); err == nil {
		t.Fatal("hash verified for a different message id")
	}
	if CheckStreamHash(hash, 11, file) {
		t.Fatal("hash validated for a different message id")
	}

	changedFile := *file
	changedFile.FileSize++
	if CheckStreamHash(hash, 10, &changedFile) {
		t.Fatal("hash validated for different file metadata")
	}
}
