package clamd

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

var (
	DefaultChunkSize    = 1 << 10
	DefaultResponseSize = 1 << 10
)

func NewClient(opts ...Option) (*Client, error) {
	newClient := &Client{
		chunkSize: DefaultChunkSize,
	}

	if err := newClient.apply(opts...); err != nil {
		return nil, err
	}

	return newClient, nil
}

type Client struct {
	dialFunc  func() (net.Conn, error)
	conn      net.Conn
	chunkSize int
}

type Option func(*Client) error

func WithUnixSocket(path string) Option {
	return func(c *Client) error {
		c.dialFunc = func() (net.Conn, error) {
			return net.Dial("unix", path)
		}

		return nil
	}
}

func WithTCPSocket(path string) Option {
	return func(c *Client) error {
		c.dialFunc = func() (net.Conn, error) {
			return net.Dial("tcp", path)
		}

		return nil
	}
}

func WithChunkSize(v int) Option {
	return func(c *Client) error {
		c.chunkSize = v
		return nil
	}
}

func (c *Client) apply(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) copy() Client {
	return *c
}

func (c *Client) Dial(ctx context.Context, opts ...Option) error {
	conn, err := c.dialFunc()
	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

type PingResult struct{}

func (c *Client) Ping(ctx context.Context, opts ...Option) (*PingResult, error) {
	payload := [][]byte{
		nCmd("PING"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	if bytes.HasPrefix(resp, []byte("PONG")) {
		return &PingResult{}, nil
	}

	return nil, fmt.Errorf("Unknown response: %v", string(resp))
}

type VersionResult struct {
	Raw string
}

func (c *Client) Version(ctx context.Context, opts ...Option) (*VersionResult, error) {
	payload := [][]byte{
		nCmd("VERSION"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &VersionResult{
		Raw: string(resp),
	}, nil
}

type ReloadResult struct {
	Raw string
}

func (c *Client) Reload(ctx context.Context, opts ...Option) (*ReloadResult, error) {
	payload := [][]byte{
		nCmd("RELOAD"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &ReloadResult{
		Raw: string(resp),
	}, nil
}

type ScanResult struct {
	Raw string
}

func (c *Client) Scan(ctx context.Context, path string, opts ...Option) (*ScanResult, error) {
	payload := [][]byte{
		nCmd(fmt.Sprintf("SCAN %s", path)),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &ScanResult{
		Raw: string(resp),
	}, nil
}

type ContscanResult struct {
	Raw string
}

func (c *Client) Contscan(ctx context.Context, path string, opts ...Option) (*ContscanResult, error) {
	payload := [][]byte{
		nCmd(fmt.Sprintf("CONTSCAN %s", path)),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &ContscanResult{
		Raw: string(resp),
	}, nil
}

type MultiscanResult struct {
	Raw string
}

func (c *Client) Multiscan(ctx context.Context, path string, opts ...Option) (*MultiscanResult, error) {
	payload := [][]byte{
		nCmd(fmt.Sprintf("MULTISCAN %s", path)),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &MultiscanResult{
		Raw: string(resp),
	}, nil
}

type InstreamResponse struct {
	Raw string
}

func (c *Client) Instream(ctx context.Context, stream []byte, opts ...Option) (*InstreamResponse, error) {
	chunks := splitStream(string(stream), c.chunkSize)

	payload := make([][]byte, 0, len(chunks)+1)

	payload = append(payload, zCmd("INSTREAM"))
	payload = append(payload, chunks...)

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &InstreamResponse{
		Raw: string(resp),
	}, nil
}

func zCmd(s string) []byte {
	return []byte(fmt.Sprintf("z%s\x00", s))
}

func nCmd(s string) []byte {
	return []byte(fmt.Sprintf("n%s\n", s))
}

type Result struct {
	Content io.ReadCloser
}

func (c *Client) sendCommand(ctx context.Context, stream [][]byte, opts ...Option) ([]byte, error) {
	if err := c.Dial(ctx); err != nil {
		return nil, err
	}

	defer c.Close()

	respChan := make(chan []byte, 1)
	defer close(respChan)

	errChan := make(chan error, 1)
	defer close(errChan)

	go func() {

		for _, i := range stream {
			if _, err := c.conn.Write(i); err != nil {
				errChan <- err
				return

			}
		}

		rawResp := make([]byte, 1024*1024)
		if _, err := c.conn.Read(rawResp); err != nil {
			errChan <- err
			return
		}

		if len(rawResp) != 0 {
			respChan <- rawResp

			return
		}

		errChan <- fmt.Errorf("Unknown response: %v", string(rawResp))
	}()

	select {
	case <-ctx.Done():
		if err := c.Close(); err != nil {
			return nil, err
		}

		return nil, ctx.Err()
	case r := <-respChan:
		return r, nil
	case e := <-errChan:
		return nil, e
	}
}

type StatsResult struct {
	Raw string
}

func (c *Client) Stats(ctx context.Context, opts ...Option) (*StatsResult, error) {
	payload := [][]byte{
		nCmd("VERSION"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &StatsResult{
		Raw: string(resp),
	}, nil
}

type VersioncommandsResult struct {
	Raw string
}

func (c *Client) Versioncommands(ctx context.Context, opts ...Option) (*VersioncommandsResult, error) {
	payload := [][]byte{
		nCmd("VERSIONCOMMANDS"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &VersioncommandsResult{
		Raw: string(resp),
	}, nil
}

type IdsessionResult struct {
	Raw string
}

func (c *Client) Idsession(ctx context.Context, opts ...Option) (*IdsessionResult, error) {
	payload := [][]byte{
		nCmd("IDSESSION"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &IdsessionResult{
		Raw: string(resp),
	}, nil
}

type EndResult struct {
	Raw string
}

func (c *Client) End(ctx context.Context, opts ...Option) (*EndResult, error) {
	payload := [][]byte{
		nCmd("END"),
	}

	resp, err := c.sendCommand(ctx, payload)
	if err != nil {
		return nil, err
	}

	return &EndResult{
		Raw: string(resp),
	}, nil
}

func splitStream(content string, size int) [][]byte {
	var (
		rawParts = []byte(content)
		length   = len(rawParts) / size
		rem      = len(rawParts) % size
	)

	nItems := length
	if rem > 0 {
		nItems++
	}

	chunks := make([][]byte, 0, nItems)

	for i := 0; i < length; i++ {
		buf := new(bytes.Buffer)

		binary.Write(buf, binary.BigEndian, uint32(size))
		buf.Write(rawParts[i*size : i*size+size])
		chunks = append(chunks, buf.Bytes())
	}

	if rem > 0 {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, uint32(rem))
		buf.Write(rawParts[len(rawParts)-(rem):])
		chunks = append(chunks, buf.Bytes())
		buf.Reset()
	}

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, uint32(0))
	chunks = append(chunks, buf.Bytes())

	return chunks
}
