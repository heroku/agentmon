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
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	maxRetries       = 3
	initialPauseTime = 1 * time.Millisecond
)

// TODO(apg): Should probably be parameterizable
//            allowing us to setup timeouts and such.
func send(req *http.Request) error {
	retries := 0
	pause := initialPauseTime

done:
	for {
		resp, err := http.DefaultClient.Do(req)
		switch {
		case err != nil:
			return err
		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			body, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("upstream service replied with status=%d: %q",
				resp.StatusCode, body)
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			break done
		default:
			if retries == maxRetries {
				return fmt.Errorf("no success after %d attempts", retries)
			}

			pause = pause * 2
			select {
			case <-req.Context().Done():
				return req.Context().Err()
			case <-time.After(pause):
				retries++
				continue done
			}
		}
	}
	return nil
}
