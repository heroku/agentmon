// Copyright (c) 2017, Heroku Inc <metrics-feedback@heroku.com>.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// * Redistributions of source code must retain the above copyright
//   notice, this list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright
//   notice, this list of conditions and the following disclaimer in the
//   documentation and/or other materials provided with the distribution.
//
// * The names of its contributors may not be used to endorse or promote
//   products derived from this software without specific prior written
//   permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

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
