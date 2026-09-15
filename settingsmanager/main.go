// Command server implements a minimal JSON echo settings manager
// for the nginx reverse proxy demo. It accepts a JSON payload via
// POST, responds immediately, and reports completion of a
// simulated background job by calling back through nginx.
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// confirmationURL is the nginx endpoint settingsmanager calls once
// a submitted job has finished processing.
const confirmationURL = "http://127.0.0.1:8080/confirmations"

// backgroundJobDuration simulates the time a submitted job takes
// to complete.
const backgroundJobDuration = 5 * time.Second

// submitHandler handles POST requests containing a JSON body. It
// validates the payload, responds immediately with a "received"
// status, and starts a simulated background job that reports its
// result to confirmationURL once finished. Any method other than
// POST results in a 405 response.
func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed",
			http.StatusMethodNotAllowed)
		return
	}

	var payload interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json: "+err.Error(),
			http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":   "received",
		"received": payload,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode response: %v", err)
	}

	go runBackgroundJob(payload)
}

// runBackgroundJob simulates a long-running task and posts a
// completion confirmation to confirmationURL once the simulated
// work is done.
func runBackgroundJob(payload interface{}) {
	time.Sleep(backgroundJobDuration)

	confirmation := map[string]interface{}{
		"status":  "applied",
		"payload": payload,
	}

	body, err := json.Marshal(confirmation)
	if err != nil {
		log.Printf("marshal confirmation: %v", err)
		return
	}

	resp, err := http.Post(confirmationURL, "application/json",
		bytes.NewReader(body))
	if err != nil {
		log.Printf("post confirmation: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("confirmation sent, status %s", resp.Status)
}

// main starts the HTTP server on localhost:9000. It is meant to
// sit behind the nginx reverse proxy and should not be exposed
// directly to the network.
func main() {
	http.HandleFunc("/submit", submitHandler)

	addr := "127.0.0.1:9000"
	log.Printf("settingsmanager listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
