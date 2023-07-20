package clamd

import (
	"context"
	"net"
)

func shouldRetry(err error) bool {
	if err == context.DeadlineExceeded {
		return false
	}

	if netErr, ok := err.(net.Error); ok {
		return !netErr.Timeout()
	}

	return false
}
