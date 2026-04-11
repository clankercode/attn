package tts

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPClientHasTimeout(t *testing.T) {
	if httpClient == http.DefaultClient {
		t.Fatal("providers should not use http.DefaultClient directly")
	}
	if httpClient.Timeout < 30*time.Second {
		t.Fatalf("provider timeout is too short: %s", httpClient.Timeout)
	}
}
