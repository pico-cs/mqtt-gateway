package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/pico-cs/mqtt-gateway/gateway"
)

func loadFile(fsys fs.FS, fn string, v any) error {
	b, err := fs.ReadFile(fsys, fn)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	return nil
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

func loadCSConfigMap(csConfigMap map[string]*gateway.CSConfig, fsys fs.FS, path string) {
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
		if err = loadFile(fsys, subPath, &cs); err != nil {
			log.Printf("...%s %s", subPath, err)
		}

		if cs.Name == "" {
			cs.Name = fileName // if name is empty use file name
		}
		_, ok := csConfigMap[cs.Name]
		csConfigMap[cs.Name] = &cs
		log.Printf("...loaded %s as %s %s", subPath, cs.Name, overwrite(ok))
		return nil
	})
}

func loadLocoConfigMap(locoConfigMap map[string]*gateway.LocoConfig, fsys fs.FS, path string) {
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
		if err = loadFile(fsys, subPath, &loco); err != nil {
			log.Printf("...%s %s", subPath, err)
		}

		if loco.Name == "" {
			loco.Name = fileName // if name is empty use file name
		}
		_, ok := locoConfigMap[loco.Name]
		locoConfigMap[loco.Name] = &loco
		log.Printf("...loaded %s as %s %s", subPath, loco.Name, overwrite(ok))
		return nil
	})
}
