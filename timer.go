// type Timer struct {
// 	State *TimerMap
// 	SysWg sync.WaitGroup

// 	Logger zerolog.Logger

// 	WebhookUrl string
// }

// func CreateTimer(WebhookUrl string, logDir string) Timer {
// 	state := TimerMap{}
// 	state.TimerMap = make(map[string]TimerMapValue)

// 	consoleLogger := zerolog.ConsoleWriter{
// 		Out:        os.Stderr,
// 		TimeFormat: time.RFC3339,
// 	}

// 	err := os.MkdirAll(logDir, 0o777)
// 	if err != nil {
// 		panic(err)
// 	}

// 	logRoot, err := os.OpenRoot(logDir)
// 	if err != nil {
// 		panic("Could not open log directory")
// 	}

// 	logFile := fmt.Sprintf("log-%d.json", time.Now().Unix())
// 	jsonLogFile, err := logRoot.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o777)
// 	if err != nil {
// 		panic("Could not open file")
// 	}

// 	multiLogger := zerolog.MultiLevelWriter(consoleLogger, jsonLogFile)

// 	return Timer{
// 		State:      &state,
// 		SysWg:      sync.WaitGroup{},
// 		WebhookUrl: WebhookUrl,

// 		Logger: zerolog.New(multiLogger).With().Timestamp().Caller().Logger(),
// 	}
// }

// func (t *Timer) RegisterHandler(w http.ResponseWriter, r *http.Request) {
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Msg("Something broke")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Could not read request body"))
// 		return
// 	}

// 	var request RegisterEvent
// 	err = json.Unmarshal(body, &request)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Msg("Request data not to format")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Request data not to format"))
// 		return
// 	}

// 	t.Logger.Info().Str("event_id", request.EventID).Int("timeout_secs", request.TimeoutSecs).
// 		Str("emit_msg", request.Emit).Msg("Received register request")

// 	if time.Duration(request.TimeoutSecs)*time.Second > MaxTimeout || request.TimeoutSecs <= 0 {
// 		t.Logger.Warn().Msg(fmt.Sprintf("Duration of %d is illegal!", request.TimeoutSecs))
// 		extendResponseBytes, err := json.Marshal(TimeoutResponse{
// 			EventID: request.EventID,
// 			Message: fmt.Sprintf("Illegal duration of %d seconds", request.TimeoutSecs),
// 		})
// 		if err != nil {
// 			t.Logger.Error().Err(err).Msg("Failed to marshal response")
// 			w.WriteHeader(http.StatusInternalServerError)
// 			_, _ = w.Write([]byte("Failed to marshal response"))
// 			return
// 		}

// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write(extendResponseBytes)
// 		return
// 	}

// 	// Fire off the goroutine

// 	t.State.Lock()

// 	_, ok := t.State.TimerMap[request.EventID]
// 	// this would mean that an existing event with
// 	// request.EventID already has a timer attached
// 	// with it
// 	if ok {
// 		t.Logger.Warn().Str("event_id", request.EventID).Msg("Existing timer attached with event")
// 		t.State.Unlock()
// 		// BUG:
// 		registerResponseBytes, _ := json.Marshal(TimeoutResponse{
// 			EventID: request.EventID,
// 			Message: fmt.Sprintf("Existing timer attached with event %s", request.EventID),
// 		})
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write(registerResponseBytes)
// 		return
// 	}

// 	timeInitiated := time.Now()

// 	cancelInstance := time.AfterFunc(time.Duration(request.TimeoutSecs)*time.Second, func() {
// 		t.State.Lock()
// 		defer t.State.Unlock()

// 		t.Logger.Info().Str("event_id", request.EventID).Int("timeout_secs", request.TimeoutSecs).
// 			Str("emit_msg", request.Emit).Str("webhook_url", t.WebhookUrl).
// 			Msg("Timer event firing to webhook")

// 		// Make the webhook call with emit
// 		response := TimeoutMessage{
// 			EventID:       request.EventID,
// 			Message:       request.Emit,
// 			TimeInitiated: timeInitiated.String(),
// 		}
// 		response_bytes, err := json.Marshal(response)
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 		}

// 		delete(t.State.TimerMap, request.EventID)

// 		// TODO better error handling and logging
// 		_, err = http.Post(t.WebhookUrl, "application/json", bytes.NewReader(response_bytes))
// 		if err != nil {
// 			t.Logger.Error().Err(err).Msg("Could not send response to webhook")
// 		}
// 		t.Logger.Info().Str("event_id", request.EventID).Str("webhook_url", t.WebhookUrl).
// 			Msg("Timer completed, sent to webhook")
// 	})

// 	t.State.TimerMap[request.EventID] = TimerMapValue{
// 		timer:    cancelInstance,
// 		duration: time.Duration(request.TimeoutSecs) * time.Second,
// 		initTime: timeInitiated,
// 	}
// 	t.State.Unlock()

// 	registerResponseBytes, _ := json.Marshal(TimeoutResponse{
// 		EventID: request.EventID,
// 		Message: fmt.Sprintf("Created event with event ID %s", request.EventID),
// 	})
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	_, _ = w.Write(registerResponseBytes)
// }

// func (t *Timer) CancelHandler(w http.ResponseWriter, r *http.Request) {
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Msg("Something broke")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Could not read request body"))
// 		return
// 	}
// 	var request CancelEvent
// 	err = json.Unmarshal(body, &request)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Msg("Request data not to format")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Request data not to format"))
// 		return
// 	}

// 	t.Logger.Info().Str("event_id", request.EventID).Msg("Received cancel request")

// 	t.State.Lock()

// 	cancelTimerHandle, ok := t.State.TimerMap[request.EventID]
// 	if !ok {
// 		t.State.Unlock()

// 		t.Logger.Error().Str("event_id", request.EventID).Msg("No event has been registered")

// 		cancelResponse, err := json.Marshal(&CancelResponse{
// 			EventID: request.EventID,
// 			Message: fmt.Sprintf("No event with event_id %s has been registered", request.EventID),
// 		})
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 		}

// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write(cancelResponse)
// 		return
// 	}

// 	timerStopRequest := cancelTimerHandle.timer.Stop()
// 	// If a stopped timer is still existing in the map,
// 	// not deleted .Stop() retunrs a false boolean value
// 	if !timerStopRequest {
// 		t.State.Unlock()
// 		// NOTE: Failed to get an event with EventID
// 		t.Logger.Warn().Str("event_id", request.EventID).Msg("Event has already been stopped")

// 		cancelResponse, err := json.Marshal(&CancelResponse{
// 			Message: fmt.Sprintf("Event with event_id %s has already been stopped", request.EventID),
// 		})
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 		}

// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write(cancelResponse)
// 		return
// 	} else {
// 		t.State.Unlock()
// 		delete(t.State.TimerMap, request.EventID)
// 	}

// 	t.Logger.Info().Str("event_id", request.EventID).Msg("Sucessfully cancelled event")
// 	cancelResponseBytes, err := json.Marshal(&CancelResponse{
// 		EventID: request.EventID,
// 		Message: fmt.Sprintf("Cancelled event with event_id %s", request.EventID),
// 	})
// 	if err != nil {
// 		t.Logger.Error().Err(err).Send()
// 	}

// 	w.Header().Add("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	_, _ = w.Write(cancelResponseBytes)
// }

// func (t *Timer) RemainingHandler(w http.ResponseWriter, r *http.Request) {
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Send()
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Failed to read body of request"))
// 		return
// 	}

// 	var request RemainingEvent
// 	err = json.Unmarshal(body, &request)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Send()
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Failed to unmarshal request json"))
// 		return
// 	}

// 	t.Logger.Info().Str("event_id", request.EventID).Msg("Received remaining request")

// 	t.State.Lock()

// 	if timerMapValue, ok := t.State.TimerMap[request.EventID]; !ok {
// 		t.State.Unlock()

// 		t.Logger.Error().Str("event_id", request.EventID).Msg("No associated timer found")

// 		remainingResponse, err := json.Marshal(RemainingResponse{
// 			EventID: request.EventID,
// 			Message: fmt.Sprintf("No associated timer with event_id %s", request.EventID),
// 		})
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 			w.WriteHeader(http.StatusInternalServerError)
// 			_, _ = w.Write([]byte("Failed to marshal response"))
// 			return
// 		}

// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write(remainingResponse)
// 	} else {
// 		// diff := time.Now().Sub(timerMapValue.initTime)
// 		diff := time.Since(timerMapValue.initTime)
// 		remaining := t.State.TimerMap[request.EventID].duration - diff

// 		t.State.Unlock()

// 		response := RemainingResponse{
// 			EventID:       request.EventID,
// 			TimeRemaining: remaining.String(),
// 			Message: fmt.Sprintf("Remaining time for event_id %s is %s",
// 				request.EventID,
// 				remaining.String(),
// 			),
// 		}

// 		responseBytes, err := json.Marshal(response)
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 			w.WriteHeader(http.StatusInternalServerError)
// 			_, _ = w.Write([]byte("Failed to marshal response"))
// 			return
// 		}

// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write(responseBytes)
// 	}
// }

// func (t *Timer) ExtendHandler(w http.ResponseWriter, r *http.Request) {
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Msg("Something broke")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Could not read request body"))
// 		return
// 	}

// 	var request ExtendEvent

// 	err = json.Unmarshal(body, &request)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Msg("Request data not to format")
// 		w.WriteHeader(http.StatusBadRequest)
// 		_, _ = w.Write([]byte("Request data not to format"))
// 		return
// 	}

// 	t.Logger.Info().Str("event_id", request.EventID).Int("timeout_secs", request.TimeoutSecs).
// 		Msg("Received extend request")

// 	t.State.Lock()

// 	// !ok for condition where no previous timer has
// 	// not been set
// 	if cancelTimerHandle, ok := t.State.TimerMap[request.EventID]; !ok {
// 		t.State.Unlock()

// 		t.Logger.Error().Str("event_id", request.EventID).Msg("No event has been registered")
// 		extendResponse, err := json.Marshal(&ExtendResponse{
// 			EventID: request.EventID,
// 			Message: fmt.Sprintf("No event with event_id %s has been registered", request.EventID),
// 		})
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 			w.WriteHeader(http.StatusInternalServerError)
// 			_, _ = w.Write([]byte("Failed to marshal response"))
// 			return
// 		}

// 		w.Header().Add("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusFailedDependency)
// 		_, _ = w.Write(extendResponse)
// 		return
// 	} else {
// 		diff := time.Since(cancelTimerHandle.initTime)
// 		remaining := t.State.TimerMap[request.EventID].duration - diff
// 		extraDuration := time.Duration(request.TimeoutSecs) * time.Second

// 		extendedDuration := remaining + extraDuration

// 		if extendedDuration > MaxTimeout || request.TimeoutSecs <= 0 {
// 			t.State.Unlock()
// 			t.Logger.Error().Int("duration", request.TimeoutSecs).Msg("Illegal Timeout Duration")
// 			extendResponseBytes, err := json.Marshal(ExtendResponse{
// 				EventID: request.EventID,
// 				Message: fmt.Sprintf("Illegal duration of %d seconds", request.TimeoutSecs),
// 			})
// 			if err != nil {
// 				t.Logger.Error().Err(err)
// 				w.WriteHeader(http.StatusInternalServerError)
// 				_, _ = w.Write([]byte("Failed to marshal response"))
// 				return
// 			}

// 			w.Header().Add("Content-Type", "application/json")
// 			w.WriteHeader(http.StatusBadRequest)
// 			_, _ = w.Write(extendResponseBytes)

// 			return
// 		}

// 		timeInitiated := time.Now()
// 		cancelTimerHandle.timer.Reset(extendedDuration)
// 		cancelTimerHandle.initTime = timeInitiated
// 		cancelTimerHandle.duration = extendedDuration

// 		// updating the state TimerMap
// 		t.State.TimerMap[request.EventID] = cancelTimerHandle

// 		t.State.Unlock()

// 		extendEventResponse, err := json.Marshal(&ExtendResponse{
// 			EventID: request.EventID,
// 			Message: fmt.Sprintf("Extended timer for event_id %s with duration %s",
// 				request.EventID,
// 				extendedDuration.String(),
// 			),
// 		})
// 		if err != nil {
// 			t.Logger.Error().Err(err).Send()
// 			w.WriteHeader(http.StatusInternalServerError)
// 			_, _ = w.Write([]byte("Failed to marshal response"))
// 			return
// 		}

// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write(extendEventResponse)
// 	}
// }

// func (t *Timer) WebhookTest(w http.ResponseWriter, r *http.Request) {
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Send()
// 		w.WriteHeader(http.StatusInternalServerError)
// 		_, _ = w.Write([]byte("Failed to read body of request"))
// 		return
// 	}

// 	var request TimeoutMessage
// 	err = json.Unmarshal(body, &request)
// 	if err != nil {
// 		t.Logger.Error().Err(err).Send()
// 		return
// 	}

// 	t.Logger.Info().Str("event_id", request.EventID).Str("message", request.Message).
// 		Msg("Received webhook request")
// }

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// // TimeoutMessage is the payload sent to the webhook when a timer fires.
// type TimeoutMessage struct {
// 	EventID       string `json:"event_id"`
// 	Message       string `json:"message"`
// 	TimeInitiated string `json:"time_initiated"`
// }

// // TimerMapValue holds the runtime data for a single timer.
// type TimerMapValue struct {
// 	timer    *time.Timer   // underlying timer (may be stopped)
// 	duration time.Duration // when running = total timeout from startedAt; when paused = remaining
// 	initTime time.Time     // non-zero when running (start time), zero when paused
// 	emit     string        // payload to send on fire
// }

// // TimerMap wraps the map with a mutex.
// type TimerMap struct {
// 	sync.Mutex
// 	TimerMap map[string]TimerMapValue
// }

// TimerResponse used for HTTP responses from timer handlers.
type TimerResponse struct {
	EventID     string `json:"event_id"`
	Status      string `json:"status,omitempty"`
	RemainingMS int64  `json:"remaining_ms,omitempty"`
	PausedAt    string `json:"paused_at,omitempty"`
	ResumedAt   string `json:"resumed_at,omitempty"`
	ExpectedEnd string `json:"expected_end_at,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Timer is the central object that manages timers and webhook delivery.
type Timer struct {
	State      *TimerMap
	SysWg      sync.WaitGroup
	Logger     zerolog.Logger
	WebhookUrl string
}

// CreateTimer constructs a Timer instance.
// logDir is accepted for compatibility; logger writes to stdout by default.
func CreateTimer(webhookUrl string, logDir string) Timer {
	// simple logger to stdout; if you want file logging, adapt here.
	zerolog.TimeFieldFormat = time.RFC3339
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	return Timer{
		State: &TimerMap{
			TimerMap: make(map[string]TimerMapValue),
		},
		SysWg:      sync.WaitGroup{},
		Logger:     logger,
		WebhookUrl: webhookUrl,
	}
}

// helper to write JSON responses
func (t *Timer) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// RegisterTimer registers and schedules a new timer.
func (t *Timer) RegisterTimer(eventID string, timeoutSeconds int64, emit string) (TimerResponse, int, error) {
	if eventID == "" || timeoutSeconds <= 0 {
		return TimerResponse{}, http.StatusBadRequest, nil
	}

	t.State.Lock()
	if _, exists := t.State.TimerMap[eventID]; exists {
		t.State.Unlock()
		return TimerResponse{}, http.StatusConflict, nil
	}

	duration := time.Duration(timeoutSeconds) * time.Second
	now := time.Now()
	// schedule AfterFunc
	mv := TimerMapValue{
		duration: duration,
		initTime: now,
		emit:     emit,
	}
	// create timer referencing closure capturing eventID
	mv.timer = time.AfterFunc(duration, func() {
		t.fire(eventID)
	})

	t.State.TimerMap[eventID] = mv
	t.State.Unlock()

	t.Logger.Info().Str("event_id", eventID).Dur("duration", duration).Msg("registered timer")
	return TimerResponse{
		EventID:     eventID,
		Status:      "running",
		RemainingMS: int64(duration / time.Millisecond),
	}, http.StatusOK, nil
}

// CancelTimer cancels and removes a timer.
func (t *Timer) CancelTimer(eventID string) (TimerResponse, int, error) {
	t.State.Lock()
	mv, ok := t.State.TimerMap[eventID]
	if !ok {
		t.State.Unlock()
		return TimerResponse{}, http.StatusNotFound, nil
	}
	// remove from map regardless of state
	delete(t.State.TimerMap, eventID)
	t.State.Unlock()

	// stop underlying timer if present
	if mv.timer != nil {
		mv.timer.Stop()
	}
	t.Logger.Info().Str("event_id", eventID).Msg("cancelled timer")
	return TimerResponse{
		EventID: eventID,
		Status:  "cancelled",
	}, http.StatusOK, nil
}

// RemainingTimer returns remaining time and status.
func (t *Timer) RemainingTimer(eventID string) (TimerResponse, int, error) {
	t.State.Lock()
	mv, ok := t.State.TimerMap[eventID]
	t.State.Unlock()
	if !ok {
		return TimerResponse{}, http.StatusNotFound, nil
	}

	// compute remaining
	var rem time.Duration
	if mv.initTime.IsZero() {
		// paused -> duration holds remaining
		rem = mv.duration
	} else {
		// running
		elapsed := time.Since(mv.initTime)
		rem = mv.duration - elapsed
		if rem < 0 {
			rem = 0
		}
	}
	status := "paused"
	if !mv.initTime.IsZero() {
		status = "running"
	}

	return TimerResponse{
		EventID:     eventID,
		Status:      status,
		RemainingMS: int64(rem / time.Millisecond),
	}, http.StatusOK, nil
}

// ExtendTimer extends remaining/duration depending on running/paused state.
func (t *Timer) ExtendTimer(eventID string, extraSeconds int64) (TimerResponse, int, error) {
	if extraSeconds <= 0 {
		return TimerResponse{}, http.StatusBadRequest, nil
	}

	t.State.Lock()
	mv, ok := t.State.TimerMap[eventID]
	if !ok {
		t.State.Unlock()
		return TimerResponse{}, http.StatusNotFound, nil
	}
	// operate on mv
	if mv.initTime.IsZero() {
		// paused: duration stores remaining
		mv.duration = mv.duration + time.Duration(extraSeconds)*time.Second
		t.State.TimerMap[eventID] = mv
		t.State.Unlock()
	} else {
		// running: compute remaining then reschedule with increased duration
		remaining := mv.duration - time.Since(mv.initTime)
		if remaining < 0 {
			remaining = 0
		}
		newDur := remaining + time.Duration(extraSeconds)*time.Second
		// stop old timer and reschedule
		if mv.timer != nil {
			mv.timer.Stop()
		}
		start := time.Now()
		mv.duration = newDur
		mv.initTime = start
		mv.timer = time.AfterFunc(newDur, func() {
			t.fire(eventID)
		})
		t.State.TimerMap[eventID] = mv
		t.State.Unlock()
	}

	// compute remaining to return
	t.State.Lock()
	mv2 := t.State.TimerMap[eventID]
	t.State.Unlock()

	var rem time.Duration
	if mv2.initTime.IsZero() {
		rem = mv2.duration
	} else {
		rem = mv2.duration - time.Since(mv2.initTime)
		if rem < 0 {
			rem = 0
		}
	}
	status := "paused"
	if !mv2.initTime.IsZero() {
		status = "running"
	}

	return TimerResponse{
		EventID:     eventID,
		Status:      status,
		RemainingMS: int64(rem / time.Millisecond),
	}, http.StatusOK, nil
}

// PauseTimer stops a running timer and stores remaining duration. Idempotent.
func (t *Timer) PauseTimer(eventID string) (TimerResponse, int, error) {
	t.State.Lock()
	mv, ok := t.State.TimerMap[eventID]
	if !ok {
		t.State.Unlock()
		return TimerResponse{}, http.StatusNotFound, nil
	}
	// if already paused
	if mv.initTime.IsZero() {
		remaining := mv.duration
		t.State.Unlock()
		return TimerResponse{
			EventID:     eventID,
			Status:      "paused",
			RemainingMS: int64(remaining / time.Millisecond),
			PausedAt:    time.Now().UTC().Format(time.RFC3339),
			Message:     "already paused",
		}, http.StatusOK, nil
	}
	// running -> stop timer and compute remaining
	// remove map lock while stopping? we keep it and update mv then store back.
	deletedMV := mv
	t.State.Unlock()

	// stop underlying timer
	if deletedMV.timer != nil {
		stopped := deletedMV.timer.Stop()
		_ = stopped // even if false, we will compute remaining
	}

	elapsed := time.Since(deletedMV.initTime)
	remaining := deletedMV.duration - elapsed
	if remaining < 0 {
		remaining = 0
	}

	// update map: store remaining in duration and zero initTime to mark paused
	t.State.Lock()
	newMV := t.State.TimerMap[eventID] // re-read in case changed
	newMV.duration = remaining
	newMV.initTime = time.Time{}
	// keep emit and timer (timer is stopped)
	t.State.TimerMap[eventID] = newMV
	t.State.Unlock()

	t.Logger.Info().Str("event_id", eventID).Dur("remaining", remaining).Msg("paused timer")
	return TimerResponse{
		EventID:     eventID,
		Status:      "paused",
		RemainingMS: int64(remaining / time.Millisecond),
		PausedAt:    time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK, nil
}

// ResumeTimer resumes a previously paused timer. Idempotent.
func (t *Timer) ResumeTimer(eventID string) (TimerResponse, int, error) {
	t.State.Lock()
	mv, ok := t.State.TimerMap[eventID]
	if !ok {
		t.State.Unlock()
		return TimerResponse{}, http.StatusNotFound, nil
	}
	// if already running
	if !mv.initTime.IsZero() {
		remaining := mv.duration - time.Since(mv.initTime)
		if remaining < 0 {
			remaining = 0
		}
		t.State.Unlock()
		return TimerResponse{
			EventID:     eventID,
			Status:      "running",
			RemainingMS: int64(remaining / time.Millisecond),
			ResumedAt:   mv.initTime.UTC().Format(time.RFC3339),
			ExpectedEnd: mv.initTime.Add(remaining).UTC().Format(time.RFC3339),
			Message:     "already running",
		}, http.StatusOK, nil
	}

	// paused state: mv.duration holds remaining
	remaining := mv.duration
	// if remaining <= 0 treat as fire immediately
	if remaining <= 0 {
		// delete and fire async
		delete(t.State.TimerMap, eventID)
		t.State.Unlock()

		go func(ev string, emit string) {
			t.Logger.Info().Str("event_id", ev).Msg("firing immediately on resume")
			payload := TimeoutMessage{
				EventID:       ev,
				Message:       emit,
				TimeInitiated: time.Now().UTC().Format(time.RFC3339),
			}
			if t.WebhookUrl != "" {
				b, _ := json.Marshal(payload)
				_, _ = http.Post(t.WebhookUrl, "application/json", bytes.NewReader(b))
			} else {
				log.Info().Msgf("timer fired: %+v", payload)
			}
		}(eventID, mv.emit)

		return TimerResponse{
			EventID: eventID,
			Status:  "done",
			Message: "timer completed",
		}, http.StatusOK, nil
	}

	// schedule a new timer or Reset existing one
	now := time.Now()
	if mv.timer == nil {
		mv.timer = time.AfterFunc(remaining, func() { t.fire(eventID) })
	} else {
		// reset existing timer to remaining
		mv.timer.Reset(remaining)
	}
	mv.initTime = now
	mv.duration = remaining

	// store back
	t.State.TimerMap[eventID] = mv
	t.State.Unlock()

	expected := now.Add(remaining).UTC().Format(time.RFC3339)
	t.Logger.Info().Str("event_id", eventID).Str("expected_end", expected).Msg("resumed timer")
	return TimerResponse{
		EventID:     eventID,
		Status:      "running",
		RemainingMS: int64(remaining / time.Millisecond),
		ResumedAt:   now.UTC().Format(time.RFC3339),
		ExpectedEnd: expected,
	}, http.StatusOK, nil
}

// fire is called when a timer expires. It deletes the timer entry and sends the webhook (or logs).
func (t *Timer) fire(eventID string) {
	// get and remove the entry
	t.State.Lock()
	mv, ok := t.State.TimerMap[eventID]
	if ok {
		delete(t.State.TimerMap, eventID)
	}
	t.State.Unlock()
	if !ok {
		return
	}

	// build payload
	payload := TimeoutMessage{
		EventID:       eventID,
		Message:       mv.emit,
		TimeInitiated: time.Now().UTC().Format(time.RFC3339),
	}

	// send webhook asynchronously (do not block timer goroutine)
	if t.WebhookUrl != "" {
		go func(p TimeoutMessage) {
			b, _ := json.Marshal(p)
			_, _ = http.Post(t.WebhookUrl, "application/json", bytes.NewReader(b))
		}(payload)
	} else {
		// log to stdout via zerolog
		t.Logger.Info().Str("event_id", eventID).Msgf("timer fired: %+v", payload)
	}
}

// ---- HTTP handler wrappers ----
// These handlers parse request bodies and delegate to the Timer methods.
// They are convenient if main.go wants to register them directly.

type registerReq struct {
	EventID        string `json:"event_id"`
	TimeoutSeconds int64  `json:"timeout_seconds"`
	Emit           string `json:"emit"`
}
type basicReq struct {
	EventID string `json:"event_id"`
}

func (t *Timer) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	resp, status, _ := t.RegisterTimer(req.EventID, req.TimeoutSeconds, req.Emit)
	t.writeJSON(w, status, resp)
}

func (t *Timer) CancelHandler(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	resp, status, _ := t.CancelTimer(req.EventID)
	t.writeJSON(w, status, resp)
}

func (t *Timer) RemainingHandler(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	resp, status, _ := t.RemainingTimer(req.EventID)
	t.writeJSON(w, status, resp)
}

func (t *Timer) ExtendHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventID        string `json:"event_id"`
		TimeoutSeconds int64  `json:"timeout_seconds"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	resp, status, _ := t.ExtendTimer(req.EventID, req.TimeoutSeconds)
	t.writeJSON(w, status, resp)
}

func (t *Timer) PauseHandler(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	resp, status, _ := t.PauseTimer(req.EventID)
	t.writeJSON(w, status, resp)
}

func (t *Timer) ResumeHandler(w http.ResponseWriter, r *http.Request) {
	var req basicReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	resp, status, _ := t.ResumeTimer(req.EventID)
	t.writeJSON(w, status, resp)
}
