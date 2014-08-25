// sysctld is a server providing sysctl values via HTTP GET requests.  Only
// integer and string values may be retrieved. Tables are not supported.
//
// To get the number of active cpus (hw.ncpu) the corresponding request is
//	http://<host>/integer/hw/ncpu
//
// To get the hostname (kern.hostname) the corresponding request is
//	http://<host>/string/kern/hostname
//
// The answer is a JSON encoded object conatining the "Name" of the requested
// value and the actual "Value". A RFC1123 compliant "Timestamp" designates the
// point in time when the data was acquired. If something went wrong an error
// message can be found in "Error".
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/blabber/go-freebsd-sysctl/sysctl"
	"log"
	"net/http"
	"strings"
	"time"
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
}

// sysctlHandler implements http.Handler and handles the sysctl requests.
type sysctlHandler struct {
	sysctlFunc func(name string, timestamp string) (sysctl *sc, err error)
}

// newSysctlHandler creates a new sysctlHandler for the sysctlType t.
func newSysctlHandler(t sysctlType) (s *sysctlHandler) {
	var sysctlFunc func(name string, timestamp string) (s *sc, err error)

	switch {
	case t == SCT_STRING:
		sysctlFunc = func(name string, timestamp string) (s *sc, err error) {
			val, err := sysctl.GetString(name)
			if err != nil {
				return
			}
			s = &sc{Name: name, Value: val, Timestamp: timestamp}
			return
		}
	case t == SCT_INTEGER:
		sysctlFunc = func(name string, timestamp string) (s *sc, err error) {
			val, err := sysctl.GetInt64(name)
			if err != nil {
				return
			}
			s = &sc{Name: name, Value: val, Timestamp: timestamp}
			return
		}
	}

	s = &sysctlHandler{sysctlFunc: sysctlFunc}
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
	val, err := h.sysctlFunc(path, timestamp)
	if err != nil {
		message := fmt.Sprintf("Could not get sysctl %v: %v", path, err)
		log.Printf(message)

		e := struct {
			Error     string
			Timestamp string
		}{
			Error:     message,
			Timestamp: timestamp,
		}
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
