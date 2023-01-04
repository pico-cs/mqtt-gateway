package devices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/pico-cs/go-client/client"
	"github.com/pico-cs/mqtt-gateway/internal/gateway"
	"github.com/pico-cs/mqtt-gateway/internal/logger"
	"golang.org/x/exp/maps"
)

// CSSet represents a set of command stations.
type CSSet struct {
	lg    logger.Logger
	gw    *gateway.Gateway
	csMap map[string]*CS
}

// NewCSSet creates new command station set instance.
func NewCSSet(lg logger.Logger, gw *gateway.Gateway) *CSSet {
	if lg == nil {
		lg = logger.Null
	}
	return &CSSet{lg: lg, gw: gw, csMap: make(map[string]*CS)}
}

// Items returns a command station map.
func (s *CSSet) Items() map[string]*CS { return maps.Clone(s.csMap) }

// Add adds a command station via a command station configuration.
func (s *CSSet) Add(config *CSConfig) (*CS, error) {
	cs, err := newCS(s.lg, config, s.gw)
	if err != nil {
		return nil, err
	}
	s.csMap[config.Name] = cs
	return cs, nil
}

// Close closes all command stations.
func (s *CSSet) Close() error {
	var lastErr error
	for _, cs := range s.csMap {
		if err := cs.close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// HandleFunc returns a http.HandleFunc handler.
func (s *CSSet) HandleFunc(addr string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		type tpldata struct {
			Title string
			Items map[string]*url.URL
		}

		data := &tpldata{Items: map[string]*url.URL{}}

		data.Title = "command stations"
		for name := range s.csMap {
			data.Items[name] = &url.URL{Scheme: "http", Host: addr, Path: fmt.Sprintf("/cs/%s", name)}
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err := idxTpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}
}

// A CS represents a command station.
type CS struct {
	lg        logger.Logger
	config    *CSConfig
	gw        *gateway.Gateway
	primary   *filter
	secondary *filter
	hndCh     chan *gateway.HndMsg
	wg        *sync.WaitGroup
	client    *client.Client
	locos     map[string]*Loco
}

// newCS returns a new command station instance.
func newCS(lg logger.Logger, config *CSConfig, gw *gateway.Gateway) (*CS, error) {
	if err := config.validate(); err != nil {
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
		lg:        lg,
		config:    config,
		gw:        gw,
		primary:   primary,
		secondary: secondary,
		hndCh:     make(chan *gateway.HndMsg, gateway.DefChanSize),
		wg:        new(sync.WaitGroup),
		locos:     map[string]*Loco{},
	}

	// open
	cs.lg.Printf("open command station %s", cs.name())
	conn, err := cs.config.conn()
	if err != nil {
		return nil, err
	}
	cs.client = client.New(conn, cs.pushHandler(gw))

	// start go routines
	go cs.cmdHandler(cs.wg, cs.hndCh, gw)

	cs.subscribe()

	return cs, nil
}

func (cs *CS) name() string { return cs.config.Name }

// close closes the command station and the underlying client connection.
func (cs *CS) close() error {
	cs.lg.Printf("close command station %s", cs.name())
	for _, loco := range cs.locos {
		if loco.isPrimary(cs) {
			loco.unsetPrimary(cs) // ignore error
			cs.unsubscribeLocoActions(loco)
		} else {
			loco.delSecondary(cs) // ignore error
			cs.unsubscribeLocoEvents(loco)
		}
	}
	cs.unsubscribe()
	return cs.client.Close()
}

// AddLoco adds a loco to the command station.
func (cs *CS) AddLoco(loco *Loco) (bool, error) {
	csName := cs.name()
	locoName := loco.name()

	if _, ok := cs.locos[locoName]; ok {
		return false, fmt.Errorf("loco %s already assigned to command station %s", locoName, csName)
	}

	if cs.primary.includes(locoName) {
		if err := loco.setPrimary(cs); err != nil {
			return false, err
		}
		cs.locos[locoName] = loco
		cs.lg.Printf("subscribe loco %s to command station %s as primary", locoName, csName)
		cs.subscribeLocoActions(loco)
		return true, nil
	}

	if cs.secondary.includes(locoName) {
		if err := loco.addSecondary(cs); err != nil {
			return false, err
		}
		cs.locos[locoName] = loco
		cs.lg.Printf("subscribe loco %s to command station %s as secondary", locoName, csName)
		cs.subscribeLocoEvents(loco)
		return true, nil
	}
	return false, nil
}

// ServeHTTP implements the http.Handler interface.
func (cs *CS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	b, err := json.MarshalIndent(cs.config, "", indent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Write(b)
}

func (cs *CS) pushHandler(gw *gateway.Gateway) func(msg client.Msg, err error) {
	return func(msg client.Msg, err error) {
		if err != nil {
			gw.PublishErr(nil, false, err)
			return
		}

		switch msg := msg.(type) {

		case *client.IOIEMsg:
			// TODO: improve performance in not looping over all the IOs
			for name, io := range cs.config.IOs {
				if io.GPIO == msg.GPIO {
					gw.Publish([]string{"cs", cs.name(), name}, true, msg.State)
				}
			}
		}
	}
}

// cmdHandler handles commands.
func (cs *CS) cmdHandler(wg *sync.WaitGroup, hndCh <-chan *gateway.HndMsg, gw *gateway.Gateway) {
	wg.Add(1)
	defer wg.Done()

	for msg := range hndCh {

		value, err := msg.Fn(msg.Value)
		if err != nil {
			gw.PublishErr(msg.TopicStrs, false, err)
			continue
		}

		// send event
		gw.Publish(msg.TopicStrs[:len(msg.TopicStrs)-1], true, value)
	}
}

func (cs *CS) subscribe() {
	cs.gw.Subscribe(cs.hndCh, cs, []string{"cs", cs.config.Name, "temp", "get"}, cs.getTemp(cs.client))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"cs", cs.config.Name, "mte", "get"}, cs.getMTE(cs.client))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"cs", cs.config.Name, "mte", "set"}, cs.setMTE(cs.client))
}

func (cs *CS) unsubscribe() {
	cs.gw.Unsubscribe(cs, []string{"cs", cs.config.Name, "tmp", "get"})
	cs.gw.Unsubscribe(cs, []string{"cs", cs.config.Name, "mte", "get"})
	cs.gw.Unsubscribe(cs, []string{"cs", cs.config.Name, "mte", "set"})
}

// subscribeLocoActions subscribes to loco actions for a loco controlled by this command station.
func (cs *CS) subscribeLocoActions(loco *Loco) {
	name := loco.name()
	addr := loco.addr()

	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "dir", "get"}, cs.getLocoDir(cs.client, addr))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "dir", "set"}, cs.setLocoDir(cs.client, addr, true))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "dir", "toggle"}, cs.toggleLocoDir(cs.client, addr))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "speed", "get"}, cs.getLocoSpeed(cs.client, addr))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "speed", "set"}, cs.setLocoSpeed(cs.client, addr, true))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "speed", "stop"}, cs.stopLoco(cs.client, addr))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "speed", "add"}, cs.addLocoSpeed(cs.client, addr))
	loco.iterFcts(func(fctName string, fctNo uint) {
		cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, fctName, "get"}, cs.getLocoFct(cs.client, addr, fctNo))
		cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, fctName, "set"}, cs.setLocoFct(cs.client, addr, fctNo, true))
		cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, fctName, "toggle"}, cs.toggleLocoFct(cs.client, addr, fctNo))
	})
}

// subscribeLocoEvents subscribes to loco events for a loco not controlled by this command station.
func (cs *CS) subscribeLocoEvents(loco *Loco) {
	name := loco.name()
	addr := loco.addr()

	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "dir"}, cs.setLocoDir(cs.client, addr, false))
	cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, "speed"}, cs.setLocoSpeed(cs.client, addr, false))
	loco.iterFcts(func(fctName string, fctNo uint) {
		cs.gw.Subscribe(cs.hndCh, cs, []string{"loco", name, fctName}, cs.setLocoFct(cs.client, addr, fctNo, false))
	})
}

// unsubscribeLocoActions unsubscribes from loco actions.
func (cs *CS) unsubscribeLocoActions(loco *Loco) {
	name := loco.name()

	cs.gw.Unsubscribe(cs, []string{"loco", name, "dir", "get"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "dir", "set"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "dir", "toggle"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "speed", "get"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "speed", "set"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "speed", "stop"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "speed", "add"})
	loco.iterFcts(func(fctName string, fctNo uint) {
		cs.gw.Unsubscribe(cs, []string{"loco", name, fctName, "get"})
		cs.gw.Unsubscribe(cs, []string{"loco", name, fctName, "set"})
		cs.gw.Unsubscribe(cs, []string{"loco", name, fctName, "toggle"})
	})
}

// unsubscribeLocoEvents unsubscribes from loco events.
func (cs *CS) unsubscribeLocoEvents(loco *Loco) {
	name := loco.name()

	cs.gw.Unsubscribe(cs, []string{"loco", name, "dir"})
	cs.gw.Unsubscribe(cs, []string{"loco", name, "speed"})
	loco.iterFcts(func(fctName string, fctNo uint) {
		cs.gw.Unsubscribe(cs, []string{"loco", name, fctName})
	})
}

func (cs *CS) getTemp(client *client.Client) gateway.HndFn {
	return func(payload any) (any, error) {
		return client.Temp()
	}
}

func (cs *CS) getMTE(client *client.Client) gateway.HndFn {
	return func(payload any) (any, error) {
		return client.MTE()
	}
}

func (cs *CS) setMTE(client *client.Client) gateway.HndFn {
	return func(payload any) (any, error) {
		enabled, ok := payload.(bool)
		if !ok {
			return nil, fmt.Errorf("setMTE: invalid enabled type %[1]T value %[1]v", payload)
		}
		return client.SetMTE(enabled)
	}
}

func (cs *CS) getLocoDir(client *client.Client, addr uint) gateway.HndFn {
	return func(payload any) (any, error) {
		return client.LocoDir(addr)
	}
}

func (cs *CS) setLocoDir(client *client.Client, addr uint, publish bool) gateway.HndFn {
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

func (cs *CS) toggleLocoDir(client *client.Client, addr uint) gateway.HndFn {
	return func(payload any) (any, error) {
		return client.ToggleLocoDir(addr)
	}
}

type speed127 uint
type speed128 uint

func (s speed127) speed128() speed128 {
	if s == 0 {
		return 0
	}
	return speed128(uint(s) + 1) // skip emergency stop
}

func (s speed127) add(delta int) speed127 {
	if delta == 0 {
		return s
	}
	speed := int(s) + delta
	switch {
	case speed < 0:
		return 0
	case speed > 126:
		return 126
	default:
		return speed127(speed)
	}
}

func (s speed128) speed127() speed127 {
	if s <= 1 {
		return 0 // skip emergency stop
	}
	return speed127(uint(s) - 1)
}

func (s speed128) add(delta int) speed128 {
	return s.speed127().add(delta).speed128()
}

func (cs *CS) getLocoSpeed(client *client.Client, addr uint) gateway.HndFn {
	return func(payload any) (any, error) {
		speed, err := client.LocoSpeed128(addr)
		if err != nil {
			return nil, err
		}
		return speed128(speed).speed127(), nil
	}
}

func (cs *CS) setLocoSpeed(client *client.Client, addr uint, publish bool) gateway.HndFn {
	return func(payload any) (any, error) {
		f64, ok := payload.(float64)
		if !ok {
			return nil, fmt.Errorf("setLocoSpeed: invalid speed type %T", payload)
		}
		speed, err := client.SetLocoSpeed128(addr, uint(speed127(f64).speed128()))
		if err != nil {
			return nil, err
		}
		if !publish {
			return nil, err
		}
		return speed128(speed).speed127(), err
	}
}

func (cs *CS) stopLoco(client *client.Client, addr uint) gateway.HndFn {
	return func(payload any) (any, error) {
		speed, err := client.SetLocoSpeed128(addr, 1) // emergency stop
		if err != nil {
			return nil, err
		}
		return speed128(speed).speed127(), nil // speed should be 0
	}
}

func (cs *CS) addLocoSpeed(client *client.Client, addr uint) gateway.HndFn {
	return func(payload any) (any, error) {
		f64, ok := payload.(float64)
		if !ok {
			return nil, fmt.Errorf("addLocoSpeed: invalid delta type %T", payload)
		}
		speed, err := client.LocoSpeed128(addr)
		if err != nil {
			return nil, err
		}
		speed, err = client.SetLocoSpeed128(addr, uint(speed128(speed).add(int(f64))))
		if err != nil {
			return nil, err
		}
		return speed128(speed).speed127(), nil
	}
}

func (cs *CS) getLocoFct(client *client.Client, addr, no uint) gateway.HndFn {
	return func(payload any) (any, error) {
		return client.LocoFct(addr, no)
	}
}

func (cs *CS) setLocoFct(client *client.Client, addr, no uint, publish bool) gateway.HndFn {
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

func (cs *CS) toggleLocoFct(client *client.Client, addr, no uint) gateway.HndFn {
	return func(payload any) (any, error) {
		return client.ToggleLocoFct(addr, no)
	}
}
