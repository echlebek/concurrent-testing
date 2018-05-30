package main

import (
	"bufio"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
)

type HTTPServer struct {
	ctx     context.Context
	server  *http.Server
	handler http.Handler
	ch      chan string
}

func (s *HTTPServer) C() chan string {
	return s.ch
}

func NewHTTPServer(ctx context.Context) *HTTPServer {
	ch := make(chan string)
	handler := newHTTPHandler(ch)
	s := &HTTPServer{
		server: &http.Server{
			Addr:    ":8888",
			Handler: handler,
		},
		ctx:     ctx,
		ch:      ch,
		handler: handler,
	}

	go func() {
		log.Println(s.server.ListenAndServe())
	}()
	log.Println("Listening on 8888")

	go s.waitForShutdown()
	return s
}

func (s *HTTPServer) waitForShutdown() {
	<-s.ctx.Done()
	if err := s.server.Shutdown(context.TODO()); err != nil {
		log.Println(err)
	}
	close(s.ch)
	// wg.Done()
}

// handleStdin writes data from standard in to a channel
func handleStdin(ctx context.Context) chan string {
	s := bufio.NewScanner(os.Stdin)
	ch := make(chan string, 1)
	go func() {
		defer func() {
			if err := s.Err(); err != nil {
				log.Println(err)
			}
			close(ch)
			// wg.Done()
		}()
		for s.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- s.Text():
			}
		}
	}()
	return ch
}

func newHTTPHandler(ch chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Println("HTTP error:", err)
		}
		ch <- string(b)
	}
}

type Multiplexer struct {
	http   *HTTPServer
	stdin  chan string
	sigs   chan os.Signal
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMultiplexer(ctx context.Context, http *HTTPServer, stdin chan string, sigs chan os.Signal, cancel context.CancelFunc) *Multiplexer {
	return &Multiplexer{
		ctx:    ctx,
		http:   http,
		stdin:  stdin,
		sigs:   sigs,
		cancel: cancel,
	}
}

func (m *Multiplexer) Start() {
	go m.run()
}

func (m *Multiplexer) run() {
	for {
		m.iter()
	}
}

func (m *Multiplexer) iter() {
	select {
	case <-m.ctx.Done():
		return
	case <-m.sigs:
		m.cancel()
	case msg := <-m.stdin:
		log.Printf("Message from stdin: %q\n", msg)
	case msg := <-m.http.C():
		log.Printf("Message from HTTP: %q\n", msg)
	}
}

func main() {
	log.Println("Awesome Server version 0.0.0 running")
	// var wg sync.WaitGroup

	// Shutdown logic
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		log.Println("Shutting down...")
		// wg.Wait()
		if err := ctx.Err(); err != nil && err != context.Canceled {
			log.Fatal(err)
		}
	}()

	// Handle os.Interrupt
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	// Echo messages from stdin
	stdin := handleStdin(ctx)

	// Echo messages from HTTP port 8888
	http := NewHTTPServer(ctx)

	// Two workers handling incoming messages
	// wg.Add(2)

	m := NewMultiplexer(ctx, http, stdin, sig, cancel)
	m.Start()

	<-ctx.Done()
}
