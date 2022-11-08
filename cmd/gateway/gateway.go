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
	if *externConfigDir != "" {
		log.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		loader.load(externFsys, "")
	}

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

	i := 0
	for _, csConfig := range loader.csConfigMap {

		log.Printf("register central station %s", csConfig.Name)

		cs, err := gateway.NewCS(csConfig, gw)
		if err != nil {
			log.Fatal(err)
		}
		defer cs.Close()

		for _, locoConfig := range loader.locoConfigMap {
			if i == 0 && locoConfig.CSName == "" { // no controlling command station defined -> use first one
				locoConfig.CSName = csConfig.Name
			}

			log.Printf("add loco %s to central station %s", locoConfig.Name, csConfig.Name)

			if err := cs.AddLoco(locoConfig); err != nil {
				log.Fatal(err)
			}
		}
		i++
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

}
