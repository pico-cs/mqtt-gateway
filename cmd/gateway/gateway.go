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
	envUsername  = "Username"
	envPassword  = "Password"
)

func lookupEnv(name, defVal string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}
	return defVal
}

func main() {

	logger := log.New(os.Stderr, "gateway", log.LstdFlags)

	config := &gateway.Config{}
	configSet := newConfigSet(logger)
	defer configSet.close()

	flag.StringVar(&config.TopicRoot, "topicRoot", lookupEnv(envTopicRoot, gateway.DefaultTopicRoot), "topic root")
	flag.StringVar(&config.Host, "host", lookupEnv(envHost, gateway.DefaultHost), "MQTT host")
	flag.StringVar(&config.Port, "port", lookupEnv(envPort, gateway.DefaultPort), "MQTT port")
	flag.StringVar(&config.Username, "username", lookupEnv(envUsername, ""), "user name")
	flag.StringVar(&config.Password, "password", lookupEnv(envPassword, ""), "password")

	externConfigDir := flag.String("configDir", "", "configuration directory")

	flag.Parse()

	logger.Printf("load embedded configuration files")
	if err := configSet.load(embedFsys, embedConfigDir); err != nil {
		logger.Fatal(err)
	}

	if *externConfigDir != "" {
		logger.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		if err := configSet.load(externFsys, "."); err != nil {
			logger.Fatal(err)
		}
	}

	gw, err := gateway.New(config)
	if err != nil {
		logger.Fatal(err)
	}
	defer gw.Close()

	if err := configSet.register(gw); err != nil {
		logger.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

}
