package clamd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
)

var (
	DefaultChunkSize    = 1 << 10
	DefaultResponseSize = 1 << 10
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

	c := &client{
		opt: opt,
	}

	return c
}

func (c client) dial(ctx context.Context) (*Conn, error) {
	conn, err := c.opt.Dialer(ctx, c.opt.Network, c.opt.Address)
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
	Body []byte
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
		Body: splitChunks(input, DefaultChunkSize),
	}, nil
}

func (c *Clamd) Do(ctx context.Context, cmd *Command) (*Result, error) {
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

		rawResp := make([]byte, 1024*1024)
		if _, err := conn.conn.Read(rawResp); err != nil {
			errChan <- err
			return
		}

		if len(rawResp) != 0 {
			respChan <- Result{
				Body: rawResp,
			}

			return
		}

		errChan <- fmt.Errorf("Unknown response: %v", string(rawResp))
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
