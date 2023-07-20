package clamd

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

type Option func(*Clamd) error

type Options struct {
	Network         string
	Address         string
	Dialer          func(ctx context.Context, network, addr string) (net.Conn, error)
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	DialTimeout     time.Duration
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	OnConnect       func(context.Context)
	TLSConfig       *tls.Config
}
