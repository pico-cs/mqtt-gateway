package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

const extJSON = ".json"

func lookupEnv(name, defVal string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}
	return defVal
}

func splitNameExt(fn string) (string, string) {
	ext := filepath.Ext(fn)
	name := fn[:len(fn)-len(ext)]
	return strings.ToLower(name), strings.ToLower(ext) // not case sensitive
}

func loadConfig(fsys fs.FS, path string, fn func(filename string, b []byte) (bool, error)) {
	fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		filename, ext := splitNameExt(d.Name())
		if ext != extJSON {
			log.Printf("...skipped %s", subPath)
			return nil
		}

		b, err := fs.ReadFile(fsys, subPath)
		if err != nil {
			log.Printf("...%s %s", subPath, err)
			return nil
		}

		overwrite, err := fn(filename, b)
		switch {
		case err != nil:
			log.Printf("...error loading %s: %s", subPath, err)
		case overwrite:
			log.Printf("...loaded %s as %s (overwrite)", subPath, filename)
		default:
			log.Printf("...loaded %s as %s", subPath, filename)
		}
		return nil
	})
}

type configs struct {
	csConfigMap   map[string]*gateway.CSConfig
	locoConfigMap map[string]*gateway.LocoConfig
}

func newConfigs() *configs {
	return &configs{
		csConfigMap:   map[string]*gateway.CSConfig{},
		locoConfigMap: map[string]*gateway.LocoConfig{},
	}
}

func (c *configs) addConfig(filename string, b []byte) (bool, error) {
	var m map[string]any

	if err := json.Unmarshal(b, &m); err != nil {
		return false, err
	}

	if _, ok := m["host"]; ok {
		csConfig, err := gateway.NewCSConfig(filename, b)
		if err != nil {
			return false, err
		}
		_, ok := c.csConfigMap[csConfig.Name]
		c.csConfigMap[csConfig.Name] = csConfig
		return ok, nil
	}
	if _, ok := m["addr"]; ok {
		locoConfig, err := gateway.NewLocoConfig(filename, b)
		if err != nil {
			return false, err
		}
		_, ok := c.locoConfigMap[locoConfig.Name]
		c.locoConfigMap[locoConfig.Name] = locoConfig
		return ok, nil
	}
	return false, fmt.Errorf("invalid configuration: %s", b)
}

func main() {

	config := &gateway.Config{}
	configs := newConfigs()

	flag.StringVar(&config.TopicRoot, "topicRoot", lookupEnv(envTopicRoot, gateway.DefaultTopicRoot), "topic root")
	flag.StringVar(&config.Host, "host", lookupEnv(envHost, gateway.DefaultHost), "MQTT host")
	flag.StringVar(&config.Port, "port", lookupEnv(envPort, gateway.DefaultPort), "MQTT port")
	flag.StringVar(&config.Username, "username", lookupEnv(envUsername, ""), "user name")
	flag.StringVar(&config.Password, "password", lookupEnv(envPassword, ""), "password")

	externConfigDir := flag.String("configDir", "", "configuration directory")

	flag.Parse()

	log.Printf("load embedded configuration files")
	loadConfig(embedFsys, embedConfigDir, configs.addConfig)

	if *externConfigDir != "" {
		log.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		loadConfig(externFsys, "", configs.addConfig)
	}

	gw, err := gateway.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer gw.Close()

	locoMap := map[string]string{}

	for _, csConfig := range configs.csConfigMap {
		log.Printf("register central station %s", csConfig.Name)
		cs, err := gateway.NewCS(csConfig, gw)
		if err != nil {
			log.Fatal(err)
		}
		defer cs.Close()

		for _, locoConfig := range configs.locoConfigMap {

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
