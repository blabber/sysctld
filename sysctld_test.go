// "THE BEER-WARE LICENSE" (Revision 42):
// <tobias.rehbein@web.de> wrote this file. As long as you retain this notice
// you can do whatever you want with this stuff. If we meet some day, and you
// think this stuff is worth it, you can buy me a beer in return.
//                                                             Tobias Rehbein

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	baseURI    = "http://localhost:8080"
	timeLayout = time.RFC1123
	origin     = "test.example"
)

var sysctlTests = []struct {
	name       string
	sctype     sysctlType
	statusCode int
}{
	{"kern.hostname", SCT_STRING, http.StatusOK},
	{"hw.ncpu", SCT_INTEGER, http.StatusOK},
	{"non.existent", SCT_INTEGER, http.StatusNotFound},
	{"non.existent", SCT_STRING, http.StatusNotFound},
}

var expectedHeaders = []struct {
	header string
	value  string
}{
	{"Access-Control-Allow-Origin", origin},
	{"Access-Control-Allow-Headers", "content-type"},
	{"Access-Control-Allow-Method", "GET"},
	{"Content-Type", "application/json"},
}

func TestSysctls(t *testing.T) {
	for _, test := range sysctlTests {
		t.Logf("-- Testing %q", test.name)

		var uri string
		switch {
		case test.sctype == SCT_INTEGER:
			uri = baseURI + integerPrefix + strings.Replace(test.name, "/", ".", -1)
		case test.sctype == SCT_STRING:
			uri = baseURI + stringPrefix + strings.Replace(test.name, "/", ".", -1)
		}

		request, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			t.Fatalf(`http.NewRequest("GET", %q, nil) failed: %v`, uri, err)
		}
		request.Header.Add("origin", "test.example")

		var handler http.Handler
		switch {
		case test.sctype == SCT_INTEGER:
			handler = http.StripPrefix(integerPrefix, newSysctlHandler(SCT_INTEGER))
		case test.sctype == SCT_STRING:
			handler = http.StripPrefix(stringPrefix, newSysctlHandler(SCT_STRING))
		}

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		// check status code
		if recorder.Code != test.statusCode {
			t.Fatalf("Unexpected status code %v (expected %v)",
				recorder.Code, test.statusCode)
		}

		// check headers
		for _, h := range expectedHeaders {
			if header := recorder.HeaderMap.Get(h.header); header != h.value {
				t.Fatalf("Header %q has value %q. Expected was %q.",
					h.header, header, h.value)
			}
		}

		// check JSON
		s := &sc{}
		decoder := json.NewDecoder(recorder.Body)
		if err := decoder.Decode(s); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}

		// check JSON: Name
		if s.Name != test.name {
			t.Fatalf("unexpected name %q (expected %q)", s.Name, test.name)
		}

		// check JSON: Value
		switch {
		case test.sctype == SCT_INTEGER:
			// float64 is the generic type the Decoder uses for numbers
			_, ok := s.Value.(float64)
			if !ok {
				t.Fatalf("Value is not an float64 but %T", s.Value)
			}
		case test.sctype == SCT_STRING:
			_, ok := s.Value.(string)
			if !ok {
				t.Fatalf("Value is not a string but %T", s.Value)
			}
		}

		// check JSON: Error
		expectError := (test.statusCode != http.StatusOK)
		if s.Error != "" && !expectError {
			t.Fatalf("unexpected error message: %q", s.Error)
		} else if s.Error == "" && expectError {
			t.Fatalf("expected error message was not returned (%q)", test.name)
		}

		// check JSON: Timestamp
		if _, err := time.Parse(timeLayout, s.Timestamp); err != nil {
			t.Fatalf("failed to parse timestamp: %v", err)
		}
	}
}
