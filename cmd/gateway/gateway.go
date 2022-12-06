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

	config := &gateway.Config{}
	configSet := newConfigSet()

	flag.StringVar(&config.TopicRoot, "topicRoot", lookupEnv(envTopicRoot, gateway.DefaultTopicRoot), "topic root")
	flag.StringVar(&config.Host, "host", lookupEnv(envHost, gateway.DefaultHost), "MQTT host")
	flag.StringVar(&config.Port, "port", lookupEnv(envPort, gateway.DefaultPort), "MQTT port")
	flag.StringVar(&config.Username, "username", lookupEnv(envUsername, ""), "user name")
	flag.StringVar(&config.Password, "password", lookupEnv(envPassword, ""), "password")

	externConfigDir := flag.String("configDir", "", "configuration directory")

	flag.Parse()

	log.Printf("load embedded configuration files")
	if err := configSet.load(embedFsys, embedConfigDir); err != nil {
		os.Exit(1)
	}

	if *externConfigDir != "" {
		log.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		if err := configSet.load(externFsys, "."); err != nil {
			os.Exit(1)
		}
	}

	gw, err := gateway.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer gw.Close()

	locoMap := map[string]string{}

	for csName, csConfig := range configSet.csConfigMap {
		log.Printf("register central station %s", csName)
		cs, err := gateway.NewCS(csConfig, gw)
		if err != nil {
			log.Fatal(err)
		}
		defer cs.Close()

		for locoName, locoConfig := range configSet.locoConfigMap {

			csAssignedName, ok := locoMap[locoName]

			controlsLoco, err := cs.AddLoco(locoConfig)
			if err != nil {
				log.Fatal(err)
			}

			if controlsLoco && ok {
				log.Fatalf("loco %s is controlled by more than one central station %s, %s", locoName, csName, csAssignedName)
			}

			locoMap[locoName] = csName

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
