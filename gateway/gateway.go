// Package gateway provides a pico-cs MQTT broker gateway.
package gateway

// use paho mqtt 3.1 broker instead the mqtt 5 version github.com/eclipse/paho.golang/paho
// because couldn't get the retain message handling work properly which is an essential part
// of this gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/exp/maps"
)

const defChanSize = 100

type hndFn func(payload any) (any, error)

type pubMsg struct {
	topic  string
	retain bool
	value  any
}

type errMsg struct {
	topic  string
	retain bool
	err    error
}

type hndMsg struct {
	topic string
	fn    hndFn
	value any
}

type subscription struct {
	owner any
	fn    hndFn
	hndCh chan<- *hndMsg
}

// Gateway represents a MQTT broker gateway.
type Gateway struct {
	config *Config
	client MQTT.Client

	listening bool

	mu      sync.RWMutex
	csMap   map[string]*CS
	locoMap map[string]*Loco

	subMu         sync.RWMutex
	subscriptions map[string][]subscription

	subTopic   string
	errorTopic string

	logger *log.Logger

	pubCh chan *pubMsg
	errCh chan *errMsg
	wg    *sync.WaitGroup
}

// New returns a new gateway instance.
func New(config *Config) (*Gateway, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	logger := config.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0) // dev/null
	}

	gw := &Gateway{
		config:        config,
		csMap:         make(map[string]*CS),
		locoMap:       make(map[string]*Loco),
		subscriptions: make(map[string][]subscription),
		subTopic:      joinTopic(config.TopicRoot, multiLevel),
		errorTopic:    joinTopic(config.TopicRoot, classError),
		logger:        logger,
		pubCh:         make(chan *pubMsg, defChanSize),
		errCh:         make(chan *errMsg, defChanSize),
		wg:            new(sync.WaitGroup),
	}

	// MQTT:
	// starting with a clean seesion without client id as receiving
	// retained messages should be enough initializing the
	// command stations
	opts := MQTT.NewClientOptions()
	opts.AddBroker(config.address())
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)
	opts.SetDefaultPublishHandler(gw.handler)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	gw.client = client

	gw.logger.Printf("connected to broker %s", config.address())

	// start go routines
	go gw.publish(gw.wg, gw.pubCh, gw.errCh)
	go gw.publishError(gw.wg, gw.errCh)

	return gw, nil
}

const (
	defaultQoS = 1
	wait       = 250 // waiting time for client disconnect in ms
)

// Close closes the gateway and the MQTT connection.
func (gw *Gateway) Close() error {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	// close command stations
	for name, cs := range gw.csMap {
		if err := cs.close(); err != nil { // ignore error
			gw.logger.Printf("closed command station %s - %s", name, err)
		} else {
			gw.logger.Printf("closed command station %s", name)
		}
	}

	// shutdown
	gw.logger.Println("shutdown gateway...")
	close(gw.pubCh)
	close(gw.errCh)
	gw.wg.Wait()
	gw.logger.Printf("disconnect from broker %s", gw.config.address())
	gw.unsubscribeBroker() // ignore error
	gw.client.Disconnect(wait)
	return nil
}

// Listen starts the gateway listening to subscriptions.
func (gw *Gateway) Listen() error {
	// separated to start listen after subscriptions not to miss retained messages
	gw.mu.Lock()
	defer gw.mu.Unlock()
	if gw.listening {
		return fmt.Errorf("gateway is already listening")
	}
	// subscribe
	return gw.subscribeBroker()
}

// CSList returns the list of command stations assigned to this gateway.
func (gw *Gateway) CSList() []*CS {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return maps.Values(gw.csMap)
}

// LocoList returns the list of locos assigned to this gateway.
func (gw *Gateway) LocoList() []*Loco {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return maps.Values(gw.locoMap)
}

func (gw *Gateway) subscribeBroker() error {
	if token := gw.client.Subscribe(gw.subTopic, defaultQoS, gw.handler); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (gw *Gateway) unsubscribeBroker() error {
	if token := gw.client.Unsubscribe(gw.subTopic); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// AddCS adds a command station to the gateway via a command station configuration.
func (gw *Gateway) AddCS(config *CSConfig) (*CS, error) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	if _, ok := gw.csMap[config.Name]; ok {
		return nil, fmt.Errorf("command station %s does already exist", config.Name)
	}
	cs, err := newCS(config, gw)
	if err != nil {
		return nil, err
	}
	gw.csMap[config.Name] = cs
	return cs, nil
}

// AddLoco adds a loco to the gateway via a loco configuration.
func (gw *Gateway) AddLoco(config *LocoConfig) (*Loco, error) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	if _, ok := gw.locoMap[config.Name]; ok {
		return nil, fmt.Errorf("loco %s does already exist", config.Name)
	}
	loco, err := newLoco(config)
	if err != nil {
		return nil, err
	}
	gw.locoMap[config.Name] = loco
	return loco, nil
}

func (gw *Gateway) subscribe(hndCh chan<- *hndMsg, owner any, topic string, fn hndFn) {
	gw.subMu.Lock()
	defer gw.subMu.Unlock()
	gw.subscriptions[topic] = append(gw.subscriptions[topic], subscription{owner: owner, fn: fn, hndCh: hndCh})
}

func (gw *Gateway) unsubscribe(owner any, topic string) {
	gw.subMu.Lock()
	defer gw.subMu.Unlock()
	l := len(gw.subscriptions[topic])
	for i, subscription := range gw.subscriptions[topic] {
		if subscription.owner == owner {
			gw.subscriptions[topic][i] = gw.subscriptions[topic][l-1]
			gw.subscriptions[topic] = gw.subscriptions[topic][:l-1]
			break
		}
	}
}

func (gw *Gateway) handler(client MQTT.Client, msg MQTT.Message) {
	topic, err := parseTopic(msg.Topic())
	if err != nil {
		gw.errCh <- &errMsg{topic: msg.Topic(), err: err}
		return
	}

	var value any
	if err := json.Unmarshal(msg.Payload(), &value); err != nil {
		gw.errCh <- &errMsg{topic: msg.Topic(), err: err}
		return
	}

	gw.logger.Printf("receive: topic %s retained %t value %v\n", msg.Topic(), msg.Retained(), value)

	gw.subMu.RLock()
	defer gw.subMu.RUnlock()

	subscriptions, ok := gw.subscriptions[topic.noRoot()]
	if !ok {
		return // nothing to do
	}

	for _, subscription := range subscriptions {
		subscription.hndCh <- &hndMsg{topic: msg.Topic(), fn: subscription.fn, value: value}
	}
}

func (gw *Gateway) publish(wg *sync.WaitGroup, pubCh <-chan *pubMsg, errCh chan<- *errMsg) {
	wg.Add(1)
	defer wg.Done()

	for msg := range pubCh {
		if msg.value == nil {
			continue // nothing to publish
		}

		gw.logger.Printf("publish: topic %s retain %t value %v\n", msg.topic, msg.retain, msg.value)

		payload, err := json.Marshal(msg.value)
		if err != nil {
			errCh <- &errMsg{topic: msg.topic, err: err}
			continue
		}

		token := gw.client.Publish(msg.topic, defaultQoS, msg.retain, payload)
		if token.Wait() && token.Error() != nil {
			errCh <- &errMsg{topic: msg.topic, err: token.Error()}
		}
	}
}

type errPayload struct {
	Topic string `json:"topic"`
	Error string `json:"error"`
}

func (gw *Gateway) publishError(wg *sync.WaitGroup, errCh <-chan *errMsg) {
	wg.Add(1)
	defer wg.Done()

	for msg := range errCh {

		gw.logger.Printf("publish: topic %s retain %t error %s\n", msg.topic, msg.retain, msg.err)

		payload, err := json.Marshal(&errPayload{Topic: msg.topic, Error: msg.err.Error()})
		if err != nil {
			// hm, we can only log...
			gw.logger.Printf("publish error: topic %s err %s", msg.topic, err)
		}

		token := gw.client.Publish(gw.errorTopic, defaultQoS, msg.retain, payload)
		if token.Wait() && token.Error() != nil {
			// hm, we can only log...
			gw.logger.Printf("publish error: topic %s err %s", msg.topic, token.Error())
		}
	}
}
