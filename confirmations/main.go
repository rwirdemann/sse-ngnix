// Command confirmations implements a small service that bridges
// completed background jobs to waiting browsers. A client opens a
// long-lived GET /confirmations connection through nginx; once the
// backend POSTs the job result to the same path, that result is
// written into the open connection as a server-sent event.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

// broker fans out published confirmations to every currently
// open GET /confirmations connection.
type broker struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// newBroker returns a broker with no subscribers.
func newBroker() *broker {
	return &broker{subs: make(map[chan []byte]struct{})}
}

// subscribe registers a new subscriber and returns its channel.
func (b *broker) subscribe() chan []byte {
	ch := make(chan []byte, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes a subscriber. It must be called once the
// subscriber's connection ends, whether by delivery or disconnect.
func (b *broker) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// publish delivers data to every open subscriber. Subscribers
// that are not ready to receive are skipped rather than blocked.
func (b *broker) publish(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- data:
		default:
		}
	}
}

var confirmations = newBroker()

// confirmationsHandler dispatches by method: GET opens a
// server-sent events stream that waits for the next confirmation,
// POST publishes a confirmation to every waiting stream.
func confirmationsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		streamHandler(w, r)
	case http.MethodPost:
		publishHandler(w, r)
	default:
		http.Error(w, "method not allowed",
			http.StatusMethodNotAllowed)
	}
}

// streamHandler opens a server-sent events connection and blocks
// until either a confirmation is published or the client
// disconnects. It writes at most one event before returning,
// which ends the connection.
func streamHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported",
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := confirmations.subscribe()
	defer confirmations.unsubscribe(ch)

	select {
	case data := <-ch:
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	case <-r.Context().Done():
	}
}

// publishHandler reads a confirmation payload and hands it to
// every open stream. The body is forwarded as-is, so it must
// already be a single-line JSON document.
func publishHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(),
			http.StatusBadRequest)
		return
	}

	log.Printf("received confirmation: %s", body)
	confirmations.publish(body)
	w.WriteHeader(http.StatusOK)
}

// main starts the HTTP server on localhost:9001. It is meant to
// sit behind the nginx reverse proxy and should not be exposed
// directly to the network.
func main() {
	http.HandleFunc("/confirmations", confirmationsHandler)

	addr := "127.0.0.1:9001"
	log.Printf("confirmations service listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
