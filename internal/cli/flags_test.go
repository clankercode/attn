package cli

import "testing"

func TestParseReturnsErrorForInvalidFlags(t *testing.T) {
	_, err := Parse([]string{"--definitely-not-a-real-flag", "hello"})
	if err == nil {
		t.Fatal("expected invalid flag to return an error")
	}
}
