package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHTTPHandler(t *testing.T) {
	c := make(chan string, 1)
	handler := newHTTPHandler(c)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()
	rd := strings.NewReader("hello, world!")
	req, err := http.NewRequest("POST", server.URL+"/", rd)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	msg := <-c

	if got, want := msg, "hello, world!"; got != want {
		t.Errorf("bad message: got %q, want %q", got, want)
	}

	defer resp.Body.Close()
}

func TestHTTPServerShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fakeHandler := func(w http.ResponseWriter, req *http.Request) {}
	srv := &http.Server{
		Addr:    ":8888",
		Handler: http.HandlerFunc(fakeHandler),
	}

	go srv.ListenAndServe()

	ch := make(chan string)

	server := &HTTPServer{
		server: srv,
		ctx:    ctx,
		ch:     ch,
	}

	go server.waitForShutdown()

	cancel()

	<-ch
}

func TestMultiplexerIterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// TODO: use an interface instead of an actual server here
	srv := &http.Server{
		Addr:    ":8888",
		Handler: nil,
	}
	go srv.ListenAndServe()
	m := &Multiplexer{
		ctx:    ctx,
		cancel: cancel,
		stdin:  make(chan string),
		http: &HTTPServer{
			ch:     make(chan string),
			server: srv,
		},
		sigs: make(chan os.Signal),
	}
	// Test that the iter method unblocks when the context is cancelled
	cancel()
	m.iter()
}

func TestMultiplexerSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	// TODO: use an interface instead of an actual server here
	srv := &http.Server{
		Addr:    ":8888",
		Handler: nil,
	}
	go srv.ListenAndServe()
	m := &Multiplexer{
		ctx:    ctx,
		cancel: cancel,
		stdin:  make(chan string),
		http: &HTTPServer{
			ch:     make(chan string),
			server: srv,
		},
		sigs: sigs,
	}
	// Test that the iter method unblocks when a signal is caught
	sigs <- os.Interrupt
	m.iter()
}

func TestMultiplexerStdin(t *testing.T) {
	// TODO(eric): make this test work!
	stdin := handleStdin(context.Background())
	m := &Multiplexer{
		stdin: stdin,
	}
	// Test that the iter method unblocks when we write to stdin
	if _, err := os.Stdin.Write([]byte("hello, world!")); err != nil {
		t.Fatal(err)
	}
	m.iter()
}
