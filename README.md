# pico-cs mqtt-gateway
[![Go Reference](https://pkg.go.dev/badge/github.com/pico-cs/mqtt-gateway/gateway.svg)](https://pkg.go.dev/github.com/pico-cs/mqtt-gateway/gateway)
[![Go Report Card](https://goreportcard.com/badge/github.com/pico-cs/mqtt-gateway)](https://goreportcard.com/report/github.com/pico-cs/mqtt-gateway)
[![REUSE status](https://api.reuse.software/badge/github.com/pico-cs/mqtt-gateway)](https://api.reuse.software/info/github.com/pico-cs/mqtt-gateway)
![](https://github.com/pico-cs/mqtt-gateway/workflows/build/badge.svg)

mqtt-gateway connects pico-cs command stations to a MQTT broker.

## Why care about a MQTT gateway

### The short answer
A model railroad can be seen as a set of [IoT](https://en.wikipedia.org/wiki/Internet_of_things) devices and [MQTT](https://mqtt.org/) is one of the standard protocols in this space.

### IoT integration
While the [pico-cs](https://github.com/pico-cs) [firmware](https://github.com/pico-cs/firmware) can be simply installed on a [Raspberry Pi Pico](https://www.raspberrypi.com/products/raspberry-pi-pico/) and used as a DCC command station whether by controlling it over a serial monitor or via serial over USB or WiFi with the help of a client library (e.g. [Go client](https://github.com/pico-cs/go-client)). But using an IoT standard protocol like MQTT provides unprecedented opportunities in integrating pico-cs command stations into an IoT (software) infrastructure. And just to name two well known examples with MQTT support:
- [Node-RED](https://nodered.org/)
- [Home Assistant](https://www.home-assistant.io/) which comes with a [MQTT broker option](https://www.home-assistant.io/integrations/mqtt/)

## Why a dedicated gateway component...
...and not having an IoT protocol integrated as part of the pico firmware.

The resources on a micro controller are limited and following the [KISS priciple](https://en.wikipedia.org/wiki/KISS_principle) each component should be simple and providing exactly the tasks it is designed for:
- the pico-cs firmware implements a simple command text protocol used for the serial as well as for the WiFi TCP/IP communication
- client libraries provide an idiomatic way in their respective programming language to communicate with the command station
- and finally this gateway provides MQTT integration (like future components might provide integrations to additional protocols)

## Build

For building there is two options available:

- [local build](#local): install Go environment and build on your local machine
- [docker build](https://github.com/pico-cs/docker-buld): no toolchain installation but a running docker environment on your local machine is required

### Local
To build go-hdb you need to have
- a working Go environment of the [latest Go version](https://golang.org/dl/) and
- [git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git) installed 

```
git clone https://github.com/pico-cs/mqtt-gateway.git
cd mqtt-gateway/cmd/gateway
go build
```
Beside building the gateway executable for the local operating system and hardware architecture Go supports 'cross compiling' for many target OS and hardware architecture combinations (for details please consult the excellent [Go documention](https://go.dev/doc/)).

Example building executable for Raspberry Pi on Raspberry Pi OS
```
GOOS=linux GOARCH=arm GOARM=7 go build
```

## Run the gateway

Preconditions:
- a running MQTT broker like [Mosquitto](https://mosquitto.org/)
- the gateway executable
- command station and model locomotive configuration files

### Gateway

Print help (parameters):

```
./gateway -h

Usage of ./gateway:
  -configDir string
    	configuration directory
  -host string
    	MQTT host (default "localhost")
  -password string
    	password
  -port string
    	MQTT port (default "1883")
  -topicRoot string
    	topic root (default "pico-cs")
  -username string
    	user name
```

...todo

### Configuration files
To configure the gateway's command station and loco parameters [YAML files](https://yaml.org/) are used. The entire configuration can be stored in one file or in multiple files. During the start of the gateway the configuration directory (parameter configDir) and it's subdirectories are scanned for valid configuration files with file extension '.yaml' or '.yml'. The directory tree scan is a depth-first search and within a directory the files are visited in a lexical order. If a configuration for an object is found more than once the last one wins.

Each object needs to define a name. As the object name is part of the [MQTT topic](#mqtt-topics) it must fullfil the following conditions:
- consist of valid MQTT topic characters and
- must not contain characters "/", "+" or "#"

All conditions are checked by the gateway on start and in case of a violation the program prints the respective error message and stops execution.

### Embedded configuration files
Beside using a configuration directory the configuration files can be embedded in the gateway executable:
- store them in as part of the source code directory at mqtt-gateway/cmd/gateway/config and
- build the binary 

This is the prefered method using a static or default configuration. During the gateway start the embedded configuration files are scanned before the 'external' configuration files (configDir parameter), so an external object configuration would overwrite an embedded one.

### Configuration examples
...todo

## MQTT topics
In MQTT a topic is a string the MQTT broker uses to determine which messages should be send to each of the connected clients. The client uses a topic to publish a message and uses topics to subscribe to messages it would like to receive from the broker.
A topic can consist of more than one level - the character used to separate levels is '/'. A client might use wildcards when subscribing to topics, where '+' is the wildcard character for a dedicated level and '#' is the multi level wildcard character which can only be used as the last level of a topic. For further details about MQTT topics please see the [MQTT specification](https://mqtt.org/mqtt-specification/).

The topic schema used by the gateway:

```
"<topic root>/<object type>/<object name>/<property>[/<command>]"
```

The message payload is whether a json encoded atomic field (aka string, number, boolean) or a json encoded object.

Please see [mqtt](mqtt.md) for information about the topics and message payloads used by the gateway.

## Licensing

Copyright 2021-2022 Stefan Miller and pico-cs contributers. Please see our [LICENSE](LICENSE.md) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/pico-cs/mqtt-gateway).
