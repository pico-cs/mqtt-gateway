package main

import (
	"os"
	"testing"
)

type loggerWrapper struct {
	T *testing.T
}

func (w *loggerWrapper) Printf(format string, v ...any) { w.T.Logf(format, v...) }
func (w *loggerWrapper) Println(v ...any)               { w.T.Log(v...) }
func (w *loggerWrapper) Fatalf(format string, v ...any) { w.T.Fatalf(format, v...) }

func testLoad(t *testing.T) {
	logger := &loggerWrapper{T: t}

	config := newConfig(logger)
	externFsys := os.DirFS("config_examples")
	if err := config.load(externFsys, "."); err != nil {
		t.Fatal(err)
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
