package gateway

import (
	"fmt"

	"github.com/pico-cs/go-client/client"
)

// A CS represents a gateway command station.
type CS struct {
	config  *CSConfig
	gateway *Gateway
	client  *client.Client
}

// NewCS returns a new command station instance.
func NewCS(config *CSConfig, gateway *Gateway) (*CS, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	cs := &CS{config: config, gateway: gateway}

	// connect
	port, err := config.port()
	if err != nil {
		return nil, err
	}
	serial, err := client.NewSerial(port)
	if err != nil {
		return nil, err
	}
	cs.client = client.New(serial, cs.pushHandler)

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
	cs.gateway.subscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "get"), cs.enabled(cs.client))
	cs.gateway.subscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "set"), cs.setEnabled(cs.client))
}

func (cs *CS) unsubscribe() {
	cs.gateway.unsubscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "get"))
	cs.gateway.unsubscribe(cs, joinTopic("cs", cs.config.Name, "enabled", "set"))
}

func (cs *CS) pushHandler(msg string) {} // TODO: push messages

func (cs *CS) enabled(client *client.Client) hndFn {
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

func (cs *CS) locoDir(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		return client.LocoDir(addr)
	}
}

func (cs *CS) setLocoDir(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		dir, ok := payload.(bool)
		if !ok {
			return nil, fmt.Errorf("setLocoDir: invalid dir type %T", payload)
		}
		return client.SetLocoDir(addr, dir)
	}
}

func (cs *CS) toggleLocoDir(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		return client.ToggleLocoDir(addr)
	}
}

func (cs *CS) locoSpeed(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		speed, err := client.LocoSpeed128(addr)
		if err == nil && speed > 0 {
			speed--
		}
		return speed, err
	}
}

func (cs *CS) setLocoSpeed(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		f64, ok := payload.(float64)
		if !ok {
			return nil, fmt.Errorf("setLocoSpeed: invalid speed type %T", payload)
		}
		speed := uint(f64)
		if speed != 0 {
			speed++ // skip emergency stop
		}
		return client.SetLocoSpeed128(addr, speed)
	}
}

func (cs *CS) stopLoco(client *client.Client, addr uint) hndFn {
	return func(payload any) (any, error) {
		return client.SetLocoSpeed128(addr, 1)
	}
}

func (cs *CS) locoFct(client *client.Client, addr, no uint) hndFn {
	return func(payload any) (any, error) {
		return client.LocoFct(addr, no)
	}
}

func (cs *CS) setLocoFct(client *client.Client, addr, no uint) hndFn {
	return func(payload any) (any, error) {
		fct, ok := payload.(bool)
		if !ok {
			return nil, fmt.Errorf("setLocoFct: invalid fct type %T", payload)
		}
		return client.SetLocoFct(addr, no, fct)
	}
}

func (cs *CS) toggleLocoFct(client *client.Client, addr, no uint) hndFn {
	return func(payload any) (any, error) {
		return client.ToggleLocoFct(addr, no)
	}
}

// AddLoco adds a loco via a loco configuration to the command station.
func (cs *CS) AddLoco(config *LocoConfig) error {
	if err := config.validate(); err != nil {
		return err
	}
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir", "get"), cs.locoDir(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir", "set"), cs.setLocoDir(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "dir", "toggle"), cs.toggleLocoDir(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed", "get"), cs.locoSpeed(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed", "set"), cs.setLocoSpeed(cs.client, config.Addr))
	cs.gateway.subscribe(cs, joinTopic("loco", config.Name, "speed", "stop"), cs.stopLoco(cs.client, config.Addr))
	for name, locoConfig := range config.Fcts {
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name, "get"), cs.locoFct(cs.client, config.Addr, locoConfig.No))
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name, "set"), cs.setLocoFct(cs.client, config.Addr, locoConfig.No))
		cs.gateway.subscribe(cs, joinTopic("loco", config.Name, name, "toggle"), cs.toggleLocoFct(cs.client, config.Addr, locoConfig.No))
	}
	return nil
}
