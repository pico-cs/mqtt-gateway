package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/pico-cs/mqtt-gateway/gateway"
)

const (
	configFilename = "gateway.json"
	csPath         = "cs"
	locoPath       = "loco"
	extJSON        = ".json"
)

type loader struct {
	config        *gateway.Config
	csConfigMap   map[string]*gateway.CSConfig
	locoConfigMap map[string]*gateway.LocoConfig
}

func newLoader() *loader {
	return &loader{
		csConfigMap:   make(map[string]*gateway.CSConfig),
		locoConfigMap: make(map[string]*gateway.LocoConfig),
	}
}

func (l *loader) loadFile(fsys fs.FS, fn string, v any) error {
	b, err := fs.ReadFile(fsys, fn)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	return nil
}

func (l *loader) log(fn string, err error, overwrite bool) {
	switch {
	case err != nil:
		log.Printf("...%s %s", fn, err)
	case overwrite:
		log.Printf("...loaded %s (overwrite)", fn)
	default:
		log.Printf("...loaded %s", fn)
	}
}

func (l *loader) loadConfig(fsys fs.FS, path string) {
	overwrite := l.config != nil

	fn := filepath.Join(path, configFilename)

	var config gateway.Config
	err := l.loadFile(fsys, fn, &config)
	if err == nil {
		l.config = &config
	}
	l.log(fn, err, overwrite)
}

func splitNameExt(fn string) (string, string) {
	ext := filepath.Ext(fn)
	name := fn[:len(fn)-len(ext)]
	return strings.ToLower(name), strings.ToLower(ext) // not case sensitive
}

func (l *loader) loadCSConfigMap(fsys fs.FS, path string) {
	fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name, ext := splitNameExt(d.Name())
		if ext != extJSON {
			log.Printf("...skipped %s", subPath)
			return nil
		}
		_, overwrite := l.csConfigMap[name]
		var cs gateway.CSConfig
		err = l.loadFile(fsys, subPath, &cs)
		if err == nil {
			if cs.Name == "" {
				cs.Name = name // if name is empty use file name
			}
			l.csConfigMap[name] = &cs
		}
		l.log(subPath, err, overwrite)
		return nil
	})
}

func (l *loader) loadLocoConfigMap(fsys fs.FS, path string) {
	fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name, ext := splitNameExt(d.Name())
		if ext != extJSON {
			log.Printf("...skipped %s", subPath)
			return nil
		}
		_, overwrite := l.locoConfigMap[name]
		var loco *gateway.LocoConfig
		err = l.loadFile(fsys, subPath, loco)
		if err == nil {
			if loco.Name == "" {
				loco.Name = name // if name is empty use file name
			}
			l.locoConfigMap[name] = loco
		}
		l.log(subPath, err, overwrite)
		return nil
	})
}

func (l *loader) load(fsys fs.FS, path string) {
	l.loadConfig(fsys, path)
	if l.config == nil {
		l.config = &gateway.Config{}
	}
	l.loadCSConfigMap(fsys, filepath.Join(path, csPath))
	l.loadLocoConfigMap(fsys, filepath.Join(path, locoPath))
}
