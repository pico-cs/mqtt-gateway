package gateway

import (
	"fmt"
	"net"
)

// Default values.
const (
	DefaultTopicRoot = "pico-cs"
	DefaultHost      = "localhost"
	DefaultPort      = "1883"
)

// Config represents mqtt configuration data for the gateway.
type Config struct {
	// root part of all gateway MQTT topics
	TopicRoot string
	// MQTT broker host
	Host string
	// MQTT broker port
	Port string
	// MQTT authentication username
	Username string
	// MQTT authentication password
	Password string
}

func (c *Config) validate() error {
	if err := CheckLevelName(c.TopicRoot); err != nil {
		return fmt.Errorf("MQTTConfig topicRoot %s: %s", c.TopicRoot, err)
	}
	return nil
}

func (c *Config) port() string {
	if c.Port == "" {
		return DefaultPort
	}
	return c.Port
}

func (c *Config) addr() string { return net.JoinHostPort(c.Host, c.port()) }
