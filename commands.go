package clamd

import (
	"context"
)

var c = DefaultClient

type PingResult struct {
	Raw string
}

func (c *Clamd) Ping(ctx context.Context, opts ...Option) (*PingResult, error) {
	cmd, err := NewCommand(CommandPing, "", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &PingResult{
		Raw: string(resp.Body),
	}, nil
}

func Ping(ctx context.Context, opts ...Option) (*PingResult, error) {
	return c.Ping(ctx, opts...)
}

type VersionResult struct {
	Raw string
}

func (c *Clamd) Version(ctx context.Context, opts ...Option) (*VersionResult, error) {
	cmd, err := NewCommand(CommandVersion, "", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &VersionResult{
		Raw: string(resp.Body),
	}, nil
}

func Version(ctx context.Context, opts ...Option) (*VersionResult, error) {
	return c.Version(ctx, opts...)
}
