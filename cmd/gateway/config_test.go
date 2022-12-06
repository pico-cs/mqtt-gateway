package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/pico-cs/mqtt-gateway/gateway"
)

func testLoad(t *testing.T) {
	cmpConfigSet := &configSet{
		csConfigMap: map[string]*gateway.CSConfig{
			"cs01": {
				Name:  "cs01",
				Port:  "/dev/ttyACM0",
				Incls: []string{".*"},
				Excls: []string{"br18"},
			},
			"cs02": {
				Name:  "cs02",
				Host:  "localhost",
				Port:  "4242",
				Incls: []string{"br18"},
			},
		},
		locoConfigMap: map[string]*gateway.LocoConfig{
			"br01": {
				Name: "br01",
				Addr: 1,
				Fcts: map[string]gateway.LocoFctConfig{
					"light": {No: 0},
					"horn":  {No: 5},
				},
			},
			"br18": {
				Name: "br18",
				Addr: 18,
				Fcts: map[string]gateway.LocoFctConfig{
					"light":   {No: 0},
					"bell":    {No: 5},
					"whistle": {No: 8},
				},
			},
		},
	}

	configSet := newConfigSet()
	externFsys := os.DirFS("config_examples")
	if err := configSet.load(externFsys, "."); err != nil {
		t.Fatal(err)
	}

	// compare
	if !reflect.DeepEqual(configSet, cmpConfigSet) {
		t.Fatalf("invalid config set %v - expected %v", configSet, cmpConfigSet)
	}

}

func TestConfig(t *testing.T) {
	tests := []struct {
		name string
		fct  func(t *testing.T)
	}{
		{"load", testLoad},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fct(t)
		})
	}
}
