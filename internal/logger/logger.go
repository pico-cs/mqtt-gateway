// Package logger provides common logging types.
package logger

import (
	"io"
	"log"
)

// Logger defines a logging interface.
type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
	Fatalf(format string, v ...any)
}

// Null is a discarding logger.
var Null = log.New(io.Discard, "", 0) // dev/null
