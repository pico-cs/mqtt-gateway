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

const defChanSize = 100

type hndFn func(payload any) (any, error)

type pubMsg struct {
	topic string
	value any
}

type errMsg struct {
	topic string
	err   error
}

type hndMsg struct {
	topic topic
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
	config            *Config
	connectionManager *autopaho.ConnectionManager

	mu            sync.RWMutex
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

	gw := &Gateway{
		config:        config,
		subscriptions: make(map[string][]subscription),
		subTopic:      joinTopic(config.TopicRoot, multiLevel),
		errorTopic:    joinTopic(config.TopicRoot, classError),
		logger:        log.New(os.Stderr, "", log.LstdFlags),
		pubCh:         make(chan *pubMsg, defChanSize),
		errCh:         make(chan *errMsg, defChanSize),
		wg:            new(sync.WaitGroup),
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
	gw.logger.Printf("connected to broker %s", config.address())

	// start go routines
	go gw.publish(gw.wg, gw.pubCh, gw.errCh)
	go gw.publishError(gw.wg, gw.errCh)

	return gw, nil
}

// Close closes the gateway and the MQTT connection.
func (gw *Gateway) Close() error {
	gw.logger.Println("shutdown gateway...")
	close(gw.pubCh)
	close(gw.errCh)
	gw.wg.Wait()
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

func (gw *Gateway) subscribe(hndCh chan<- *hndMsg, owner any, topic string, fn hndFn) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.subscriptions[topic] = append(gw.subscriptions[topic], subscription{owner: owner, fn: fn, hndCh: hndCh})
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
		gw.errCh <- &errMsg{topic: p.Topic, err: err}
		return
	}

	gw.mu.RLock()
	defer gw.mu.RUnlock()

	subscriptions, ok := gw.subscriptions[topic.noRoot()]
	if !ok {
		return // nothing to do
	}

	var value any
	if err := json.Unmarshal(p.Payload, &value); err != nil {
		gw.errCh <- &errMsg{topic: p.Topic, err: err}
		return
	}

	// log.Printf("unmarshall payload %[1]v %[1]s value %[2]T %[2]v\n", p.Payload, payload)

	for _, subscription := range subscriptions {
		subscription.hndCh <- &hndMsg{topic: topic, fn: subscription.fn, value: value}
	}
}

func (gw *Gateway) publish(wg *sync.WaitGroup, pubCh <-chan *pubMsg, errCh chan<- *errMsg) {
	wg.Add(1)
	defer wg.Done()

	for msg := range pubCh {
		if msg.value == nil {
			continue // nothing to publish
		}

		gw.logger.Printf("publish: topic %s value %v", msg.topic, msg.value)

		payload, err := json.Marshal(msg.value)
		if err != nil {
			errCh <- &errMsg{topic: msg.topic, err: err}
			continue
		}

		publish := &paho.Publish{
			QoS:     1,    // QoS == 1
			Retain:  true, // retain msg, so that new joiners will get the latest message
			Topic:   msg.topic,
			Payload: payload,
		}

		if _, err := gw.connectionManager.Publish(context.Background(), publish); err != nil {
			errCh <- &errMsg{topic: msg.topic, err: err}
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

		gw.logger.Printf("publish error: %s", msg.err)

		payload, err := json.Marshal(&errPayload{Topic: msg.topic, Error: msg.err.Error()})
		if err != nil {
			// hm, we can only log...
			gw.logger.Printf("publish error: topic %s err %s", msg.topic, err)
		}

		publish := &paho.Publish{
			QoS:     1, // QoS == 1
			Retain:  false,
			Topic:   gw.errorTopic,
			Payload: payload,
		}

		if _, err := gw.connectionManager.Publish(context.Background(), publish); err != nil {
			// hm, we can only log...
			gw.logger.Printf("publish error: topic %s error %s", msg.topic, err)
		}
	}
}
