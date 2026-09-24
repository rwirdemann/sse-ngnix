// Command server implements a minimal protobuf echo settings manager
// for the nginx reverse proxy demo. It accepts a ServiceConfig
// protobuf message via POST, tracks it as a transaction, and lets
// its confirmation be polled by transaction id.
package main

import (
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"settingsmanager-ngnix/proto/pb"
)

// protobufContentType is the media type used for request and
// response bodies carrying serialized protobuf messages.
const protobufContentType = "application/x-protobuf"

// applyDelay is how long a transaction stays in "received" before
// transitioning to "applied".
const applyDelay = 5 * time.Second

// transactionStore tracks the latest confirmation per transaction
// id. It is written once when a transaction is submitted and once
// more when it transitions to "applied".
type transactionStore struct {
	mu           sync.Mutex
	confirmation map[string]*pb.Confirmation
}

func newTransactionStore() *transactionStore {
	return &transactionStore{confirmation: make(map[string]*pb.Confirmation)}
}

func (s *transactionStore) set(confirmation *pb.Confirmation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmation[confirmation.GetTransactionId()] = confirmation
}

func (s *transactionStore) get(transactionID string) (*pb.Confirmation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	confirmation, ok := s.confirmation[transactionID]
	return confirmation, ok
}

var transactions = newTransactionStore()

// submitHandler handles POST requests containing a serialized
// ServiceConfig message. It registers the transaction as
// "received", responds with that Confirmation, and schedules its
// transition to "applied" after applyDelay. Any method other than
// POST results in a 405 response.
func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed",
			http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(),
			http.StatusBadRequest)
		return
	}

	var config pb.ServiceConfig
	if err := proto.Unmarshal(body, &config); err != nil {
		http.Error(w, "invalid protobuf: "+err.Error(),
			http.StatusBadRequest)
		return
	}

	log.Printf("received config: transaction_id=%s service_name=%s neonpulse_ids=%v message_type=%s neonpulse_id_header=%s",
		config.GetTransactionId(), config.GetServiceName(), config.GetNeonpulseIds(),
		r.Header.Get("X-Message-Type"), r.Header.Get("X-Neonpulse-Id"))

	response := confirmationFor(&config, pb.Status_received, "received")
	transactions.set(response)

	responseBody, err := proto.Marshal(response)
	if err != nil {
		http.Error(w, "marshal response: "+err.Error(),
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", protobufContentType)
	if _, err := w.Write(responseBody); err != nil {
		log.Printf("write response: %v", err)
	}

	go applyAfterDelay(&config)
}

// applyAfterDelay transitions a transaction to "applied" after
// applyDelay has passed.
func applyAfterDelay(config *pb.ServiceConfig) {
	time.Sleep(applyDelay)
	applied := confirmationFor(config, pb.Status_applied, "applied")
	transactions.set(applied)
	log.Printf("transaction applied: transaction_id=%s", applied.GetTransactionId())
}

// confirmationHandler handles GET requests polling for a
// transaction's current confirmation. The transaction id is
// expected as the last path segment. Unknown transactions result
// in a 404 response.
func confirmationHandler(w http.ResponseWriter, r *http.Request) {
	transactionID := r.PathValue("transactionID")
	if transactionID == "" {
		http.Error(w, "missing transactionID path segment",
			http.StatusBadRequest)
		return
	}

	confirmation, ok := transactions.get(transactionID)
	if !ok {
		http.Error(w, "unknown transaction", http.StatusNotFound)
		return
	}

	body, err := proto.Marshal(confirmation)
	if err != nil {
		http.Error(w, "marshal confirmation: "+err.Error(),
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", protobufContentType)
	if _, err := w.Write(body); err != nil {
		log.Printf("write confirmation: %v", err)
	}
}

// confirmationFor builds a Confirmation for the given config,
// carrying the first neonpulse id if any were submitted.
func confirmationFor(config *pb.ServiceConfig, status pb.Status, info string) *pb.Confirmation {
	var neonpulseID string
	if ids := config.GetNeonpulseIds(); len(ids) > 0 {
		neonpulseID = ids[0]
	}

	return &pb.Confirmation{
		CreatedAt:     timestamppb.Now(),
		TransactionId: config.GetTransactionId(),
		ServiceName:   config.GetServiceName(),
		NeonpulseId:   neonpulseID,
		Status:        status,
		Info:          info,
	}
}

// main starts the HTTP server on localhost:9000. It is meant to
// sit behind the nginx reverse proxy and should not be exposed
// directly to the network.
func main() {
	http.HandleFunc("/config/update", submitHandler)
	http.HandleFunc("GET /config/confirmations/{transactionID}", confirmationHandler)

	addr := "127.0.0.1:9000"
	log.Printf("settingsmanager listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
