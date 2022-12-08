package gateway

import (
	"fmt"
	"sync"

	"github.com/pico-cs/go-client/client"
)

// A CS represents a command station.
type CS struct {
	name      string
	gateway   *Gateway
	client    *client.Client
	primary   *filter
	secondary *filter
	hndCh     chan *hndMsg
	wg        *sync.WaitGroup
}

// newCS returns a new command station instance.
func newCS(config *CSConfig, gateway *Gateway) (*CS, error) {
	// copy csConfig (value parameter)
	if err := config.validate(); err != nil {
		return nil, err
	}

	conn, err := config.conn()
	if err != nil {
		return nil, err
	}

	primary, err := config.Primary.filter()
	if err != nil {
		return nil, err
	}
	secondary, err := config.Secondary.filter()
	if err != nil {
		return nil, err
	}

	cs := &CS{
		name:      config.Name,
		gateway:   gateway,
		primary:   primary,
		secondary: secondary,
		hndCh:     make(chan *hndMsg, defChanSize),
		wg:        new(sync.WaitGroup),
	}

	cs.client = client.New(conn, cs.pushHandler)

	cs.subscribe()

	// start go routine
	go cs.handler(cs.wg, cs.hndCh, gateway.pubCh, gateway.errCh)

	return cs, nil
}

func (cs *CS) String() string { return cs.name }

// Name returns the command station name.
func (cs *CS) Name() string { return cs.name }

// close closes the command station and the underlying client connection.
func (cs *CS) close() error {
	close(cs.hndCh)
	cs.wg.Wait()
	cs.unsubscribe()
	return cs.client.Close()
}

func (cs *CS) handler(wg *sync.WaitGroup, hndCh <-chan *hndMsg, pubCh chan<- *pubMsg, errCh chan<- *errMsg) {
	wg.Add(1)
	defer wg.Done()

	for msg := range hndCh {
		if value, err := msg.fn(msg.value); err != nil {
			errCh <- &errMsg{topic: msg.topic.String(), err: err}
		} else {
			pubCh <- &pubMsg{topic: msg.topic.noCommand(), value: value}
		}
	}
}

func (cs *CS) subscribe() {
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("cs", cs.name, "enabled", "get"), cs.getEnabled(cs.client))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("cs", cs.name, "enabled", "set"), cs.setEnabled(cs.client))
}

func (cs *CS) unsubscribe() {
	cs.gateway.unsubscribe(cs, joinTopic("cs", cs.name, "enabled", "get"))
	cs.gateway.unsubscribe(cs, joinTopic("cs", cs.name, "enabled", "set"))
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
func (cs *CS) subscribeLocoActions(loco *Loco) {
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "dir", "get"), cs.getLocoDir(cs.client, loco.addr))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "dir", "set"), cs.setLocoDir(cs.client, loco.addr, true))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "dir", "toggle"), cs.toggleLocoDir(cs.client, loco.addr))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "speed", "get"), cs.getLocoSpeed(cs.client, loco.addr))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "speed", "set"), cs.setLocoSpeed(cs.client, loco.addr, true))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "speed", "stop"), cs.stopLoco(cs.client, loco.addr))
	for name, fct := range loco.fcts {
		cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, name, "get"), cs.getLocoFct(cs.client, loco.addr, fct.No))
		cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, name, "set"), cs.setLocoFct(cs.client, loco.addr, fct.No, true))
		cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, name, "toggle"), cs.toggleLocoFct(cs.client, loco.addr, fct.No))
	}
}

// subscribeLocoListeners subscribes to loco listeners for a loco not controlled by this command station.
func (cs *CS) subscribeLocoListeners(loco *Loco) {
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "dir"), cs.setLocoDir(cs.client, loco.addr, false))
	cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, "speed"), cs.setLocoSpeed(cs.client, loco.addr, false))
	for name, fct := range loco.fcts {
		cs.gateway.subscribe(cs.hndCh, cs, joinTopic("loco", loco.name, name), cs.setLocoFct(cs.client, loco.addr, fct.No, false))
	}
}

// AddLoco adds a loco to the command station.
func (cs *CS) AddLoco(loco *Loco) error {
	loco.mu.Lock()
	defer loco.mu.Unlock()

	primary := cs.primary.includes(loco.name)
	secondary := false
	if !primary {
		secondary = cs.secondary.includes(loco.name)
	}

	switch {
	case primary && loco.primary != "":
		return fmt.Errorf("loco %s already added as primary to command station %s", loco.name, cs.name)
	case primary && loco.primary == cs.name:
		return fmt.Errorf("loco %s already added as primary to command station %s", loco.name, cs.name)
	case primary:
		cs.subscribeLocoActions(loco)
		loco.primary = cs.name
		return nil

	case secondary && loco.isSecondary(cs.name):
		return fmt.Errorf("loco %s already added as secondary to command station %s", loco.name, cs.name)
	case secondary:
		cs.subscribeLocoListeners(loco)
		loco.addSecondary(cs.name)
		return nil

	default:
		return nil // whether primary nor secondary
	}
}
