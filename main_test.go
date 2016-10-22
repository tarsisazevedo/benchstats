package main

import (
	"bytes"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestVisitURL(t *testing.T) {
	server := httptest.NewServer(nil)
	stat := []Stat{}
	var wg sync.WaitGroup
	wg.Add(1)
	visit(server.URL, &stat, &wg)
	wg.Wait()
	if len(stat) == 0 {
		t.Fatalf("Got Error. Expecting one stat, got zero.")
	}
}

func TestSumarizeStat(t *testing.T) {
	expected := `Average request time: 1s
DNS Lookup: 0.2s
TCP Connections: 0.2s
Server Procesing: 0.2s
Server Tranfer: 0.4s
`
	stats := []Stat{{
		DNSLookup:        time.Duration(0.2 * float64(time.Second)),
		TCPConnection:    time.Duration(0.2 * float64(time.Second)),
		ServerProccesing: time.Duration(0.2 * float64(time.Second)),
		ContentTransfer:  time.Duration(0.4 * float64(time.Second)),
		Total:            time.Duration(1 * time.Second),
	}}
	var buf bytes.Buffer
	sumarize(stats, &buf)
	if buf.String() != expected {
		t.Fatalf("Wrong sumary")
	}
}

func runMainForTest(t *testing.T, wantedExit int, args ...string) {
	exit := Main(args...)

	if exit != wantedExit {
		t.Fatalf("got exit code %d, but wanted %d", exit, wantedExit)
	}
}

func TestCallWithoutConnectionFlag(t *testing.T) {

	called := false
	usage = func() { called = true }

	runMainForTest(t, 1, "http://dummydomain.com")

	if !called {
		t.Error("should call usage without -c flag")
	}
}

func TestCallWithoutUrl(t *testing.T) {
	called := false
	usage = func() { called = true }

	runMainForTest(t, 1, "-c", "10")

	if !called {
		t.Error("should call usage wihout url")
	}
}
