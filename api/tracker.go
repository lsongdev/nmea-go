// Package api keeps the live state of an NMEA 0183 receiver (position fix,
// satellites in view, recent sentences) and serves it over HTTP.
package api

import (
	"bufio"
	"context"
	"io"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lsongdev/nmea-go/nmea"
)

const (
	// logSize is how many recent sentences the tracker keeps.
	logSize = 500
	// satTTL is how long a satellite stays in view after its last GSV.
	satTTL = 10 * time.Second
	// usedTTL is how long a PRN counts as used after its last GSA.
	usedTTL = 3 * time.Second
)

// Fix is the receiver's latest navigation solution, merged from RMC, GGA,
// GNS, GSA, GST, VTG and ZDA.
type Fix struct {
	Valid    bool      `json:"valid"`
	Time     time.Time `json:"time"` // UTC from the receiver; zero until known
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	Altitude float64   `json:"altitude"`  // metres above mean sea level
	GeoidSep float64   `json:"geoid_sep"` // metres
	Speed    float64   `json:"speed"`     // knots
	SpeedKMH float64   `json:"speed_kmh"`
	Course   float64   `json:"course"` // degrees true
	Quality  int       `json:"quality"`
	Mode     int       `json:"mode"` // 1=no fix, 2=2D, 3=3D
	NumSats  int       `json:"num_sats"`
	PDOP     float64   `json:"pdop"`
	HDOP     float64   `json:"hdop"`
	VDOP     float64   `json:"vdop"`
	LatStd   float64   `json:"lat_std"` // metres, from GST
	LonStd   float64   `json:"lon_std"`
	AltStd   float64   `json:"alt_std"`
	Updated  time.Time `json:"updated"` // server clock when a position arrived
}

// Satellite is one satellite in view.
type Satellite struct {
	System    string    `json:"system"` // GPS, GLONASS, Galileo, BeiDou, QZSS, NavIC
	Talker    string    `json:"talker"`
	PRN       int       `json:"prn"`
	Elevation int       `json:"elevation"` // degrees
	Azimuth   int       `json:"azimuth"`   // degrees
	SNR       int       `json:"snr"`       // dB-Hz; 0 when not tracked
	Used      bool      `json:"used"`      // in the fix (GSA)
	Seen      time.Time `json:"seen"`
}

// Line is one received sentence.
type Line struct {
	Seq    uint64    `json:"seq"`
	Time   time.Time `json:"time"`
	Raw    string    `json:"raw"`
	Talker string    `json:"talker,omitempty"`
	Type   string    `json:"type,omitempty"`
	Error  string    `json:"error,omitempty"`
}

// Status describes the data source and sentence counters.
type Status struct {
	Source    string         `json:"source"`
	Connected bool           `json:"connected"`
	Error     string         `json:"error,omitempty"` // why the source is down
	Started   time.Time      `json:"started"`
	LastLine  time.Time      `json:"last_line"`
	Total     uint64         `json:"total"`
	Invalid   uint64         `json:"invalid"`
	Counts    map[string]int `json:"counts"` // by sentence type
	Messages  []string       `json:"messages"`
}

// Snapshot is the whole live state.
type Snapshot struct {
	Version    uint64      `json:"version"`
	Status     Status      `json:"status"`
	Fix        Fix         `json:"fix"`
	Satellites []Satellite `json:"satellites"`
}

type satKey struct {
	talker string
	prn    int
}

// usedKey is a satellite in a GSA; system is "" when the GSA has no
// system ID (before NMEA 4.10) and the PRN may belong to any system.
type usedKey struct {
	system string
	prn    int
}

// gsaSystems names the NMEA 4.10 GSA system IDs.
var gsaSystems = map[int]string{1: "GPS", 2: "GLONASS", 3: "Galileo", 4: "BeiDou", 5: "QZSS", 6: "NavIC"}

// Tracker merges parsed sentences into the live state. It is safe for
// concurrent use.
type Tracker struct {
	mu       sync.RWMutex
	version  uint64
	status   Status
	fix      Fix
	sats     map[satKey]Satellite
	used     map[usedKey]time.Time
	log      []Line // ring buffer
	next     int
	seq      uint64
	subs     map[chan Line]struct{}
	now      func() time.Time
	messages []string // recent TXT messages
}

// NewTracker returns an empty tracker for a source described by name.
func NewTracker(source string) *Tracker {
	return &Tracker{
		status: Status{Source: source, Started: time.Now(), Counts: map[string]int{}},
		sats:   map[satKey]Satellite{},
		used:   map[usedKey]time.Time{},
		subs:   map[chan Line]struct{}{},
		now:    time.Now,
	}
}

// Consume reads sentences from r until it fails or ctx ends. The source
// counts as connected while it runs.
func (t *Tracker) Consume(ctx context.Context, r io.Reader) error {
	t.setConnected(true, nil)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 4096)
	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if i := strings.IndexByte(line, '$'); i > 0 {
			line = line[i:] // skip leading noise on the line
		}
		if line == "" || line[0] != '$' {
			continue
		}
		t.Feed(line)
	}
	err := scanner.Err()
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = io.EOF
	}
	t.setConnected(false, err)
	return err
}

func (t *Tracker) setConnected(ok bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Connected = ok
	t.status.Error = ""
	if err != nil {
		t.status.Error = err.Error()
	}
	t.version++
}

// SetError records why the source cannot be opened.
func (t *Tracker) SetError(err error) { t.setConnected(false, err) }

// Feed parses one raw sentence and merges it into the state. It returns the
// parse error, which is also kept in the sentence log.
func (t *Tracker) Feed(raw string) error {
	s, err := nmea.Parse(raw)
	now := t.now()

	t.mu.Lock()
	t.seq++
	line := Line{Seq: t.seq, Time: now, Raw: raw}
	t.status.Total++
	t.status.LastLine = now
	if err != nil {
		line.Error = err.Error()
		t.status.Invalid++
	} else {
		line.Talker, line.Type = s.Talker(), s.Type()
		t.status.Counts[s.Type()]++
		t.apply(s, now)
	}
	if len(t.log) < logSize {
		t.log = append(t.log, line)
	} else {
		t.log[t.next] = line
	}
	t.next = (t.next + 1) % logSize
	t.version++
	subs := make([]chan Line, 0, len(t.subs))
	for ch := range t.subs {
		subs = append(subs, ch)
	}
	t.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- line:
		default: // a slow subscriber misses lines rather than blocking the source
		}
	}
	return err
}

// apply merges s into the state. Callers hold t.mu.
func (t *Tracker) apply(s nmea.Sentence, now time.Time) {
	f := &t.fix
	if full, ok := nmea.TimeFromSentence(s); ok {
		f.Time = full
	} else if tod, ok := nmea.TimeOfDay(s); ok && !f.Time.IsZero() {
		f.Time = nmea.MergeTime(f.Time, tod)
	}
	switch v := s.(type) {
	case nmea.RMC:
		f.Valid = v.Valid()
		if v.Valid() {
			f.Lat, f.Lon, f.Speed, f.Course = v.Lat, v.Lon, v.Speed, v.Course
			f.SpeedKMH = v.Speed * 1.852
			f.Updated = now
		}
	case nmea.GGA:
		f.Quality, f.NumSats, f.HDOP = v.Quality, v.NumSats, v.HDOP
		if v.Valid() {
			f.Lat, f.Lon, f.Altitude, f.GeoidSep = v.Lat, v.Lon, v.Altitude, v.GeoidSep
			f.Updated = now
		}
	case nmea.GNS:
		if v.Valid() {
			f.Lat, f.Lon, f.Altitude, f.GeoidSep = v.Lat, v.Lon, v.Altitude, v.GeoidSep
			f.NumSats, f.HDOP = v.NumSats, v.HDOP
			f.Updated = now
		}
	case nmea.GSA:
		f.Mode, f.PDOP, f.HDOP, f.VDOP = v.FixMode, v.PDOP, v.HDOP, v.VDOP
		for _, prn := range v.Satellites {
			t.used[usedKey{gsaSystems[v.SystemID], prn}] = now
		}
	case nmea.GSV:
		for _, sv := range v.Satellites {
			if sv.PRN == 0 {
				continue
			}
			t.sats[satKey{v.Talker(), sv.PRN}] = Satellite{
				System: System(v.Talker(), sv.PRN), Talker: v.Talker(), PRN: sv.PRN,
				Elevation: sv.Elevation, Azimuth: sv.Azimuth, SNR: sv.SNR, Seen: now,
			}
		}
	case nmea.VTG:
		f.Course, f.Speed, f.SpeedKMH = v.CourseTrue, v.SpeedKnots, v.SpeedKMH
	case nmea.GST:
		f.LatStd, f.LonStd, f.AltStd = v.LatStd, v.LonStd, v.AltStd
	case nmea.TXT:
		t.messages = append(t.messages, v.Text)
		if len(t.messages) > 20 {
			t.messages = t.messages[len(t.messages)-20:]
		}
	}
}

// Version changes whenever the state changes.
func (t *Tracker) Version() uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.version
}

// Snapshot returns a copy of the live state.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	st := t.status
	st.Counts = maps.Clone(t.status.Counts)
	st.Messages = append([]string{}, t.messages...)
	return Snapshot{Version: t.version, Status: st, Fix: t.fix, Satellites: t.satellites()}
}

// satellites returns the satellites in view, by system and PRN. Callers
// hold t.mu.
func (t *Tracker) satellites() []Satellite {
	now := t.now()
	out := []Satellite{}
	for _, s := range t.sats {
		if now.Sub(s.Seen) > satTTL {
			continue
		}
		for _, k := range [2]usedKey{{s.System, s.PRN}, {"", s.PRN}} {
			if seen, ok := t.used[k]; ok && now.Sub(seen) <= usedTTL {
				s.Used = true
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].System != out[j].System {
			return out[i].System < out[j].System
		}
		return out[i].PRN < out[j].PRN
	})
	return out
}

// Lines returns up to limit recent sentences after seq, oldest first,
// optionally only those of one type.
func (t *Tracker) Lines(after uint64, limit int, typ string) []Line {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := len(t.log)
	start := 0
	if n == logSize {
		start = t.next
	}
	out := []Line{}
	for i := range n {
		l := t.log[(start+i)%n]
		if l.Seq <= after || (typ != "" && l.Type != typ) {
			continue
		}
		out = append(out, l)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Subscribe returns a channel of every sentence received from now on, and
// a function that ends the subscription.
func (t *Tracker) Subscribe() (<-chan Line, func()) {
	ch := make(chan Line, 64)
	t.mu.Lock()
	t.subs[ch] = struct{}{}
	t.mu.Unlock()
	return ch, func() {
		t.mu.Lock()
		delete(t.subs, ch)
		t.mu.Unlock()
	}
}

// System names the constellation of a satellite from its GSV talker, or
// from its PRN when the talker is GN (combined).
func System(talker string, prn int) string {
	switch talker {
	case "GP":
		if prn >= 193 && prn <= 202 {
			return "QZSS"
		}
		if prn >= 33 && prn <= 64 {
			return "SBAS"
		}
		return "GPS"
	case "GL":
		return "GLONASS"
	case "GA":
		return "Galileo"
	case "GB", "BD":
		return "BeiDou"
	case "GQ":
		return "QZSS"
	case "GI":
		return "NavIC"
	}
	switch {
	case prn >= 1 && prn <= 32:
		return "GPS"
	case prn >= 33 && prn <= 64:
		return "SBAS"
	case prn >= 65 && prn <= 96:
		return "GLONASS"
	case prn >= 193 && prn <= 202:
		return "QZSS"
	case prn >= 201 && prn <= 263:
		return "BeiDou"
	case prn >= 301 && prn <= 336:
		return "Galileo"
	}
	return "Other"
}
