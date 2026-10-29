package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func sampleTracker(t *testing.T) *Tracker {
	t.Helper()
	f, err := os.Open("../testdata/sample.nmea")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr := NewTracker("sample")
	sc := bufio.NewScanner(f)
	for i := 0; i < 60 && sc.Scan(); i++ {
		tr.Feed(sc.Text())
	}
	return tr
}

func get(t *testing.T, h http.Handler, path string, v any) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if v != nil {
		if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
			t.Fatalf("GET %s: %v: %s", path, err, w.Body)
		}
	}
	return w
}

func TestState(t *testing.T) {
	h := NewHandler(sampleTracker(t), Options{})
	var snap Snapshot
	if w := get(t, h, "/state", &snap); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	f := snap.Fix
	if !f.Valid || f.Mode != 3 || f.Quality != 1 {
		t.Errorf("fix = %+v", f)
	}
	if f.Lat < 31.2 || f.Lat > 31.3 || f.Lon < 121.4 || f.Lon > 121.5 {
		t.Errorf("position = %v, %v", f.Lat, f.Lon)
	}
	if f.Time.Year() != 2026 || f.LatStd == 0 || f.PDOP == 0 {
		t.Errorf("time/accuracy = %v %v %v", f.Time, f.LatStd, f.PDOP)
	}
	systems := map[string]int{}
	used := 0
	for _, s := range snap.Satellites {
		systems[s.System]++
		if s.Used {
			used++
		}
	}
	if systems["GPS"] != 10 || systems["GLONASS"] != 6 || systems["Galileo"] != 5 || systems["BeiDou"] != 8 {
		t.Errorf("systems = %v", systems)
	}
	if used == 0 {
		t.Error("no satellite marked used")
	}
	if snap.Status.Counts["GSV"] == 0 || snap.Status.Total == 0 || len(snap.Status.Messages) != 2 {
		t.Errorf("status = %+v", snap.Status)
	}
}

func TestSentences(t *testing.T) {
	tr := sampleTracker(t)
	tr.Feed("$GPRMC,bad*FF")
	h := NewHandler(tr, Options{})
	var lines []Line
	get(t, h, "/sentences?limit=5", &lines)
	if len(lines) != 5 || lines[4].Error == "" {
		t.Fatalf("lines = %+v", lines)
	}
	get(t, h, "/sentences?type=rmc&limit=0", &lines)
	for _, l := range lines {
		if l.Type != "RMC" {
			t.Fatalf("type filter returned %+v", l)
		}
	}
	last := lines[len(lines)-1].Seq
	get(t, h, "/sentences?after="+strconv.FormatUint(last, 10), &lines)
	if len(lines) == 0 || lines[0].Seq <= last {
		t.Fatalf("after: %+v", lines)
	}
	if w := get(t, h, "/sentences?limit=x", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad limit: %d", w.Code)
	}
}

func TestLogWraps(t *testing.T) {
	tr := NewTracker("x")
	for range logSize + 10 {
		tr.Feed("$GPTXT,01,01,02,hello*2F")
	}
	lines := tr.Lines(0, 0, "")
	if len(lines) != logSize || lines[0].Seq != 11 || lines[len(lines)-1].Seq != logSize+10 {
		t.Fatalf("got %d lines, %d..%d", len(lines), lines[0].Seq, lines[len(lines)-1].Seq)
	}
}

func TestParse(t *testing.T) {
	h := NewHandler(NewTracker("x"), Options{})
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/parse", strings.NewReader(body)))
		return w
	}
	w := post(`{"sentence":"$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A"}`)
	var res struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != http.StatusOK || res.Type != "RMC" || res.Data["status"] != "A" || res.Data["time"] != "1994-03-23T12:35:19Z" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := post(`{"sentence":"$GPRMC,1*00"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad checksum: %d", w.Code)
	}
}

func TestToken(t *testing.T) {
	h := NewHandler(NewTracker("x"), Options{Token: "secret"})
	if w := get(t, h, "/status", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodGet, "/status", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("token: %d", w.Code)
	}
}

func TestEvents(t *testing.T) {
	tr := sampleTracker(t)
	srv := httptest.NewServer(NewHandler(tr, Options{Interval: 10 * time.Millisecond}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events?sentences=1", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(nil, 1<<20)
	seen := map[string]bool{}
	go func() {
		time.Sleep(50 * time.Millisecond)
		tr.Feed("$GPTXT,01,01,02,hello*2F")
	}()
	for sc.Scan() && !(seen["state"] && seen["sentence"]) {
		if ev, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
			seen[ev] = true
		}
	}
	if !seen["state"] || !seen["sentence"] {
		t.Fatalf("events seen: %v", seen)
	}
}

func TestConsume(t *testing.T) {
	tr := NewTracker("x")
	in := "noise\r\n\x00$GPTXT,01,01,02,hello*2F\r\n$GPGGA,bad\r\n"
	if err := tr.Consume(context.Background(), strings.NewReader(in)); err == nil {
		t.Fatal("Consume returned nil at EOF")
	}
	st := tr.Snapshot().Status
	if st.Total != 2 || st.Invalid != 1 || st.Connected || st.Error == "" {
		t.Fatalf("status = %+v", st)
	}
}

func TestUsedBySystem(t *testing.T) {
	tr := NewTracker("x")
	for _, body := range []string{
		"GPGSV,1,1,01,19,40,083,46",
		"GBGSV,1,1,01,19,30,120,40",
		"GNGSA,A,3,19,,,,,,,,,,,,1.0,1.0,1.0,1", // GPS 19 only
	} {
		var cs byte
		for i := range len(body) {
			cs ^= body[i]
		}
		if err := tr.Feed("$" + body + "*" + strings.ToUpper(strconv.FormatUint(uint64(cs)|0x100, 16)[1:])); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range tr.Snapshot().Satellites {
		if s.Used != (s.System == "GPS") {
			t.Errorf("%s %d used = %v", s.System, s.PRN, s.Used)
		}
	}
}
