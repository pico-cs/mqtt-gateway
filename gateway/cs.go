package gateway

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/pico-cs/go-client/client"
)

// A CS represents a gateway command station.
type CS struct {
	config  *CSConfig
	gateway *Gateway
	client  *client.Client
	mu      sync.Mutex
	locos   map[string]*LocoConfig
	inclsRe []*regexp.Regexp
	exclsRe []*regexp.Regexp
}

// NewCS returns a new command station instance.
func NewCS(config *CSConfig, gateway *Gateway) (*CS, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	conn, err := config.conn()
	if err != nil {
		return nil, err
	}

	cs := &CS{config: config, gateway: gateway, locos: make(map[string]*LocoConfig)}

	for _, incl := range config.Incls {
		re, err := regexp.Compile(incl)
		if err != nil {
			return nil, err
		}
		cs.inclsRe = append(cs.inclsRe, re)
	}

	for _, excl := range config.Excls {
		re, err := regexp.Compile(excl)
		if err != nil {
			return nil, err
		}
		cs.exclsRe = append(cs.exclsRe, re)
	}

	cs.client = client.New(conn, cs.pushHandler)

	cs.subscribe()

	return cs, nil
}

func (cs *CS) String() string { return cs.config.String() }

// Close closes the command station and the underlying client connection.
func (cs *CS) Close() error {
	cs.unsubscribe()
	return cs.client.Close()
}

func (cs *CS) subscribe() {
	cs.gateway.subscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "get"), cs.getEnabled(cs.client))
	cs.gateway.subscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "set"), cs.setEnabled(cs.client))
}

func (cs *CS) unsubscribe() {
	cs.gateway.unsubscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "get"))
	cs.gateway.unsubscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "set"))
}

func (cs *CS) pushHandler(msg string) {} // TODO: push messages

func (cs *CS) getEnabled(client *client.Client) hndFn {
	return func(payload any) (any, error) {
		return client.Enabled()
	}
}

func (cs *CS) setEnabled(client *client.Client) hndFn {
	return func(payload any) (any, error) {
		enabled, ok := payload.(bool)
		if !ok {
			return nil, fmt.Errorf("setEnabled: invalid enabled type %[1]T value %[1]v", payload)
		}
		return client.SetEnabled(enabled)
	}
}

func (cs *CS) getLocoDir(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		return client.LocoDir(addr)
	}
}

func (cs *CS) setLocoDir(client *client.Client, addr uint, publish bool) hndFn {
	return func(payload any) (any, error) {
		dir, ok := payload.(bool)
		if !ok {
			return nil, fmt.Errorf("setLocoDir: invalid dir type %T", payload)
		}
		dir, err := client.SetLocoDir(addr, dir)
		if !publish {
			return nil, err
		}
		return dir, err
	}
}

func (cs *CS) toggleLocoDir(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		return client.ToggleLocoDir(addr)
	}
}

func (cs *CS) getLocoSpeed(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		speed, err := client.LocoSpeed128(addr)
		if err == nil && speed > 0 {
			speed--
		}
		return speed, err
	}
}

func (cs *CS) setLocoSpeed(client *client.Client, addr uint, publish bool) hndFn {
	return func(payload any) (any, error) {
		f64, ok := payload.(float64)
		if !ok {
			return nil, fmt.Errorf("setLocoSpeed: invalid speed type %T", payload)
		}
		speed := uint(f64)
		if speed != 0 {
			speed++ // skip emergency stop
		}
		speed, err := client.SetLocoSpeed128(addr, speed)
		if !publish {
			return nil, err
		}
		return speed, err
	}
}

func (cs *CS) stopLoco(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		return client.SetLocoSpeed128(addr, 1)
	}
}

func (cs *CS) getLocoFct(client *client.Client, addr, no uint) hndFn {
	return func(payload any) (any, error) {
		return client.LocoFct(addr, no)
	}
}

func (cs *CS) setLocoFct(client *client.Client, addr, no uint, publish bool) hndFn {
	return func(payload any) (any, error) {
		fct, ok := payload.(bool)
		if !ok {
			return nil, fmt.Errorf("setLocoFct: invalid fct type %T", payload)
		}
		fct, err := client.SetLocoFct(addr, no, fct)
		if !publish {
			return nil, err
		}
		return fct, err
	}
}

func (cs *CS) toggleLocoFct(client *client.Client, addr, no uint) hndFn {
	return func(payload any) (any, error) {
		return client.ToggleLocoFct(addr, no)
	}
}

// subscribeLocoActions subscribes to loco actions for a loco controlled by this command station.
func (cs *CS) subscribeLocoActions(config *LocoConfig) {
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir", "get"), cs.getLocoDir(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir", "set"), cs.setLocoDir(cs.client, config.Addr, true))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir", "toggle"), cs.toggleLocoDir(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed", "get"), cs.getLocoSpeed(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed", "set"), cs.setLocoSpeed(cs.client, config.Addr, true))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed", "stop"), cs.stopLoco(cs.client, config.Addr))
	for name, locoConfig := range config.Fcts {
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name, "get"), cs.getLocoFct(cs.client, config.Addr, locoConfig.No))
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name, "set"), cs.setLocoFct(cs.client, config.Addr, locoConfig.No, true))
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name, "toggle"), cs.toggleLocoFct(cs.client, config.Addr, locoConfig.No))
	}
}

// subscribeLocoListeners subscribes to loco listeners for a loco not controlled by this command station.
func (cs *CS) subscribeLocoListeners(config *LocoConfig) {
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir"), cs.setLocoDir(cs.client, config.Addr, false))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed"), cs.setLocoSpeed(cs.client, config.Addr, false))
	for name, locoConfig := range config.Fcts {
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name), cs.setLocoFct(cs.client, config.Addr, locoConfig.No, false))
	}
}

func (cs *CS) controlsLoco(locoName string) bool {
	incl := false
	for _, re := range cs.inclsRe {
		if re.MatchString(locoName) {
			incl = true
			break
		}
	}
	if incl {
		for _, re := range cs.exclsRe {
			if re.MatchString(locoName) {
				incl = false
				break
			}
		}
	}
	return incl
}

// AddLoco adds a loco via a loco configuration to the command station.
func (cs *CS) AddLoco(config *LocoConfig) (bool, error) {
	if err := config.validate(); err != nil {
		return false, err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.locos[config.Name]; ok {
		return false, fmt.Errorf("loco %s was already added", config.Name)
	}
	cs.locos[config.Name] = config

	if !cs.controlsLoco(config.Name) {
		cs.subscribeLocoListeners(config)
		return false, nil
	}
	cs.subscribeLocoActions(config)
	return true, nil
}
