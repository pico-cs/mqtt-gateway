// Package gateway provides a pico-cs MQTT broker gateway.
package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

type hndFn func(payload any) (any, error)

type subscription struct {
	owner any
	fn    hndFn
}

// Gateway represents a MQTT broker gateway.
type Gateway struct {
	config            *Config
	connectionManager *autopaho.ConnectionManager

	mu            sync.RWMutex
	subscriptions map[string][]subscription

	subTopic   string
	errorTopic string

	logger *log.Logger
}

// New returns a new gateway instance.
func New(config *Config) (*Gateway, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	gw := &Gateway{
		config:        config,
		subscriptions: make(map[string][]subscription),
		subTopic:      joinTopic(config.TopicRoot, multiLevel),
		errorTopic:    joinTopic(config.TopicRoot, classError),
		logger:        log.New(os.Stderr, "", log.LstdFlags),
	}

	pahoConfig := autopaho.ClientConfig{
		BrokerUrls:     []*url.URL{{Scheme: "tcp", Host: config.address()}},
		OnConnectError: func(err error) { log.Println(err) },
		ClientConfig: paho.ClientConfig{
			Router: paho.NewSingleHandlerRouter(gw.handler),
		},
	}

	pahoConfig.SetUsernamePassword(config.Username, []byte(config.Password))

	connectionManager, err := autopaho.NewConnection(context.Background(), pahoConfig)
	//cancel()
	if err != nil {
		return nil, err
	}

	gw.connectionManager = connectionManager

	// don't wait forever in case of connection issues like invalid host or port.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := connectionManager.AwaitConnection(ctx); err != nil {
		return nil, err
	}

	if err := gw.subscribeBroker(); err != nil {
		return nil, err
	}
	gw.logger.Printf("connect to broker %s", config.address())
	return gw, nil
}

// Close closes the gateway and the MQTT connection.
func (gw *Gateway) Close() error {
	gw.logger.Printf("disconnect from broker %s", gw.config.address())
	gw.unsubscribeBroker() // ignore error
	return gw.connectionManager.Disconnect(context.Background())
}

func (gw *Gateway) subscribeBroker() error {
	sub := &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			gw.subTopic: {QoS: 1}, //QoS 1: at least once
		},
	}
	if suback, err := gw.connectionManager.Subscribe(context.Background(), sub); err != nil {
		gw.logger.Printf("subscribe suback %v error %s", suback, err)
		return err
	}
	return nil
}

func (gw *Gateway) unsubscribeBroker() error {
	unsub := &paho.Unsubscribe{
		Topics: []string{gw.subTopic},
	}
	if unsuback, err := gw.connectionManager.Unsubscribe(context.Background(), unsub); err != nil {
		gw.logger.Printf("unsubscribe unsuback %v error %s", unsuback, err)
		return err
	}
	return nil
}

func (gw *Gateway) subscribe(owner any, topic string, fn hndFn) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.subscriptions[topic] = append(gw.subscriptions[topic], subscription{owner: owner, fn: fn})
}

func (gw *Gateway) unsubscribe(owner any, topic string) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	l := len(gw.subscriptions[topic])
	for i, subscription := range gw.subscriptions[topic] {
		if subscription.owner == owner {
			gw.subscriptions[topic][i] = gw.subscriptions[topic][l-1]
			gw.subscriptions[topic] = gw.subscriptions[topic][:l-1]
			break
		}
	}
}

func (gw *Gateway) handler(p *paho.Publish) {
	topic, err := parseTopic(p.Topic)
	if err != nil {
		gw.publishError(p.Topic, err)
		return
	}

	gw.mu.RLock()
	defer gw.mu.RUnlock()

	subscriptions, ok := gw.subscriptions[topic.noRoot()]
	if !ok {
		return // nothing to do
	}

	var payload any
	if err := json.Unmarshal(p.Payload, &payload); err != nil {
		gw.publishError(p.Topic, err)
		return
	}
	// log.Printf("unmarshall payload %[1]v %[1]s value %[2]T %[2]v\n", p.Payload, payload)

	for _, subscription := range subscriptions {
		if reply, err := subscription.fn(payload); err != nil {
			gw.publishError(p.Topic, err)
		} else {
			gw.publish(topic.noCommand(), reply)
		}
	}
}

func (gw *Gateway) publish(topic string, reply any) {
	if reply == nil {
		return // nothing to publish
	}

	gw.logger.Printf("publish: topic %s value %v", topic, reply)

	payload, err := json.Marshal(reply)
	if err != nil {
		gw.publishError(topic, err)
		return
	}

	publish := &paho.Publish{
		QoS:     1,    // QoS == 1
		Retain:  true, // retain msg, so that new joiners will get the latest message
		Topic:   topic,
		Payload: payload,
	}

	if _, err := gw.connectionManager.Publish(context.Background(), publish); err != nil {
		gw.publishError(topic, err)
	}
}

type errorMsg struct {
	Topic string `json:"topic"`
	Error string `json:"error"`
}

func (gw *Gateway) publishError(topic string, err error) {
	payload, err := json.Marshal(&errorMsg{Topic: topic, Error: err.Error()})
	if err != nil {
		panic(err) // should never happen
	}

	gw.logger.Printf("publish error: %v", payload)

	publish := &paho.Publish{
		QoS:     1, // QoS == 1
		Retain:  false,
		Topic:   gw.errorTopic,
		Payload: payload,
	}

	if _, err := gw.connectionManager.Publish(context.Background(), publish); err != nil {
		// hm, we can only log...
		gw.logger.Printf("publish error: topic %s text %s", topic, err)
	}
}
