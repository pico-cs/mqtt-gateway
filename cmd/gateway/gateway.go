package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pico-cs/mqtt-gateway/internal/devices"
	"github.com/pico-cs/mqtt-gateway/internal/gateway"
	"github.com/pico-cs/mqtt-gateway/internal/logger"
	"github.com/pico-cs/mqtt-gateway/internal/server"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

//go:embed config/*
var embedFsys embed.FS

const (
	embedConfigDir = "config"
)

const (
	envHTTPHost      = "HTTP-HOST"
	envHTTPPort      = "HTTP-PORT"
	envMQTTTopicRoot = "MQTT-TOPIC-ROOT"
	envMQTTHost      = "MQTT-HOST"
	envMQTTPort      = "MQTT-PORT"
	envMQTTUsername  = "MQTT-USERNAME"
	envMQTTPassword  = "MQTT-PASSWORD"
)

func lookupEnv(name, def string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}
	return def
}

func addStringVarFlag(p *string, name, env, def, usage string) {
	flag.StringVar(p, name, lookupEnv(env, def), fmt.Sprintf("%s (environment variable: %s)", usage, env))
}

var jamlExts = []string{".yaml", ".yml"}

type config struct {
	lg            logger.Logger
	csConfigMap   map[string]*devices.CSConfig
	locoConfigMap map[string]*devices.LocoConfig
}

func newConfig(lg logger.Logger) *config {
	return &config{
		lg:            lg,
		csConfigMap:   map[string]*devices.CSConfig{},
		locoConfigMap: map[string]*devices.LocoConfig{},
	}
}

func (c *config) parseYaml(b []byte) error {
	cd := yaml.NewDecoder(bytes.NewBuffer(b))
	dd := yaml.NewDecoder(bytes.NewBuffer(b))

	for {
		var m map[string]any

		err := cd.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		typ, ok := m["type"]
		if !ok {
			return fmt.Errorf("invalid document %v - type missing", m)
		}
		if _, ok := m["name"]; !ok {
			return fmt.Errorf("invalid document %v - name missing", m)
		}

		switch typ {
		case devices.CtCS:
			var csConfig devices.CSConfig
			if err := dd.Decode(&csConfig); err != nil {
				return err
			}
			c.csConfigMap[csConfig.Name] = &csConfig
		case devices.CtLoco:
			var locoConfig devices.LocoConfig
			if err := dd.Decode(&locoConfig); err != nil {
				return err
			}
			c.locoConfigMap[locoConfig.Name] = &locoConfig
		default:
			return fmt.Errorf("invalid configuration %v", m)
		}
	}
	return nil
}

func (c *config) load(fsys fs.FS, path string) error {
	return fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !slices.Contains(jamlExts, filepath.Ext(d.Name())) {
			c.lg.Printf("...skipped %s", subPath)
			return nil
		}

		b, err := fs.ReadFile(fsys, subPath)
		if err != nil {
			c.lg.Printf("...%s %s", subPath, err)
			return err
		}

		if err := c.parseYaml(b); err != nil {
			c.lg.Printf("...error loading %s: %s", subPath, err)
			return err
		}
		c.lg.Printf("...loaded %s", subPath)
		return nil
	})
}

type deviceSets struct {
	csSet   *devices.CSSet
	locoSet *devices.LocoSet
}

func newDeviceSets(lg logger.Logger, gw *gateway.Gateway) *deviceSets {
	return &deviceSets{
		csSet:   devices.NewCSSet(lg, gw),
		locoSet: devices.NewLocoSet(lg),
	}
}

func (s *deviceSets) close() {
	s.csSet.Close()
	s.locoSet.Close()
}

func (s *deviceSets) register(config *config) error {
	for _, csConfig := range config.csConfigMap {
		if _, err := s.csSet.Add(csConfig); err != nil {
			return err
		}
	}
	for _, locoConfig := range config.locoConfigMap {
		if _, err := s.locoSet.Add(locoConfig); err != nil {
			return err
		}
	}
	for _, cs := range s.csSet.Items() {
		for _, loco := range s.locoSet.Items() {
			if _, err := cs.AddLoco(loco); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *deviceSets) registerHTTP(server *server.Server) {
	server.HandleFunc("/cs", s.csSet.HandleFunc(server.Addr()))
	server.HandleFunc("/loco", s.locoSet.HandleFunc(server.Addr()))
	for name, cs := range s.csSet.Items() {
		server.Handle(fmt.Sprintf("/cs/%s", name), cs)
	}
	for name, loco := range s.locoSet.Items() {
		server.Handle(fmt.Sprintf("/loco/%s", name), loco)
	}
}

func main() {

	var lg = log.New(os.Stderr, "", log.LstdFlags)

	check := func(err error) {
		if err != nil {
			lg.Fatal(err)
		}
	}

	httpConfig := &server.Config{}
	mqttConfig := &gateway.Config{}

	addStringVarFlag(&httpConfig.Host, "httpHost", envHTTPHost, server.DefaultHost, "HTTP host")
	addStringVarFlag(&httpConfig.Port, "httpPort", envHTTPPort, server.DefaultPort, "HTTP port")
	addStringVarFlag(&mqttConfig.TopicRoot, "mqttTopicRoot", envMQTTTopicRoot, gateway.DefaultTopicRoot, "MQTT topic root")
	addStringVarFlag(&mqttConfig.Host, "mqttHost", envMQTTHost, gateway.DefaultHost, "MQTT host")
	addStringVarFlag(&mqttConfig.Port, "mqttPort", envMQTTPort, gateway.DefaultPort, "MQTT port")
	addStringVarFlag(&mqttConfig.Username, "mqttUsername", envMQTTUsername, "", "MQTT username")
	addStringVarFlag(&mqttConfig.Password, "mqttPassword", envMQTTPassword, "", "MQTT password")

	externConfigDir := flag.String("configDir", "", "configuration directory")

	flag.Parse()

	gw, err := gateway.New(lg, mqttConfig)
	check(err)
	defer gw.Close()

	// http server
	server := server.New(lg, httpConfig)
	defer server.Close()

	lg.Printf("load embedded configuration files")
	config := newConfig(lg)
	check(config.load(embedFsys, embedConfigDir))

	if *externConfigDir != "" {
		lg.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		check(config.load(externFsys, "."))
	}

	// register devices
	deviceSets := newDeviceSets(lg, gw)
	defer deviceSets.close()
	check(deviceSets.register(config))
	deviceSets.registerHTTP(server)

	// start http server listen and serve
	check(server.ListenAndServe())

	// start gateway listening
	check(gw.Listen())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
}
