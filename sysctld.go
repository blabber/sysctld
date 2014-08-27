// "THE BEER-WARE LICENSE" (Revision 42):
// <tobias.rehbein@web.de> wrote this file. As long as you retain this notice
// you can do whatever you want with this stuff. If we meet some day, and you
// think this stuff is worth it, you can buy me a beer in return.
//                                                             Tobias Rehbein

// sysctld is a server providing sysctl values via HTTP GET requests.  Only
// integer and string values may be retrieved. Tables are not supported.
//
// To get the number of active cpus (hw.ncpu) the corresponding request is
//	http://<host>/integer/hw/ncpu
//
// To get the hostname (kern.hostname) the corresponding request is
//	http://<host>/string/kern/hostname
//
// The answer is a JSON encoded object containing the "Name" of the requested
// value and the actual "Value". A RFC1123 compliant "Timestamp" designates the
// point in time when the data was acquired. If something went wrong an error
// message can be found in "Error".
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/blabber/go-freebsd-sysctl/sysctl"
)

const (
	stringPrefix  = "/sysctl/string/"
	integerPrefix = "/sysctl/integer/"
)

type sysctlType int

const (
	SCT_STRING sysctlType = iota
	SCT_INTEGER
)

// Struct sc represents a sysctl to be encoded in JSON.
type sc struct {
	Name      string
	Value     interface{}
	Timestamp string
	Error     string
}

// sysctlHandler implements http.Handler and handles the sysctl requests.
type sysctlHandler struct {
	scType sysctlType
	scFunc func(string) (interface{}, error)
}

// newSysctlHandler creates a new sysctlHandler for the sysctlType t.
func newSysctlHandler(t sysctlType) *sysctlHandler {
	var scFunc func(string) (interface{}, error)

	switch {
	case t == SCT_STRING:
		scFunc = func(name string) (v interface{}, err error) {
			v, err = sysctl.GetString(name)
			return
		}
	case t == SCT_INTEGER:
		scFunc = func(name string) (v interface{}, err error) {
			v, err = sysctl.GetInt64(name)
			return
		}
	}

	return &sysctlHandler{scFunc: scFunc, scType: t}
}

// ServeHTTP serves the requested sysctl encoded in JSON.
func (h sysctlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	name := strings.Replace(r.URL.Path, "/", ".", -1)
	timestamp := time.Now().Format(time.RFC1123)
	sc := &sc{Timestamp: timestamp, Name: name}

	var val interface{}
	val, err := h.scFunc(name)
	if err != nil {
		message := fmt.Sprintf("Could not get sysctl %v: %v", name, err)
		log.Printf(message)

		sc.Error = message
		switch {
		case h.scType == SCT_INTEGER:
			val = 0
		case h.scType == SCT_STRING:
			val = ""
		}

		w.WriteHeader(http.StatusNotFound)
	}
	sc.Value = val

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(sc); err != nil {
		log.Printf("error: encoder.Encode: %v", err)
		return
	}
}

// corsWrapper returns a handler that serves HTTP requests by adding
// Cross-Origin Resource Sharing (CORS) headers to the response and invoking
// the handler h.
func corsWrapper(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Add("Access-Control-Allow-Origin", origin)
			w.Header().Add("Access-Control-Allow-Headers", "content-type")
			w.Header().Add("Access-Control-Allow-Method", "GET")
		}

		h.ServeHTTP(w, r)
	})
}

func main() {
	http.Handle(stringPrefix, http.StripPrefix(stringPrefix,
		corsWrapper(newSysctlHandler(SCT_STRING))))
	http.Handle(integerPrefix, http.StripPrefix(integerPrefix,
		corsWrapper(newSysctlHandler(SCT_INTEGER))))

	address := flag.String("address", ":8080", "address to listen on")
	flag.Parse()

	log.Printf(`Listening on "%v"...`, *address)
	http.ListenAndServe(*address, nil)
}
