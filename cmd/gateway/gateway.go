package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pico-cs/mqtt-gateway/gateway"
)

//go:embed config/*
var embedFsys embed.FS

const (
	embedConfigDir = "config"
)

const (
	envTopicRoot = "TopicRoot"
	envHost      = "Host"
	envPort      = "Port"
)

func main() {

	lookupEnv := func(name, defVal string) string {
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return defVal
	}

	topicRoot := lookupEnv(envTopicRoot, gateway.DefaultTopicRoot)
	host := lookupEnv(envHost, gateway.DefaultHost)
	port := lookupEnv(envPort, gateway.DefaultPort)

	topicRootValue := &strValue{s: &topicRoot}
	hostValue := &strValue{s: &host}
	portValue := &strValue{s: &port}

	flag.Var(topicRootValue, "topicRoot", "topic root")
	flag.Var(hostValue, "host", "host")
	flag.Var(portValue, "port", "port")
	externConfigDir := flag.String("configDir", "", "configuration directory")

	flag.Parse()

	loader := newLoader()
	log.Printf("load embedded configuration files")
	loader.load(embedFsys, embedConfigDir)
	log.Printf("load external configuration files at %s", *externConfigDir)
	externFsys := os.DirFS(*externConfigDir)
	loader.load(externFsys, "")

	if topicRootValue.isSet || loader.config.TopicRoot == "" {
		loader.config.TopicRoot = topicRoot
	}
	if hostValue.isSet || loader.config.Host == "" {
		loader.config.Host = host
	}
	if portValue.isSet || loader.config.Port == "" {
		loader.config.Port = port
	}

	gw, err := gateway.New(loader.config)
	if err != nil {
		log.Fatal(err)
	}
	defer gw.Close()

	for _, csConfig := range loader.csConfigMap {
		cs, err := gateway.NewCS(csConfig, gw)
		if err != nil {
			log.Fatal(err)
		}
		defer cs.Close()
	}

	/*
		csConfig := &gateway.CSConfig{
			Name: "cs01",
		}
	*/

	// TODO locos

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

}
