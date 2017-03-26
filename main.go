package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"sync"
	"time"

	"text/template"
)

type Stat struct {
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProccesing time.Duration
	ContentTransfer  time.Duration
	Total            time.Duration
}

var c int
var t int

func init() {
	flag.IntVar(&c, "c", 1, "Number of connections. It should be > 0.")
	flag.IntVar(&t, "t", 1, "Total calls.")
}

func main() {
	flag.Parse()
	fsArgs := flag.Args()
	if len(fsArgs) == 0 {
		os.Exit(1)
	}
	tc := t
	nc := c
	stats := make(chan Stat, tc*2)
	url := fsArgs[0]
	bench(url, tc, nc, stats)
	sumarize(stats, os.Stdout)
	os.Exit(0)
}

func bench(url string, quantity int, threads int, stats chan Stat) {
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go visit(url, stats, quantity, &wg)
	}
	wg.Wait()
}

func visit(url string, stats chan Stat, tc int, wg *sync.WaitGroup) {
	defer wg.Done()
	if !strings.HasPrefix(url, "http") {
        	url = "http://" + url
	}
	for {
		var dnsStart, dnsDone, connDone, gotConn, transferInit, done time.Time
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatalf("new request failed: %v", err)
		}
		trace := &httptrace.ClientTrace{
			DNSStart: func(info httptrace.DNSStartInfo) {
				dnsStart = time.Now()
			},
			DNSDone: func(info httptrace.DNSDoneInfo) {
				dnsDone = time.Now()
			},
			ConnectStart: func(x, y string) {
				if dnsDone.IsZero() {
					dnsDone = time.Now()
				}
			},
			ConnectDone: func(net, addr string, err error) {
				if err != nil {
					log.Fatalf("unable to connect to host %v: %v", addr, err)
				}
				connDone = time.Now()
			},
			GotConn: func(info httptrace.GotConnInfo) {
				gotConn = time.Now()
			},
			GotFirstResponseByte: func() { transferInit = time.Now() },
		}
		req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))
		tr := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		client := &http.Client{
			Transport: tr,
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("request failed: %v", err)
		}
		fmt.Printf("%s\n", resp.Status)
		done = time.Now()
		if transferInit.IsZero() {
			transferInit = done
		}
		if dnsStart.IsZero() {
			dnsStart = dnsDone
		}
		stat := Stat{
			DNSLookup:        dnsDone.Sub(dnsStart),
			TCPConnection:    connDone.Sub(dnsDone),
			TLSHandshake:     gotConn.Sub(connDone),
			ServerProccesing: transferInit.Sub(gotConn),
			ContentTransfer:  done.Sub(transferInit),
			Total:            done.Sub(dnsStart),
		}
		stats <- stat
		if len(stats) >= tc {
			break
		}
	}
}

func sumarize(stats chan Stat, w io.Writer) {
	summ := Stat{}
	size := int64(len(stats))
	close(stats)
	for s := range stats {
		summ.DNSLookup = time.Duration(summ.DNSLookup.Nanoseconds() + s.DNSLookup.Nanoseconds())
		summ.TCPConnection = time.Duration(summ.TCPConnection.Nanoseconds() + s.TCPConnection.Nanoseconds())
		summ.TLSHandshake = time.Duration(summ.TLSHandshake.Nanoseconds() + s.TLSHandshake.Nanoseconds())
		summ.ServerProccesing = time.Duration(summ.ServerProccesing.Nanoseconds() + s.ServerProccesing.Nanoseconds())
		summ.ContentTransfer = time.Duration(summ.ContentTransfer.Nanoseconds() + s.ContentTransfer.Nanoseconds())
		summ.Total = time.Duration(summ.Total.Nanoseconds() + s.Total.Nanoseconds())
	}
	summ.DNSLookup = time.Duration((summ.DNSLookup.Nanoseconds() / size))
	summ.TCPConnection = time.Duration((summ.TCPConnection.Nanoseconds() / size))
	summ.TLSHandshake = time.Duration((summ.TLSHandshake.Nanoseconds() / size))
	summ.ServerProccesing = time.Duration((summ.ServerProccesing.Nanoseconds() / size))
	summ.ContentTransfer = time.Duration((summ.ContentTransfer.Nanoseconds() / size))
	summ.Total = time.Duration((summ.Total.Nanoseconds() / size))
	sumaryTmpl := `Average request time: {{.Total.Seconds }}s
DNS Lookup: {{ .DNSLookup.Seconds }}s
TCP Connections: {{ .TCPConnection.Seconds }}s
Server Procesing: {{ .ServerProccesing.Seconds }}s
Server Tranfer: {{ .ContentTransfer.Seconds }}s
`
	tmpl, _ := template.New("summary").Parse(sumaryTmpl)
	tmpl.Execute(w, summ)
}
