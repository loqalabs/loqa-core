package bus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ambiware-labs/loqa-core/internal/config"
	"github.com/nats-io/nats.go"
)

// Client wraps NATS connection and JetStream context with minimal helpers.
type Client struct {
	conn *nats.Conn
	js   nats.JetStreamContext
	log  *slog.Logger
}

func Connect(ctx context.Context, cfg config.BusConfig, log *slog.Logger) (*Client, error) {
	if len(cfg.Servers) == 0 {
		return nil, errors.New("no NATS servers configured")
	}

	options := []nats.Option{
		nats.Name("loqa-runtime"),
		nats.Timeout(time.Duration(cfg.ConnectTimeout) * time.Millisecond),
	}

	if cfg.Username != "" || cfg.Password != "" {
		options = append(options, nats.UserInfo(cfg.Username, cfg.Password))
	}
	if cfg.Token != "" {
		options = append(options, nats.Token(cfg.Token))
	}
	if cfg.TLSInsecure {
		options = append(options, nats.Secure(&tls.Config{InsecureSkipVerify: true}))
	}

	url := strings.Join(cfg.Servers, ",")
	conn, err := nats.Connect(url, options...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	log.Info("connected to NATS", slog.String("servers", url))

	return &Client{
		conn: conn,
		js:   js,
		log:  log,
	}, nil
}

func (c *Client) Close() {
	if c == nil {
		return
	}
	c.log.Info("closing NATS connection")
	c.conn.Drain()
	c.conn.Close()
}

func (c *Client) Healthy() bool {
	return c != nil && c.conn != nil && c.conn.Status() == nats.CONNECTED
}

func (c *Client) JetStream() nats.JetStreamContext {
	return c.js
}

func (c *Client) Conn() *nats.Conn {
	return c.conn
}

func (c *Client) Logger() *slog.Logger {
	return c.log
}
