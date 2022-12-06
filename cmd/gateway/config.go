package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/pico-cs/mqtt-gateway/gateway"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

var jamlExts = []string{".yaml", ".yml"}

type configSet struct {
	logger        *log.Logger
	csConfigMap   map[string]*gateway.CSConfig
	locoConfigMap map[string]*gateway.LocoConfig
	csList        []*gateway.CS
}

func newConfigSet(logger *log.Logger) *configSet {
	if logger == nil {
		logger = log.New(io.Discard, "", 0) // dev/null
	}
	return &configSet{
		logger:        logger,
		csConfigMap:   map[string]*gateway.CSConfig{},
		locoConfigMap: map[string]*gateway.LocoConfig{},
	}
}

func (c *configSet) close() error {
	var lastErr error
	for _, cs := range c.csList {
		if err := cs.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
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

func (c *configSet) parseYaml(b []byte) error {
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

func (c *configSet) load(fsys fs.FS, path string) error {
	return fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !slices.Contains(jamlExts, filepath.Ext(d.Name())) {
			c.logger.Printf("...skipped %s", subPath)
			return nil
		}

		b, err := fs.ReadFile(fsys, subPath)
		if err != nil {
			c.logger.Printf("...%s %s", subPath, err)
			return err
		}

		if err = c.parseYaml(b); err != nil {
			c.logger.Printf("...error loading %s: %s", subPath, err)
			return err
		}

		c.logger.Printf("...loaded %s", subPath)
		return nil
	})
}

func (c *configSet) register(gw *gateway.Gateway) error {
	locoMap := map[string]string{}

	for csName, csConfig := range c.csConfigMap {
		c.logger.Printf("register central station %s", csName)
		cs, err := gateway.NewCS(csConfig, gw)
		if err != nil {
			return err
		}
		c.csList = append(c.csList, cs)

		for locoName, locoConfig := range c.locoConfigMap {

			csAssignedName, ok := locoMap[locoName]

			controlsLoco, err := cs.AddLoco(locoConfig)
			if err != nil {
				return err
			}

			if controlsLoco && ok {
				return fmt.Errorf("loco %s is controlled by more than one central station %s, %s", locoName, csName, csAssignedName)
			}

			locoMap[locoName] = csName

			if controlsLoco {
				c.logger.Printf("added loco %s controlled by central station %s", locoConfig.Name, csConfig.Name)
			} else {
				c.logger.Printf("added loco %s to central station %s", locoConfig.Name, csConfig.Name)
			}
		}
	}
	return nil
}
