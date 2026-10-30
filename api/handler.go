package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lsongdev/nmea-go/nmea"
)

const maxBodySize = 64 << 10

// Options configures the HTTP handler.
type Options struct {
	// Token, when set, must be sent as "Authorization: Bearer <token>" on
	// every request. Hosts that authenticate requests themselves leave it
	// empty.
	Token string
	// Interval is how often /events sends a changed state; zero is 500ms.
	Interval time.Duration
}

// Handler serves the API with relative routes ("/state", "/sentences",
// ...), so it can be mounted under any prefix.
type Handler struct {
	tracker *Tracker
	opts    Options
	mux     *http.ServeMux
}

// NewHandler returns the API handler for t.
func NewHandler(t *Tracker, opts Options) *Handler {
	if opts.Interval <= 0 {
		opts.Interval = 500 * time.Millisecond
	}
	h := &Handler{tracker: t, opts: opts, mux: http.NewServeMux()}
	routes := map[string]func(*http.Request) (any, error){
		"GET /state":      h.state,
		"GET /status":     h.status,
		"GET /fix":        h.fix,
		"GET /satellites": h.satellites,
		"GET /sentences":  h.sentences,
		"POST /parse":     h.parse,
	}
	for pattern, fn := range routes {
		h.mux.Handle(pattern, endpoint(fn))
	}
	h.mux.HandleFunc("GET /events", h.events)
	h.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "no such endpoint"})
	})
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="nmea-go"`)
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized"})
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) authorized(r *http.Request) bool {
	if h.opts.Token == "" {
		return true
	}
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(got), []byte(h.opts.Token)) == 1
}

type errorBody struct {
	Error string `json:"error"`
}

type badRequestError struct{ msg string }

func (e *badRequestError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return &badRequestError{fmt.Sprintf(format, args...)}
}

// endpoint adapts a handler returning (value, error) to JSON responses.
func endpoint(fn func(*http.Request) (any, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		v, err := fn(r)
		if err != nil {
			status := http.StatusInternalServerError
			if _, ok := errors.AsType[*badRequestError](err); ok {
				status = http.StatusBadRequest
			}
			writeJSON(w, status, errorBody{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, v)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func (h *Handler) state(*http.Request) (any, error) { return h.tracker.Snapshot(), nil }

func (h *Handler) status(*http.Request) (any, error) { return h.tracker.Snapshot().Status, nil }

func (h *Handler) fix(*http.Request) (any, error) { return h.tracker.Snapshot().Fix, nil }

func (h *Handler) satellites(*http.Request) (any, error) {
	return h.tracker.Snapshot().Satellites, nil
}

// sentences lists recent sentences: ?after=<seq>&limit=<n>&type=<RMC>.
func (h *Handler) sentences(r *http.Request) (any, error) {
	q := r.URL.Query()
	after, err := uintParam(q.Get("after"))
	if err != nil {
		return nil, badRequest("after: %v", err)
	}
	limit, err := uintParam(q.Get("limit"))
	if err != nil {
		return nil, badRequest("limit: %v", err)
	}
	if limit == 0 {
		limit = 100
	}
	return h.tracker.Lines(after, int(limit), strings.ToUpper(q.Get("type"))), nil
}

func uintParam(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseUint(s, 10, 64)
}

type parseRequest struct {
	Sentence string `json:"sentence"`
}

type parseResult struct {
	Talker string `json:"talker"`
	Type   string `json:"type"`
	Raw    string `json:"raw"`
	Data   any    `json:"data"` // the parsed nmea struct
}

// parse decodes one sentence without feeding it to the tracker.
func (h *Handler) parse(r *http.Request) (any, error) {
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, badRequest("invalid JSON body: %v", err)
	}
	s, err := nmea.Parse(req.Sentence)
	if err != nil {
		return nil, badRequest("%v", err)
	}
	return parseResult{Talker: s.Talker(), Type: s.Type(), Raw: s.Raw(), Data: s}, nil
}

// events streams server-sent events: "state" with the whole snapshot when
// it changes (at most every Interval), and with ?sentences=1 a "sentence"
// event per received line.
func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "streaming unsupported"})
		return
	}
	var lines <-chan Line
	if v := r.URL.Query().Get("sentences"); v != "" && v != "0" && v != "false" {
		ch, cancel := h.tracker.Subscribe()
		defer cancel()
		lines = ch
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		return err
	}
	var sent uint64
	sendState := func() error {
		snap := h.tracker.Snapshot()
		if snap.Version == sent {
			return nil
		}
		sent = snap.Version
		return send("state", snap)
	}
	if err := sendState(); err != nil {
		return
	}
	flusher.Flush()

	tick := time.NewTicker(h.opts.Interval)
	defer tick.Stop()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	for {
		var err error
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			err = sendState()
		case <-keepalive.C:
			_, err = fmt.Fprint(w, ": keepalive\n\n")
		case l := <-lines:
			err = send("sentence", l)
		}
		if err != nil {
			return
		}
		flusher.Flush()
	}
}
