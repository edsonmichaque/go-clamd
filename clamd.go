package clamd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var (
	MaxChunkSize      = 1 << 10
	MaxResponseBuffer = 1 << 10
	defaultOptions    = &Options{
		Network:    "tcp",
		Address:    "localhost:3310",
		MaxRetries: 3,
	}
	DefaultClient = &Clamd{
		client: newClient(defaultOptions),
	}
)

func New(opt *Options) (*Clamd, error) {
	c := &Clamd{
		client: newClient(opt),
	}

	return c, nil
}

type Clamd struct {
	*client
}

type client struct {
	opt *Options
}

func newClient(opt *Options) *client {
	if opt.Network == "" {
		opt.Network = "tcp"
	}

	if opt.Address == "" {
		opt.Address = "localhost:3310"
	}

	if opt.Dialer == nil {
		opt.Dialer = newDialer(opt)
	}

	if opt.MaxRetries == 0 {
		opt.MaxRetries = defaultOptions.MaxRetries
	}

	c := &client{
		opt: opt,
	}

	return c
}

func (c client) dial(ctx context.Context) (*Conn, error) {
	opt := c.opt
	if opt == nil {
		opt = defaultOptions
	}

	conn, err := opt.Dialer(ctx, opt.Network, opt.Address)
	if err != nil {
		return nil, err
	}

	return &Conn{conn: conn}, nil
}

func newDialer(opt *Options) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := &net.Dialer{
			Timeout: opt.DialTimeout,
		}

		return dialer.DialContext(ctx, network, address)
	}
}

type Conn struct {
	conn net.Conn
}

type Command struct {
	Name string
	Body [][]byte
}

const (
	CommandShutdown        = "SHUTDOWN"
	CommandPing            = "PING"
	CommandReload          = "RELOAD"
	CommandVersion         = "VERSION"
	CommandScan            = "SCAN"
	CommandMuliScan        = "MULTISCAN"
	CommandVersionCommands = "VERSIONCOMMANDS"
	CommandInstream        = "INSTREAM"
	CommandContScan        = "CONTSCAN"
	CommandAllMatchScan    = "ALLMATCHSCAN"
	CommandStats           = "STATS"
)

type Result struct {
	Attempts int
	Body     []byte
}

func NewCommand(cmdName string, arg string, args io.ReadCloser) (*Command, error) {
	commandBuilders := map[string]func(string, string, io.ReadCloser) (*Command, error){
		CommandPing:            newCommand,
		CommandReload:          newCommand,
		CommandVersion:         newCommand,
		CommandStats:           newCommand,
		CommandVersionCommands: newCommand,
		CommandAllMatchScan:    newCommandWithArgs,
		CommandScan:            newCommandWithArgs,
		CommandContScan:        newCommandWithArgs,
		CommandMuliScan:        newCommandWithArgs,
		CommandInstream:        newCommandWithBody,
	}

	builder, ok := commandBuilders[cmdName]
	if !ok {
		return nil, errors.New("invalid command")
	}

	return builder(cmdName, arg, args)
}

func newCommand(cmd, arg string, body io.ReadCloser) (*Command, error) {
	return &Command{Name: fmt.Sprintf("n%s\n", cmd)}, nil
}

func newCommandWithArgs(cmd, arg string, body io.ReadCloser) (*Command, error) {
	return &Command{Name: fmt.Sprintf("n%s %s\n", cmd, arg)}, nil
}

func newCommandWithBody(cmd, arg string, body io.ReadCloser) (*Command, error) {
	name := fmt.Sprintf("z%s\x00", cmd)
	input, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return &Command{
		Name: name,
		Body: splitChunks(input, MaxChunkSize),
	}, nil
}

func (c *Clamd) Do(ctx context.Context, cmd *Command) (*Result, error) {
	var (
		res      *Result
		err      error
		attempts int
	)

	wait := 100 * time.Millisecond

	for attempts = 0; attempts < c.opt.MaxRetries; attempts++ {
		res, err = c.send(ctx, cmd)
		if err != nil {
			if shouldRetry(err) {
				if attempts != 0 {
					time.Sleep(wait)
					wait = wait * 2
				}

				continue
			}

			return nil, err
		}
	}

	res.Attempts = attempts + 1
	return res, nil

}

func (c *Clamd) send(ctx context.Context, cmd *Command) (*Result, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}

	defer conn.conn.Close()

	respChan := make(chan Result, 1)
	defer close(respChan)

	errChan := make(chan error, 1)
	defer close(errChan)

	go func() {

		if _, err := conn.conn.Write([]byte(cmd.Name)); err != nil {
			errChan <- err
			return
		}

		for _, i := range cmd.Body {
			if _, err := conn.conn.Write(i); err != nil {
				errChan <- err
				return

			}
		}

		var buf bytes.Buffer
		var count int

		for {
			chunk := make([]byte, 0, 8192)
			n, err := conn.conn.Read(chunk)
			if err != nil && err != io.EOF {
				errChan <- err
				return
			}

			count += n

			if _, err := buf.Write(chunk[:n]); err != nil {
				errChan <- err
				return
			}

			if n == 0 {
				break
			}
		}

		respChan <- Result{
			Body: buf.Bytes(),
		}

	}()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-respChan:
			return &r, nil
		case e := <-errChan:
			return nil, e
		}
	}
}
