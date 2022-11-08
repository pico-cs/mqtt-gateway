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

func (l *loader) loadConfig(fsys fs.FS, path string) {
	fn := filepath.Join(path, configFilename)

	var config gateway.Config
	if err := l.loadFile(fsys, fn, &config); err != nil {
		log.Printf("...%s %s", fn, err)
	}

	ok := l.config != nil
	l.config = &config
	log.Printf("...loaded %s %s", fn, overwrite(ok))
}

func splitNameExt(fn string) (string, string) {
	ext := filepath.Ext(fn)
	name := fn[:len(fn)-len(ext)]
	return strings.ToLower(name), strings.ToLower(ext) // not case sensitive
}

type overwrite bool

func (ow overwrite) String() string {
	if !ow {
		return ""
	}
	return "(overwrite)"
}

func (l *loader) loadCSConfigMap(fsys fs.FS, path string) {
	fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fileName, ext := splitNameExt(d.Name())
		if ext != extJSON {
			log.Printf("...skipped %s", subPath)
			return nil
		}
		var cs gateway.CSConfig
		if err = l.loadFile(fsys, subPath, &cs); err != nil {
			log.Printf("...%s %s", subPath, err)
		}

		if cs.Name == "" {
			cs.Name = fileName // if name is empty use file name
		}
		_, ok := l.csConfigMap[cs.Name]
		l.csConfigMap[cs.Name] = &cs
		log.Printf("...loaded %s as %s %s", subPath, cs.Name, overwrite(ok))
		return nil
	})
}

func (l *loader) loadLocoConfigMap(fsys fs.FS, path string) {
	fs.WalkDir(fsys, path, func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fileName, ext := splitNameExt(d.Name())
		if ext != extJSON {
			log.Printf("...skipped %s", subPath)
			return nil
		}
		var loco gateway.LocoConfig
		if err = l.loadFile(fsys, subPath, &loco); err != nil {
			log.Printf("...%s %s", subPath, err)
		}

		if loco.Name == "" {
			loco.Name = fileName // if name is empty use file name
		}
		_, ok := l.locoConfigMap[loco.Name]
		l.locoConfigMap[loco.Name] = &loco
		log.Printf("...loaded %s as %s %s", subPath, loco.Name, overwrite(ok))
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
