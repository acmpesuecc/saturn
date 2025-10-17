// package main

// import (
// 	"encoding/json"
// 	"flag"
// 	"fmt"

// 	"net/http"
// 	"sync"
// 	"time"

// 	"bytes"
// 	"io"
// 	"log"
// 	"os"

// )

// var

// func main() {

// 	WEBHOOK_URL := flag.String("webhook_url", "http://localhost:3000/webhook", "where do you want your emitted event to go?")
// 	LOG_DIR_JSON := flag.String("log_dir", "./logs/", "directory for logger")
// 	flag.Parse()

// 	mux := http.NewServeMux()
// 	timer := CreateTimer(*WEBHOOK_URL, *LOG_DIR_JSON)

// 	timer.Logger.Info().Msg(fmt.Sprintf("Sending events on webhook URL %s", *WEBHOOK_URL))

// 	// System goroutine;
// 	timer.SysWg.Add(1)

// 	// Handle when a user tries to register a timeout
// 	// Spawn the goroutine
// 	mux.HandleFunc("POST /register", timer.RegisterHandler)
// 	mux.HandleFunc("POST /cancel", timer.CancelHandler)
// 	mux.HandleFunc("POST /remaining", timer.RemainingHandler)
// 	mux.HandleFunc("POST /extend", timer.ExtendHandler)

// 	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
// 		// log.Println("Received GET /test")
// 		timer.Logger.Info().Msg("Received GET /test")
// 		response := struct {
// 			Time int64  `json:"time"`
// 			Data string `json:"data"`
// 		}{
// 			Time: time.Now().Unix(),
// 			Data: fmt.Sprintf("saturn is up: %d timer(s) running", len(timer.State.TimerMap)),
// 		}

// 		responseBytes, _ := json.Marshal(response)
// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write(responseBytes)
// 	})

// 	// placeholder for the webhook
// 	mux.HandleFunc("POST /webhook", timer.WebhookTest)

// 	var systemwg sync.WaitGroup
// 	systemwg.Add(2)

// 	// Spawn server goroutine
// 	go func() {
// 		timer.Logger.Info().Msg("Starting Server...")
// 		err := http.ListenAndServe(":3000", mux)
// 		if err != nil {
// 			// log.Println(fmt.Errorf("Error in server -> %v", err))
// 			timer.Logger.Error().Msg(fmt.Sprintf("Error in server: %v", err))
// 			defer systemwg.Done()
// 		}
// 	}()

// 	// Spawn timer goroutine
// 	go func() {
// 		defer systemwg.Done()
// 		timer.SysWg.Wait()
// 	}()

// 	systemwg.Wait()
// }

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var webhookURL string

func init() {
	webhookURL = os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		// fallback to CLI flag or hardcode if you prefer; for demo we keep empty meaning no external POST
		webhookURL = ""
	}
}

type registerReq struct {
	EventID        string `json:"event_id"`
	TimeoutSeconds int64  `json:"timeout_seconds"`
	Emit           string `json:"emit"`
}

type basicReq struct {
	EventID string `json:"event_id"`
}

type timerResponse struct {
	EventID     string `json:"event_id"`
	Status      string `json:"status"`
	RemainingMS int64  `json:"remaining_ms,omitempty"`
	PausedAt    string `json:"paused_at,omitempty"`
	ResumedAt   string `json:"resumed_at,omitempty"`
	ExpectedEnd string `json:"expected_end_at,omitempty"`
	Message     string `json:"message,omitempty"`
}

type TimerEntry struct {
	eventID string
	emit    string

	mu       sync.Mutex
	timer    *time.Timer
	duration time.Duration // original timeout
	// When running: startedAt is set, remaining is ignored (0).
	// When paused: remaining set, startedAt zero.
	startedAt time.Time
	remaining time.Duration
	status    string // "running","paused","cancelled","done"
}

var (
	timers   = make(map[string]*TimerEntry)
	timersMu sync.Mutex
)

func main() {
	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/cancel", handleCancel)
	http.HandleFunc("/remaining", handleRemaining)
	http.HandleFunc("/extend", handleExtend)
	http.HandleFunc("/pause", handlePause)
	http.HandleFunc("/resume", handleResume)

	addr := ":3000"
	log.Printf("starting server at %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.EventID == "" || req.TimeoutSeconds <= 0 {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	timersMu.Lock()
	if _, exists := timers[req.EventID]; exists {
		timersMu.Unlock()
		http.Error(w, "event already exists", http.StatusConflict)
		return
	}

	te := &TimerEntry{
		eventID:  req.EventID,
		emit:     req.Emit,
		duration: time.Duration(req.TimeoutSeconds) * time.Second,
		status:   "running",
	}
	te.startedAt = time.Now()
	// schedule
	te.timer = time.AfterFunc(te.duration, func() {
		fireTimer(te)
	})

	timers[req.EventID] = te
	timersMu.Unlock()

	writeJSON(w, http.StatusOK, timerResponse{
		EventID:     req.EventID,
		Status:      "running",
		RemainingMS: int64(te.duration / time.Millisecond),
	})
}

func handleCancel(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	timersMu.Lock()
	te, ok := timers[req.EventID]
	if !ok {
		timersMu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	timersMu.Unlock()

	te.mu.Lock()
	defer te.mu.Unlock()
	if te.status == "done" || te.status == "cancelled" {
		writeJSON(w, http.StatusOK, timerResponse{
			EventID: req.EventID,
			Status:  te.status,
			Message: "already finished or cancelled",
		})
		return
	}
	if te.timer != nil {
		te.timer.Stop()
	}
	te.status = "cancelled"
	writeJSON(w, http.StatusOK, timerResponse{
		EventID: req.EventID,
		Status:  "cancelled",
	})
}

func handleRemaining(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	timersMu.Lock()
	te, ok := timers[req.EventID]
	timersMu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	te.mu.Lock()
	defer te.mu.Unlock()

	var rem time.Duration
	switch te.status {
	case "running":
		elapsed := time.Since(te.startedAt)
		rem = te.duration - elapsed
		if rem < 0 {
			rem = 0
		}
	case "paused":
		rem = te.remaining
	default:
		rem = 0
	}

	writeJSON(w, http.StatusOK, timerResponse{
		EventID:     req.EventID,
		Status:      te.status,
		RemainingMS: int64(rem / time.Millisecond),
	})
}

func handleExtend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventID        string `json:"event_id"`
		TimeoutSeconds int64  `json:"timeout_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.EventID == "" || req.TimeoutSeconds <= 0 {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	timersMu.Lock()
	te, ok := timers[req.EventID]
	timersMu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	te.mu.Lock()
	defer te.mu.Unlock()
	if te.status == "done" || te.status == "cancelled" {
		http.Error(w, "cannot extend finished timer", http.StatusConflict)
		return
	}

	newDur := time.Duration(req.TimeoutSeconds) * time.Second
	if te.status == "running" {
		// stop existing, compute remaining from now then add newDur
		if te.timer != nil {
			te.timer.Stop()
		}
		elapsed := time.Since(te.startedAt)
		rem := te.duration - elapsed
		if rem < 0 {
			rem = 0
		}
		te.duration = rem + newDur
		te.startedAt = time.Now()
		te.timer = time.AfterFunc(te.duration, func() { fireTimer(te) })
	} else if te.status == "paused" {
		te.remaining = te.remaining + newDur
	} else {
		http.Error(w, "invalid timer state", http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusOK, timerResponse{
		EventID:     req.EventID,
		Status:      te.status,
		RemainingMS: int64(getRemainingLocked(te) / time.Millisecond),
	})
}

func handlePause(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	timersMu.Lock()
	te, ok := timers[req.EventID]
	timersMu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	te.mu.Lock()
	defer te.mu.Unlock()

	// idempotent: if already paused, return current state
	if te.status == "paused" {
		writeJSON(w, http.StatusOK, timerResponse{
			EventID:     req.EventID,
			Status:      "paused",
			RemainingMS: int64(te.remaining / time.Millisecond),
			PausedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if te.status == "done" || te.status == "cancelled" {
		http.Error(w, "cannot pause finished or cancelled timer", http.StatusConflict)
		return
	}

	// if running -> compute remaining and stop underlying timer
	if te.status == "running" {
		if te.timer != nil {
			te.timer.Stop()
		}
		elapsed := time.Since(te.startedAt)
		rem := te.duration - elapsed
		if rem < 0 {
			rem = 0
		}
		te.remaining = rem
		te.startedAt = time.Time{}
		te.status = "paused"
		writeJSON(w, http.StatusOK, timerResponse{
			EventID:     req.EventID,
			Status:      "paused",
			RemainingMS: int64(te.remaining / time.Millisecond),
			PausedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	// any other state
	http.Error(w, "invalid timer state", http.StatusConflict)
}

func handleResume(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	timersMu.Lock()
	te, ok := timers[req.EventID]
	timersMu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	te.mu.Lock()
	defer te.mu.Unlock()

	// idempotent: if already running, return current state
	if te.status == "running" {
		writeJSON(w, http.StatusOK, timerResponse{
			EventID:     req.EventID,
			Status:      "running",
			RemainingMS: int64(getRemainingLocked(te) / time.Millisecond),
			ResumedAt:   te.startedAt.UTC().Format(time.RFC3339),
			ExpectedEnd: te.startedAt.Add(getRemainingLocked(te)).UTC().Format(time.RFC3339),
		})
		return
	}

	if te.status == "done" || te.status == "cancelled" {
		http.Error(w, "cannot resume finished or cancelled timer", http.StatusConflict)
		return
	}

	if te.status == "paused" {
		// if remaining <= 0, fire immediately
		if te.remaining <= 0 {
			te.status = "done"
			// fire synchronously
			go fireTimer(te)
			writeJSON(w, http.StatusOK, timerResponse{
				EventID:     req.EventID,
				Status:      "done",
				RemainingMS: 0,
				ResumedAt:   time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		// schedule AfterFunc for remaining
		te.startedAt = time.Now()
		te.status = "running"
		rem := te.remaining
		te.timer = time.AfterFunc(rem, func() { fireTimer(te) })
		// reset remaining because now running
		te.remaining = 0
		writeJSON(w, http.StatusOK, timerResponse{
			EventID:     req.EventID,
			Status:      "running",
			RemainingMS: int64(rem / time.Millisecond),
			ResumedAt:   te.startedAt.UTC().Format(time.RFC3339),
			ExpectedEnd: te.startedAt.Add(rem).UTC().Format(time.RFC3339),
		})
		return
	}

	http.Error(w, "invalid timer state", http.StatusConflict)
}

func fireTimer(te *TimerEntry) {
	// lock the timer entry to set status and prevent races
	te.mu.Lock()
	// if cancelled or already done, do nothing
	if te.status == "cancelled" || te.status == "done" {
		te.mu.Unlock()
		return
	}
	te.status = "done"
	te.mu.Unlock()

	// send webhook if configured
	payload := map[string]string{
		"event_id":       te.eventID,
		"message":        te.emit,
		"time_initiated": time.Now().UTC().Format(time.RFC3339),
	}
	if webhookURL != "" {
		data, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		// don't block if webhook fails; best effort
		go func() {
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("webhook post failed: %v", err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	} else {
		// for local demo: just log
		log.Printf("timer fired: %v", payload)
	}
}

func getRemainingLocked(te *TimerEntry) time.Duration {
	// caller must hold te.mu
	if te.status == "running" {
		elapsed := time.Since(te.startedAt)
		rem := te.duration - elapsed
		if rem < 0 {
			return 0
		}
		return rem
	}
	if te.status == "paused" {
		if te.remaining < 0 {
			return 0
		}
		return te.remaining
	}
	return 0
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
