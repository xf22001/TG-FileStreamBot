package token

import (
	"EverythingSuckz/fsb/config"
	"strings"
	"testing"
)

type testPayload struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestTokenEncodeDecode(t *testing.T) {
	oldAPIHash := config.ValueOf.ApiHash
	t.Cleanup(func() {
		config.ValueOf.ApiHash = oldAPIHash
	})
	config.ValueOf.ApiHash = "secret"

	encoded, err := Encode("kind-a", testPayload{ID: 7, Name: "file"})
	if err != nil {
		t.Fatal(err)
	}

	var decoded testPayload
	if err := Decode(encoded, "kind-a", &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != 7 || decoded.Name != "file" {
		t.Fatalf("decoded %+v", decoded)
	}

	if err := Decode(encoded, "kind-b", &decoded); err == nil {
		t.Fatal("expected kind mismatch to fail")
	}

	tampered := strings.Replace(encoded, "A", "B", 1)
	if tampered != encoded {
		if err := Decode(tampered, "kind-a", &decoded); err == nil {
			t.Fatal("expected tampered token to fail")
		}
	}
}
