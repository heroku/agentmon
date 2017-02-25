package reporter

import (
	"fmt"
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
		case resp.StatusCode >= 500:
			return fmt.Errorf("upstream service replied with status=%d",
				resp.StatusCode)
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
