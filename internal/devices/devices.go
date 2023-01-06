// Package devices provides the pico-cs device and configuration types.
package devices

import (
	"net/http"
	"strings"
)

// ident for json marshalling.
var indent = strings.Repeat(" ", 4)

// HTTPHandler is a anlder function providing the main html index for the devices.
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(idxHTML))
}
