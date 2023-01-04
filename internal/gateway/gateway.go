// Package gateway provides a pico-cs MQTT broker gateway.
package gateway

// use paho mqtt 3.1 broker instead the mqtt 5 version github.com/eclipse/paho.golang/paho
// because couldn't get the retain message handling work properly which is an essential part
// of this gateway

import (
	"encoding/json"
	"fmt"
	"sync"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/pico-cs/mqtt-gateway/internal/logger"
)

// DefChanSize defines the default channel size.
const DefChanSize = 100

// HndFn represents a handler function.
type HndFn func(payload any) (any, error)

// HndMsg represents a message provided to a registered handler.
type HndMsg struct {
	TopicStrs []string
	Fn        HndFn
	Value     any
}

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

type subscription struct {
	owner any
	fn    HndFn
	hndCh chan<- *HndMsg
}

const classError = "error"

// Gateway represents a MQTT broker gateway.
type Gateway struct {
	lg     logger.Logger
	config *Config
	client MQTT.Client

	mu            sync.RWMutex
	listening     bool
	subscriptions map[string][]subscription

	subTopic   string
	errorTopic string

	pubCh chan *pubMsg
	errCh chan *errMsg
	wg    *sync.WaitGroup
}

// New returns a new gateway instance.
func New(lg logger.Logger, config *Config) (*Gateway, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	if lg == nil {
		lg = logger.Null
	}

	gw := &Gateway{
		lg:            lg,
		config:        config,
		subscriptions: make(map[string][]subscription),
		subTopic:      topicJoinStr(config.TopicRoot, multiLevel),
		errorTopic:    topicJoinStr(config.TopicRoot, classError),
		pubCh:         make(chan *pubMsg, DefChanSize),
		errCh:         make(chan *errMsg, DefChanSize),
		wg:            new(sync.WaitGroup),
	}

	// MQTT:
	// starting with a clean seesion without client id as receiving
	// retained messages should be enough initializing the
	// command stations
	opts := MQTT.NewClientOptions()
	opts.AddBroker(config.addr())
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

	lg.Printf("connect to broker %s", config.addr())

	// start go routines
	go gw.publish(gw.wg, gw.pubCh, gw.errCh)
	go gw.publishError(gw.wg, gw.errCh)

	return gw, nil
}

// topicRoot returns the topic root.
func (gw *Gateway) topicRoot() string { return gw.config.TopicRoot }

const (
	defaultQoS = 1
	wait       = 250 // waiting time for client disconnect in ms
)

// Close closes the gateway.
func (gw *Gateway) Close() error {
	// shutdown
	gw.lg.Println("shutdown gateway...")
	close(gw.pubCh)
	close(gw.errCh)
	gw.wg.Wait()
	gw.lg.Printf("disconnect from broker %s", gw.config.addr())
	gw.unsubscribeBroker() // ignore error
	gw.client.Disconnect(wait)
	return nil
}

// PublishErr publishes a error message
func (gw *Gateway) PublishErr(topicStrs []string, retain bool, err error) {
	topicRootStr := topicJoin(append([]string{gw.topicRoot()}, topicStrs...))
	gw.errCh <- &errMsg{topic: topicRootStr, retain: retain, err: err}
}

// Publish publishes a message.
func (gw *Gateway) Publish(topicStrs []string, retain bool, value any) {
	topicRootStr := topicJoin(append([]string{gw.topicRoot()}, topicStrs...))
	gw.pubCh <- &pubMsg{topic: topicRootStr, retain: retain, value: value}
}

// Listen starts the gateway listening to the mqtt broker.
func (gw *Gateway) Listen() error {
	// separated to start listen after subscriptions not to miss retained messages
	gw.mu.Lock()
	defer gw.mu.Unlock()
	if gw.listening {
		return fmt.Errorf("gateway is already listening")
	}
	gw.listening = true
	// subscribe
	return gw.subscribeBroker()
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

// Subscribe subscribes a message handler.
func (gw *Gateway) Subscribe(hndCh chan<- *HndMsg, owner any, topicStrs []string, fn HndFn) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	topicStr := topicJoin(topicStrs)
	gw.subscriptions[topicStr] = append(gw.subscriptions[topicStr], subscription{owner: owner, fn: fn, hndCh: hndCh})
}

// Unsubscribe unsubscribes a message handler.
func (gw *Gateway) Unsubscribe(owner any, topicStrs []string) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	topicStr := topicJoin(topicStrs)
	l := len(gw.subscriptions[topicStr])
	for i, subscription := range gw.subscriptions[topicStr] {
		if subscription.owner == owner {
			gw.subscriptions[topicStr][i] = gw.subscriptions[topicStr][l-1]
			gw.subscriptions[topicStr] = gw.subscriptions[topicStr][:l-1]
			break
		}
	}
}

func (gw *Gateway) handler(client MQTT.Client, msg MQTT.Message) {
	topicStrs := topicSplit(msg.Topic())

	var value any
	if err := json.Unmarshal(msg.Payload(), &value); err != nil {
		gw.errCh <- &errMsg{topic: msg.Topic(), err: err}
		return
	}

	gw.lg.Printf("receive topic %s retained %t value %v\n", msg.Topic(), msg.Retained(), value)

	gw.mu.RLock()
	defer gw.mu.RUnlock()

	topicNoRootStr := topicJoin(topicStrs[1:]) // no root
	subscriptions, ok := gw.subscriptions[topicNoRootStr]
	if !ok {
		return // nothing to do
	}

	for _, subscription := range subscriptions {
		subscription.hndCh <- &HndMsg{TopicStrs: topicStrs[1:], Fn: subscription.fn, Value: value}
	}
}

func (gw *Gateway) publish(wg *sync.WaitGroup, pubCh <-chan *pubMsg, errCh chan<- *errMsg) {
	wg.Add(1)
	defer wg.Done()

	for msg := range pubCh {
		if msg.value == nil {
			continue // nothing to publish
		}

		gw.lg.Printf("publish topic %s retain %t value %v\n", msg.topic, msg.retain, msg.value)

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

		gw.lg.Printf("publish topic %s retain %t error %s\n", msg.topic, msg.retain, msg.err)

		payload, err := json.Marshal(&errPayload{Topic: msg.topic, Error: msg.err.Error()})
		if err != nil {
			// hm, we can only log...
			gw.lg.Printf("publish error topic %s err %s", msg.topic, err)
		}

		token := gw.client.Publish(gw.errorTopic, defaultQoS, msg.retain, payload)
		if token.Wait() && token.Error() != nil {
			// hm, we can only log...
			gw.lg.Printf("publish error topic %s err %s", msg.topic, token.Error())
		}
	}
}
