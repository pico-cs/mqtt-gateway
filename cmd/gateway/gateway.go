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

	"github.com/pico-cs/mqtt-gateway/gateway"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
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

var jamlExts = []string{".yaml", ".yml"}

func lookupEnv(name, defVal string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}
	return defVal
}

func loadConfig(fsys fs.FS, path string, fn func(b []byte) error) error {
	return fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !slices.Contains(jamlExts, filepath.Ext(d.Name())) {
			log.Printf("...skipped %s", subPath)
			return nil
		}

		b, err := fs.ReadFile(fsys, subPath)
		if err != nil {
			log.Printf("...%s %s", subPath, err)
			return nil
		}

		err = fn(b)
		if err != nil {
			log.Printf("...error loading %s: %s", subPath, err)
		} else {
			log.Printf("...loaded %s", subPath)
		}
		return err
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

func isCSConfig(m map[string]any) bool {
	if _, ok := m["host"]; ok {
		return true
	}
	if _, ok := m["port"]; ok {
		return true
	}
	return false
}

func isLocoConfig(m map[string]any) bool {
	if _, ok := m["addr"]; ok {
		return true
	}
	return false
}

func (c *configs) parseYaml(b []byte) error {
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

		if _, ok := m["name"]; !ok {
			return fmt.Errorf("invalid document %v - name missing", m)
		}

		switch {

		case isCSConfig(m):
			var csConfig gateway.CSConfig
			if err := dd.Decode(&csConfig); err != nil {
				return err
			}
			c.csConfigMap[csConfig.Name] = &csConfig

		case isLocoConfig(m):
			var locoConfig gateway.LocoConfig
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
	if err := loadConfig(embedFsys, embedConfigDir, configs.parseYaml); err != nil {
		os.Exit(1)
	}

	if *externConfigDir != "" {
		log.Printf("load external configuration files at %s", *externConfigDir)
		externFsys := os.DirFS(*externConfigDir)
		if err := loadConfig(externFsys, ".", configs.parseYaml); err != nil {
			os.Exit(1)
		}
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
