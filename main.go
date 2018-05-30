package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
)

// handleStdin writes data from standard in to a channel
func handleStdin(ctx context.Context) chan string {
	s := bufio.NewScanner(os.Stdin)
	ch := make(chan string, 1)
	go func() {
		defer func() {
			close(ch)
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

// handleHTTP writes HTTP request bodies to a channel
func handleHTTP(ctx context.Context) chan string {
	ch := make(chan string, 1)
	hf := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Println("HTTP error:", err)
		}
		ch <- string(b)
	})
	server := http.Server{
		Addr:    ":8888",
		Handler: hf,
	}
	go func() {
		log.Println(server.ListenAndServe())
	}()
	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.TODO()); err != nil {
			log.Println(err)
		}
		close(ch)
	}()
	return ch
}

func main() {
	fmt.Println("Awesome Server version 0.0.0 running")

	// Shutdown logic
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
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
	http := handleHTTP(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sig:
			fmt.Println("\rCaught signal, shutting down")
			cancel()
		case msg := <-stdin:
			fmt.Printf("Message from stdin: %q\n", msg)
		case msg := <-http:
			fmt.Printf("Message from HTTP: %q\n", msg)
		}
	}
}
