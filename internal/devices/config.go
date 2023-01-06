package devices

import (
	"fmt"
	"regexp"

	"github.com/pico-cs/go-client/client"
	"github.com/pico-cs/mqtt-gateway/internal/gateway"
	"golang.org/x/exp/slices"
)

// Configuration Types
const (
	CtCS   = "cs"
	CtLoco = "loco"
)

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
	Incls []string `json:"incls"`
	// list of regular expressions defining which set of devices should be excluded
	// excluding regular expressions do have precedence over including regular expressions
	Excls []string `json:"excls"`
}

// NewFilter returns a new Filter instance.
func NewFilter() *Filter {
	return &Filter{Incls: []string{}, Excls: []string{}}
}

func (f *Filter) filter() (*filter, error) { return newFilter(f.Incls, f.Excls) }

// CSIOConfig represents configuration data for a command station IO.
type CSIOConfig struct {
	// command station GPIO
	GPIO uint `json:"gpio"`
}

// CSConfig represents configuration data for a command station.
type CSConfig struct {
	// command station name (used in topic)
	Name string `json:"name"`
	// pico_w host in case of WiFi TCP/IP connection
	Host string `json:"host"`
	// TCP/IP port (WiFi) or serial port (serial over USB)
	Port string `json:"port"`
	// filter of devices for which this command station should be a primary device
	Primary *Filter `json:"primary"`
	// filter of devices for which this command station should be a secondary device
	Secondary *Filter `json:"secondary"`
	// command station IO mapping (key is used in topic)
	IOs map[string]CSIOConfig `json:"ios"`
}

// NewCSConfig returns a new CSConfig instance.
func NewCSConfig() *CSConfig {
	return &CSConfig{
		Primary:   NewFilter(),
		Secondary: NewFilter(),
		IOs:       map[string]CSIOConfig{},
	}
}

func (c *CSConfig) validate() error {
	if err := gateway.CheckLevelName(c.Name); err != nil {
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
	No uint `json:"no"`
}

// LocoConfig represents configuration data for a loco.
type LocoConfig struct {
	// loco name (used in topic)
	Name string `json:"name"`
	// loco decoder address
	Addr uint `json:"addr"`
	// loco function mapping (key is used in topic)
	Fcts map[string]LocoFctConfig `json:"fcts"`
}

// NewLocoConfig returns a new LocoConfig instance.
func NewLocoConfig() *LocoConfig {
	return &LocoConfig{Fcts: map[string]LocoFctConfig{}}
}

// ReservedFctNames is the list of reserved function names which cannot be used in loco configurations.
var ReservedFctNames = []string{"dir", "speed"}

// make sure, that reserved names cannot be changed.
var reservedFctNames = slices.Clone(ReservedFctNames)

func (c *LocoConfig) validate() error {
	if err := gateway.CheckLevelName(c.Name); err != nil {
		return fmt.Errorf("LocoConfig name %s: %s", c.Name, err)
	}
	for name := range c.Fcts {
		if slices.Contains(reservedFctNames, name) {
			return fmt.Errorf("LocoConfig name %s: function name %s is reserved", c.Name, name)
		}
	}
	return nil
}
