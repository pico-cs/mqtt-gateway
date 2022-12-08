package gateway

import (
	"fmt"
	"log"
	"net"
	"regexp"

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
	// Logger
	Logger *log.Logger
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

type filter struct {
	inclExpList []*regexp.Regexp
	exclExpList []*regexp.Regexp
	inclList    []string // cache result
}

func compile(items []string) ([]*regexp.Regexp, error) {
	r := make([]*regexp.Regexp, 0, len(items))
	for _, item := range items {
		re, err := regexp.Compile(item)
		if err != nil {
			return nil, err
		}
		r = append(r, re)
	}
	return r, nil
}

func newFilter(inclRegList, exclRegList []string) (*filter, error) {
	inclExpList, err := compile(inclRegList)
	if err != nil {
		return nil, err
	}
	exclExpList, err := compile(exclRegList)
	if err != nil {
		return nil, err
	}
	return &filter{inclExpList: inclExpList, exclExpList: exclExpList}, nil
}

func (f *filter) includes(name string) bool {
	incl := false
	for _, re := range f.inclExpList {
		if re.MatchString(name) {
			incl = true
			break
		}
	}
	if incl {
		for _, re := range f.exclExpList {
			if re.MatchString(name) {
				incl = false
				break
			}
		}
	}
	if incl { // cache entry
		f.inclList = append(f.inclList, name)
	}
	return incl
}

// Filter represents an including and excluding list of device names.
type Filter struct {
	// list of regular expressions defining which set of devices should be included
	Incls []string
	// list of regular expressions defining which set of devices should be excluded
	// excluding regular expressions do have precedence over including regular expressions
	Excls []string
}

func (f *Filter) String() string {
	return fmt.Sprintf("include %v exclude %v", f.Incls, f.Excls)
}

func (f *Filter) filter() (*filter, error) { return newFilter(f.Incls, f.Excls) }

// CSConfig represents configuration data for a command station.
type CSConfig struct {
	// command station name (used in topic)
	Name string
	// pico_w host in case of WiFi TCP/IP connection
	Host string
	// TCP/IP port (WiFi) or serial port (serial over USB)
	Port string
	// filter of devices for which this command station should be a primary device
	Primary Filter
	// filter of devices for which this command station should be a secondary device
	Secondary Filter
}

func (c *CSConfig) String() string {
	return fmt.Sprintf("name %s host %s port %s primary devices %s secondary devices %s",
		c.Name,
		c.Host,
		c.Port,
		c.Primary,
		c.Secondary,
	)
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

// LocoConfig represents configuration data for a loco.
type LocoConfig struct {
	// loco name (used in topic)
	Name string
	// loco decoder address
	Addr uint
	// loco function mapping (key is used in topic)
	Fcts map[string]LocoFct
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
