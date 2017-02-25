package reporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendCancels(t *testing.T) {
	requestMade := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade <- struct{}{}
	})
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest failed with %q", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	cancel()

	err = send(req)
	if err == nil {
		t.Fatalf("expected cancellation error in send")
	}

	select {
	case <-time.After(10 * time.Millisecond):
	case <-requestMade:
		t.Errorf("request made despite being cancelled")
	}
}
