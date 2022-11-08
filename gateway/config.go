package gateway

import (
	"fmt"
	"net"

	"github.com/pico-cs/go-client/client"
)

// Default values.
const (
	DefaultTopicRoot = "pico-cs"
	DefaultHost      = "localhost"
	DefaultPort      = "1883"
)

// Config represents configuration data for the gateway.
type Config struct {
	TopicRoot string `json:"topicRoot"`
	Host      string `json:"host"`
	Port      string `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

func (c *Config) validate() error {
	if err := checkTopicLevelName(c.TopicRoot); err != nil {
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

func (c *Config) address() string { return net.JoinHostPort(c.Host, c.port()) }

// CSConfig represents configuration data for a command station.
type CSConfig struct {
	Name    string
	Port    string `json:"port"`
	Primary bool   `json:"primary"`
}

func (c *CSConfig) String() string {
	return fmt.Sprintf("name: %s post: %s, primary: %t", c.Name, c.Port, c.Primary)
}

func (c *CSConfig) validate() error {
	if err := checkTopicLevelName(c.Name); err != nil {
		return fmt.Errorf("CSConfig name %s: %s", c.Name, err)
	}
	return nil
}

func (c *CSConfig) port() (string, error) {
	if c.Port == "" {
		return client.SerialDefaultPortName()
	}
	return c.Port, nil
}

// LocoFctConfig represents configuration data for a loco function.
type LocoFctConfig struct {
	No uint `json:"no"`
}

// LocoConfig represents configuration data for a loco.
type LocoConfig struct {
	CSName string                   `json:"csName"`
	Name   string                   `json:"name"`
	Addr   uint                     `json:"addr"`
	Fcts   map[string]LocoFctConfig `json:"fcts"`
}

func (c *LocoConfig) validate() error {
	if err := checkTopicLevelName(c.Name); err != nil {
		return fmt.Errorf("LocoConfig name %s: %s", c.Name, err)
	}
	return nil
}
