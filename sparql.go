package sparql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/garsue/sparql/logger"
)

// Client queries to its SPARQL endpoint.
type Client struct {
	HTTPClient http.Client
	Logger     logger.Logger
	Endpoint   string
	dialer     net.Dialer
	transport  http.Transport
	prefixes   map[string]URI
}

// Option sets an option to the SPARQL client.
type Option func(*Client) error

// Timeout sets the connection timeout duration. Also KeepAlive timeout.
func Timeout(timeout time.Duration) Option {
	return func(c *Client) error {
		c.dialer.Timeout = timeout
		c.dialer.KeepAlive = timeout
		return nil
	}
}

// MaxIdleConns sets max idle connections.
func MaxIdleConns(n int) Option {
	return func(c *Client) error {
		c.transport.MaxIdleConns = n
		return nil
	}
}

// IdleConnTimeout sets the maximum amount of time an idle
// (keep-alive) connection.
func IdleConnTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		c.transport.IdleConnTimeout = timeout
		return nil
	}
}

// Prefix sets a global PREFIX for all queries.
func Prefix(prefix string, uri URI) Option {
	return func(c *Client) error {
		c.prefixes[prefix] = uri
		return nil
	}
}

// New returns `sparql.Client`.
func New(endpoint string, opts ...Option) (*Client, error) {
	dialer := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}
	client := &Client{
		dialer: dialer,
		transport: http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Logger:   *logger.New(),
		Endpoint: endpoint,
		prefixes: make(map[string]URI),
	}
	client.HTTPClient.Transport = &client.transport
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Close closes this client
func (c *Client) Close() error {
	if c.HTTPClient.Transport == nil {
		return errors.New("already closed")
	}
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok {
		return fmt.Errorf("unknown RoundTripper, %+v", c.HTTPClient.Transport)
	}
	transport.CloseIdleConnections()
	c.HTTPClient.Transport = nil
	return nil
}

// Ping sends a HTTP HEAD request to the endpoint.
func (c *Client) Ping(ctx context.Context) error {
	request, err := http.NewRequest(http.MethodHead, c.Endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}
	defer c.Logger.LogCloseError(resp.Body)
	c.Logger.Debug.Printf("Ping %+v", resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SPARQL ping error. status code %d", resp.StatusCode)
	}
	return nil
}
