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
	TopicRoot string
	Host      string
	Port      string
	Username  string
	Password  string
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
// Name:
// name of the command station
// Host:
// pico_w host in case of TCP/IP connection
// Port:
// port in case of TCP/IP connection (pico_w) or serial port for pico
// Incls:
// is an array of regular expressions defining which set of locos should be controlled by this command station
// Excls:
// is an array of regular expressions defining which set of locos should not be controlled by this command station
// excluding regular expressions do have precedence over including regular expressions
type CSConfig struct {
	Name  string   `json:"name"`
	Host  string   `json:"host"`
	Port  string   `json:"port"`
	Incls []string `json:"incls"`
	Excls []string `json:"excls"`
}

func (c *CSConfig) String() string {
	return fmt.Sprintf("name %s host %s port %s, include %v, exclude %v", c.Name, c.Host, c.Port, c.Incls, c.Excls)
}

func (c *CSConfig) validate() error {
	if err := checkTopicLevelName(c.Name); err != nil {
		return fmt.Errorf("CSConfig name %s: %s", c.Name, err)
	}
	return nil
}

func (c *CSConfig) conn() (client.Conn, error) {
	if c.Host != "" { // TCP connection
		return client.NewTCPClient(c.Host, c.Port)
	}
	// serial connection
	return client.NewSerial(c.Port)
}

// LocoFctConfig represents configuration data for a loco function.
type LocoFctConfig struct {
	No uint `json:"no"`
}

// LocoConfig represents configuration data for a loco.
type LocoConfig struct {
	Name string                   `json:"name"`
	Addr uint                     `json:"addr"`
	Fcts map[string]LocoFctConfig `json:"fcts"`
}

func (c *LocoConfig) validate() error {
	if err := checkTopicLevelName(c.Name); err != nil {
		return fmt.Errorf("LocoConfig name %s: %s", c.Name, err)
	}
	return nil
}
