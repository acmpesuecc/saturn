package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPauseResumeFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/pause", handlePause)
	mux.HandleFunc("/resume", handleResume)
	mux.HandleFunc("/remaining", handleRemaining)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// register 2 second timer
	res, err := http.Post(ts.URL+"/register", "application/json",
		strings.NewReader(`{"event_id":"test1","timeout_seconds":2,"emit":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	// pause immediately
	res, err = http.Post(ts.URL+"/pause", "application/json",
		strings.NewReader(`{"event_id":"test1"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	// remaining should report paused
	res, err = http.Post(ts.URL+"/remaining", "application/json",
		strings.NewReader(`{"event_id":"test1"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	if r["status"] != "paused" {
		t.Fatalf("expected paused, got %v", r["status"])
	}

	// resume and allow to fire
	res, err = http.Post(ts.URL+"/resume", "application/json",
		strings.NewReader(`{"event_id":"test1"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	time.Sleep(3 * time.Second) // wait beyond original timeout
}
