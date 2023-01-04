package server

import (
	"net"
)

// Default values.
const (
	DefaultHost = "localhost"
	DefaultPort = "50000"
)

// Config represents http configuration data for the gateway.
type Config struct {
	// HTTP gateway host
	Host string
	// HTTP Gateway port
	Port string
}

func (c *Config) port() string {
	if c.Port == "" {
		return DefaultPort
	}
	return c.Port
}

func (c *Config) addr() string { return net.JoinHostPort(c.Host, c.port()) }
