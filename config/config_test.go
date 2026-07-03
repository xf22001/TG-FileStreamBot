package config

import (
	"testing"
)

func TestCollectMultiTokensOnlyAcceptsNumberedVars(t *testing.T) {
	got := collectMultiTokens([]string{
		"MULTI_TOKEN1=token-a",
		"MULTI_TOKEN_TXT_FILE=tokens.txt",
		"MULTI_TOKEN_FOO=ignored",
		"MULTI_TOKEN2=token-b",
		"MULTI_TOKEN3=",
		"OTHER=value",
	})
	want := []string{"token-a", "token-b"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
