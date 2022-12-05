package gateway

import (
	"fmt"
	"net"

	"github.com/pico-cs/go-client/client"
	"golang.org/x/exp/slices"
)

// Default values.
const (
	DefaultTopicRoot = "pico-cs"
	DefaultHost      = "localhost"
	DefaultPort      = "1883"
)

// Config represents configuration data for the gateway.
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
	// command station name (used in topic)
	Name string
	// pico_w host in case of WiFi TCP/IP connection
	Host string
	// TCP/IP port (WiFi) or serial port (serial over USB)
	Port string
	// list of regular expressions defining which set of objects should be controlled by this command station
	// as the primary command station
	Incls []string
	// list of regular expressions defining which set of objects should be controlled by this command station
	// as a secondary command station
	// excluding regular expressions do have precedence over including regular expressions
	Excls []string // List of regular expressions defining
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
	// loco decoder function number
	No uint
}

// LocoConfig represents configuration data for a loco.
type LocoConfig struct {
	// loco name (used in topic)
	Name string
	// loco decoder address
	Addr uint
	// loco function mapping (key is used in topic)
	Fcts map[string]LocoFctConfig
}

// ReservedFctNames is the list of reserved function names which cannot be used in loco configurations.
var ReservedFctNames = []string{"dir", "speed"}

func (c *LocoConfig) validate() error {
	if err := checkTopicLevelName(c.Name); err != nil {
		return fmt.Errorf("LocoConfig name %s: %s", c.Name, err)
	}
	for name := range c.Fcts {
		if slices.Contains(ReservedFctNames, name) {
			return fmt.Errorf("LocoConfig name %s: function name %s is reserved", c.Name, name)
		}
	}
	return nil
}
