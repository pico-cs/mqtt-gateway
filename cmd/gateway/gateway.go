package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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

const (
	csPath   = "cs"
	locoPath = "loco"
	extJSON  = ".json"
)

func lookupEnv(name, defVal string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}
	return defVal
}

func main() {

	config := &gateway.Config{}

	flag.StringVar(&config.TopicRoot, "topicRoot", lookupEnv(envTopicRoot, gateway.DefaultTopicRoot), "topic root")
	flag.StringVar(&config.Host, "host", lookupEnv(envHost, gateway.DefaultHost), "MQTT host")
	flag.StringVar(&config.Port, "port", lookupEnv(envPort, gateway.DefaultPort), "MQTT port")
	flag.StringVar(&config.Username, "username", lookupEnv(envUsername, ""), "user name")
	flag.StringVar(&config.Password, "password", lookupEnv(envPassword, ""), "password")

	externConfigDir := flag.String("configDir", "", "configuration directory")

	flag.Parse()

	csConfigMap := make(map[string]*gateway.CSConfig)
	locoConfigMap := make(map[string]*gateway.LocoConfig)

	log.Printf("load embedded configuration files")
	loadCSConfigMap(csConfigMap, embedFsys, filepath.Join(embedConfigDir, csPath))
	loadLocoConfigMap(locoConfigMap, embedFsys, filepath.Join(embedConfigDir, locoPath))

	if *externConfigDir != "" {
		log.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		loadCSConfigMap(csConfigMap, externFsys, csPath)
		loadLocoConfigMap(locoConfigMap, externFsys, locoPath)
	}

	gw, err := gateway.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer gw.Close()

	locoMap := map[string]string{}

	for _, csConfig := range csConfigMap {
		log.Printf("register central station %s", csConfig.Name)
		cs, err := gateway.NewCS(csConfig, gw)
		if err != nil {
			log.Fatal(err)
		}
		defer cs.Close()

		for _, locoConfig := range locoConfigMap {

			csName, ok := locoMap[locoConfig.Name]

			controlsLoco, err := cs.AddLoco(locoConfig)
			if err != nil {
				log.Fatal(err)
			}

			if controlsLoco && ok {
				log.Fatalf("loco %s is controlled by more than one central station %s, %s", locoConfig.Name, csConfig.Name, csName)
			}

			if controlsLoco {
				log.Printf("added loco %s controlled by central station %s", locoConfig.Name, csConfig.Name)
			} else {
				log.Printf("added loco %s to central station %s", locoConfig.Name, csConfig.Name)
			}
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

}
