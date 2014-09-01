// "THE BEER-WARE LICENSE" (Revision 42):
// <tobias.rehbein@web.de> wrote this file. As long as you retain this notice
// you can do whatever you want with this stuff. If we meet some day, and you
// think this stuff is worth it, you can buy me a beer in return.
//                                                             Tobias Rehbein

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	timeLayout = time.RFC1123
	origin     = "test.example"
)

type sysctlTest struct {
	name       string
	sctype     sysctlType
	statusCode int
}

func (t sysctlTest) String() string {
	return fmt.Sprintf("Test: name %v, type %v, expected status: %v", t.name, t.sctype, t.statusCode)
}

var sysctlTests = []sysctlTest{
	{"kern.hostname", sctString, http.StatusOK},
	{"hw.ncpu", sctInteger, http.StatusOK},
	{"non.existent", sctString, http.StatusNotFound},
	{"non.existent", sctInteger, http.StatusNotFound},
}

type header struct {
	header string
	value  string
}

var expectedSysctlHeaders = []header{
	{"Content-Type", "application/json"},
}

var expectedCorsHeaders = []header{
	{"Access-Control-Allow-Origin", origin},
	{"Access-Control-Allow-Headers", "content-type"},
	{"Access-Control-Allow-Method", "GET"},
}

var dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	return
})

func checkHeaders(actual http.Header, expected []header, t *testing.T) {
	for _, h := range expected {
		if header := actual.Get(h.header); header != h.value {
			t.Errorf("Header %q has value %q. Expected was %q.", h.header, header, h.value)
		}
	}
}

func TestCorsWrapper(t *testing.T) {
	request, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Errorf(`http.NewRequest("GET", "/", nil) failed: %v`, err)
	}
	request.Header.Add("origin", "test.example")

	handler := corsWrapper(dummyHandler)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	// check headers
	checkHeaders(recorder.HeaderMap, expectedCorsHeaders, t)
}

func TestSysctls(t *testing.T) {
	for _, test := range sysctlTests {
		t.Logf("-- %v", test)

		uri := strings.Replace(test.name, ".", "/", -1)
		request, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			t.Errorf(`http.NewRequest("GET", %q, nil) failed: %v`, uri, err)
		}
		request.Header.Add("origin", "test.example")

		handler := newSysctlHandler(test.sctype)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		// check status code
		if recorder.Code != test.statusCode {
			t.Errorf("Unexpected status code %v (expected %v)", recorder.Code, test.statusCode)
		}

		// check header
		checkHeaders(recorder.HeaderMap, expectedSysctlHeaders, t)

		// check JSON
		s := &sc{}
		decoder := json.NewDecoder(recorder.Body)
		if err := decoder.Decode(s); err != nil {
			t.Errorf("failed to decode json: %v", err)
		}

		// check JSON: Name
		if s.Name != test.name {
			t.Errorf("unexpected name %q (expected %q)", s.Name, test.name)
		}

		// check JSON: Value
		switch test.sctype {
		case sctInteger:
			// float64 is the generic type the decoder uses for numbers
			_, ok := s.Value.(float64)
			if !ok {
				t.Errorf("Value is not an float64 but %T", s.Value)
			}
		case sctString:
			_, ok := s.Value.(string)
			if !ok {
				t.Errorf("Value is not a string but %T", s.Value)
			}
		}

		// check JSON: Error
		expectError := (test.statusCode != http.StatusOK)
		if s.Error != "" && !expectError {
			t.Errorf("unexpected error message: %q", s.Error)
		} else if s.Error == "" && expectError {
			t.Error("expected error message was not returned")
		}

		// check JSON: Timestamp
		if _, err := time.Parse(timeLayout, s.Timestamp); err != nil {
			t.Errorf("failed to parse timestamp: %v", err)
		}
	}
}
