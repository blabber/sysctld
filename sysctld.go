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
	scFunc func(name string, timestamp string) (sysctl *sc, err error)
}

// newSysctlHandler creates a new sysctlHandler for the sysctlType t.
func newSysctlHandler(t sysctlType) (s *sysctlHandler) {
	var scFunc func(name string, timestamp string) (s *sc, err error)

	switch {
	case t == SCT_STRING:
		scFunc = func(name string, timestamp string) (s *sc, err error) {
			val, err := sysctl.GetString(name)
			if err != nil {
				return
			}
			s = &sc{Name: name, Value: val, Timestamp: timestamp}
			return
		}
	case t == SCT_INTEGER:
		scFunc = func(name string, timestamp string) (s *sc, err error) {
			val, err := sysctl.GetInt64(name)
			if err != nil {
				return
			}
			s = &sc{Name: name, Value: val, Timestamp: timestamp}
			return
		}
	}

	s = &sysctlHandler{scFunc: scFunc, scType: t}
	return
}

// ServeHTTP serves the requested sysctl encoded in JSON.
func (h *sysctlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	if origin := r.Header.Get("Origin"); origin != "" {
		// Enable Cross-Origin Resource Sharing (CORS)
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Headers", "content-type")
		w.Header().Add("Access-Control-Allow-Method", "GET")
	}

	path := strings.Replace(r.URL.Path, "/", ".", -1)
	timestamp := time.Now().Format(time.RFC1123)
	val, err := h.scFunc(path, timestamp)
	if err != nil {
		message := fmt.Sprintf("Could not get sysctl %v: %v", path, err)
		log.Printf(message)

		e := &sc{Name: path, Error: message, Timestamp: timestamp}
		switch {
		case h.scType == SCT_INTEGER:
			e.Value = 0
		case h.scType == SCT_STRING:
			e.Value = ""
		}
		w.WriteHeader(http.StatusNotFound)
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(e); err != nil {
			log.Printf("error: encoder.Encode: %v", err)
			return
		}
		return
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(val); err != nil {
		log.Printf("error: encoder.Encode: %v", err)
		return
	}
}

func main() {
	http.Handle(stringPrefix, http.StripPrefix(stringPrefix, newSysctlHandler(SCT_STRING)))
	http.Handle(integerPrefix, http.StripPrefix(integerPrefix, newSysctlHandler(SCT_INTEGER)))

	address := flag.String("address", ":8080", "address to listen on")
	flag.Parse()

	log.Printf(`Listening on "%v"...`, *address)
	http.ListenAndServe(*address, nil)
}
