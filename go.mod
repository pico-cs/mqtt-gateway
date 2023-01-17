module github.com/pico-cs/mqtt-gateway

go 1.19

// replace github.com/pico-cs/go-client => ../go-client
// replace github.com/pico-cs/go-client/client => ../go-client/client

require (
	github.com/eclipse/paho.mqtt.golang v1.4.2
	github.com/pico-cs/go-client v0.4.3
	golang.org/x/exp v0.0.0-20230116083435-1de6713980de
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	go.bug.st/serial v1.5.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
)
