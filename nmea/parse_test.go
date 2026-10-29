package nmea

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

// sum frames a sentence body as "$<body>*<checksum>".
func sum(body string) string {
	var cs byte
	for i := 0; i < len(body); i++ {
		cs ^= body[i]
	}
	return fmt.Sprintf("$%s*%02X", body, cs)
}

func mustParse(t *testing.T, raw string) Sentence {
	t.Helper()
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	return s
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestRMC(t *testing.T) {
	s := mustParse(t, "$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A").(RMC)
	if s.Talker() != "GP" || s.Type() != "RMC" || !s.Valid() {
		t.Fatalf("%+v", s)
	}
	if want := time.Date(1994, 3, 23, 12, 35, 19, 0, time.UTC); !s.Time.Equal(want) {
		t.Errorf("time = %v, want %v", s.Time, want)
	}
	if !near(s.Lat, 48.1173) || !near(s.Lon, 11.516666666) {
		t.Errorf("position = %v %v", s.Lat, s.Lon)
	}
	if s.Speed != 22.4 || s.Course != 84.4 || s.MagVar != -3.1 {
		t.Errorf("speed %v course %v magvar %v", s.Speed, s.Course, s.MagVar)
	}

	s = mustParse(t, sum("GNRMC,080000.25,A,3113.82400,S,12128.42200,W,0.1,,291026,,,A,V")).(RMC)
	if want := time.Date(2026, 10, 29, 8, 0, 0, 250e6, time.UTC); !s.Time.Equal(want) {
		t.Errorf("time = %v, want %v", s.Time, want)
	}
	if s.Lat > 0 || s.Lon > 0 || s.Mode != "A" || s.NavStatus != "V" || s.MagVar != 0 {
		t.Errorf("%+v", s)
	}

	s = mustParse(t, sum("GPRMC,000001,V,,,,,,,,,,N")).(RMC)
	if s.Valid() || s.Time.Year() != 0 || s.Time.Second() != 1 {
		t.Errorf("void RMC without date: %+v", s)
	}
}

func TestGGA(t *testing.T) {
	s := mustParse(t, "$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47").(GGA)
	if !s.Valid() || s.NumSats != 8 || s.HDOP != 0.9 || s.Altitude != 545.4 || s.GeoidSep != 46.9 {
		t.Fatalf("%+v", s)
	}
	if s.Time.Hour() != 12 || s.Time.Minute() != 35 || s.Time.Second() != 19 {
		t.Errorf("time = %v", s.Time)
	}
	// Midnight is a real time of day, not "no time".
	s = mustParse(t, sum("GPGGA,000000,,,,,0,00,,,M,,M,,")).(GGA)
	if s.Time.IsZero() || s.Valid() {
		t.Errorf("midnight: %+v", s)
	}
}

func TestGSA(t *testing.T) {
	s := mustParse(t, "$GPGSA,A,3,04,05,,09,12,,,24,,,,,2.5,1.3,2.1*39").(GSA)
	if !s.Valid() || fmt.Sprint(s.Satellites) != "[4 5 9 12 24]" || s.PDOP != 2.5 || s.VDOP != 2.1 || s.SystemID != 0 {
		t.Fatalf("%+v", s)
	}
	s = mustParse(t, sum("GNGSA,A,3,01,03,,,,,,,,,,,1.2,0.8,0.9,4")).(GSA)
	if s.SystemID != 4 {
		t.Errorf("system id = %d", s.SystemID)
	}
}

func TestGSV(t *testing.T) {
	s := mustParse(t, "$GPGSV,2,1,08,01,40,083,46,02,17,308,41,12,07,344,39,14,22,228,45*75").(GSV)
	if s.NumMsg != 2 || s.MsgNum != 1 || s.NumSats != 8 || len(s.Satellites) != 4 {
		t.Fatalf("%+v", s)
	}
	if s.Satellites[3] != (Satellite{PRN: 14, Elevation: 22, Azimuth: 228, SNR: 45}) {
		t.Errorf("sat = %+v", s.Satellites[3])
	}
	s = mustParse(t, sum("GPGSV,1,1,02,05,35,120,40,13,12,160,,1")).(GSV)
	if len(s.Satellites) != 2 || s.SignalID != 1 || s.Satellites[1].SNR != 0 {
		t.Errorf("signal id: %+v", s)
	}
	s = mustParse(t, sum("GPGSV,1,1,01,05,35,120,40,,,,,,,,")).(GSV)
	if len(s.Satellites) != 1 {
		t.Errorf("padding: %+v", s.Satellites)
	}
	s = mustParse(t, sum("GPGSV,1,1,00")).(GSV)
	if len(s.Satellites) != 0 {
		t.Errorf("empty: %+v", s)
	}
}

func TestOthers(t *testing.T) {
	vtg := mustParse(t, sum("GPVTG,054.7,T,034.4,M,005.5,N,010.2,K,A")).(VTG)
	if vtg.CourseTrue != 54.7 || vtg.CourseMagnetic != 34.4 || vtg.SpeedKnots != 5.5 || vtg.SpeedKMH != 10.2 || vtg.Mode != "A" {
		t.Errorf("VTG %+v", vtg)
	}
	gll := mustParse(t, sum("GPGLL,4916.45,N,12311.12,W,225444,A,A")).(GLL)
	if !gll.Valid() || !near(gll.Lat, 49.274166666) || !near(gll.Lon, -123.18533333) || gll.Time.Hour() != 22 {
		t.Errorf("GLL %+v", gll)
	}
	zda := mustParse(t, sum("GPZDA,201530.00,04,07,2002,-05,00")).(ZDA)
	if want := time.Date(2002, 7, 4, 20, 15, 30, 0, time.UTC); !zda.Time.Equal(want) || zda.LocalZoneHours != -5 {
		t.Errorf("ZDA %+v", zda)
	}
	txt := mustParse(t, sum("GPTXT,01,01,02,u-blox ag - www.u-blox.com")).(TXT)
	if txt.Text != "u-blox ag - www.u-blox.com" || txt.MsgType != 2 {
		t.Errorf("TXT %+v", txt)
	}
	gns := mustParse(t, sum("GNGNS,014035.00,4332.69262,S,17235.48549,E,RR,13,0.9,25.63,11.24,,")).(GNS)
	if !gns.Valid() || gns.Mode != "RR" || gns.NumSats != 13 || gns.Lat > 0 {
		t.Errorf("GNS %+v", gns)
	}
	if gns := mustParse(t, sum("GNGNS,014035.00,,,,,NN,00,,,,,")).(GNS); gns.Valid() {
		t.Errorf("GNS NN is valid")
	}
	gst := mustParse(t, sum("GPGST,172814.0,0.006,0.023,0.020,273.6,0.023,0.020,0.031")).(GST)
	if gst.LatStd != 0.023 || gst.AltStd != 0.031 || gst.Orient != 273.6 {
		t.Errorf("GST %+v", gst)
	}
}

func TestUnknownAndProprietary(t *testing.T) {
	u := mustParse(t, sum("GPXTE,A,A,0.67,L,N")).(UnknownSentence)
	if u.Type() != "XTE" || len(u.Fields) != 5 {
		t.Errorf("%+v", u)
	}
	p := mustParse(t, sum("PUBX,00,081350.00,4717.113210,N")).(UnknownSentence)
	if p.Talker() != "P" || p.Type() != "UBX" || p.Fields[0] != "00" {
		t.Errorf("%+v", p)
	}
	p = mustParse(t, sum("PMTK001,604,3")).(UnknownSentence)
	if p.Type() != "MTK001" {
		t.Errorf("%+v", p)
	}
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want error
	}{
		{"", ErrFormat},
		{"GPGGA,1*00", ErrFormat},
		{"$GPGGA,123519", ErrChecksum},
		{"$GPGGA,123519*0", ErrChecksum},
		{"$GPGGA,123519*ZZ", ErrChecksum},
		{"$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6B", ErrChecksum},
		{sum("GP"), ErrFormat},
		{sum("GPGGA,123519,4807.038,N"), ErrFormat}, // truncated
		{sum("GPRMC,123519,A"), ErrFormat},
	} {
		if _, err := Parse(tc.raw); !errors.Is(err, tc.want) {
			t.Errorf("Parse(%q) = %v, want %v", tc.raw, err, tc.want)
		}
	}
	// Lower-case checksums and surrounding whitespace are accepted.
	if _, err := Parse("  $GPGSA,A,3,04,05,,09,12,,,24,,,,,2.5,1.3,2.1*39\r\n"); err != nil {
		t.Error(err)
	}
	if _, err := Parse("$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6a"); err != nil {
		t.Error(err)
	}
}

func TestParseHelpers(t *testing.T) {
	if v := parseLatLon("4807", "N"); !near(v, 48.116666666) {
		t.Errorf("no decimals: %v", v)
	}
	if v := parseLatLon("00030.0", "W"); !near(v, -0.5) {
		t.Errorf("small: %v", v)
	}
	if v := parseLatLon("x", "N"); v != 0 {
		t.Errorf("garbage: %v", v)
	}
	for in, want := range map[string]int{"010180": 1980, "311299": 1999, "010100": 2000, "291026": 2026} {
		if y, _, _, ok := parseDate(in); !ok || y != want {
			t.Errorf("parseDate(%s) = %d %v, want %d", in, y, ok, want)
		}
	}
	for _, in := range []string{"", "1235", "12a519", "250000", "126000", "123519x", "123519.5x"} {
		if !parseTime(in).IsZero() {
			t.Errorf("parseTime(%q) accepted", in)
		}
	}
	if ns := parseTime("123519.123456789123").Nanosecond(); ns != 123456789 {
		t.Errorf("nanoseconds = %d", ns)
	}
}

func TestReader(t *testing.T) {
	in := strings.Join([]string{
		"garbage line",
		"\x00\x00" + sum("GPGGA,235958,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,"), // no date yet
		sum("GPRMC,235959,A,4807.038,N,01131.000,E,0,0,311226,,,A"),
		"$GPGGA,bad*00",
		sum("GPGGA,000001.5,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,"), // past midnight
	}, "\r\n")
	r := NewReader(strings.NewReader(in))

	if _, err := r.Read(); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Time(); ok {
		t.Error("time known before any date")
	}
	if _, err := r.Read(); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Time(); !got.Equal(time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)) {
		t.Errorf("after RMC: %v", got)
	}
	if _, err := r.Read(); !errors.Is(err, ErrChecksum) {
		t.Errorf("bad line: %v", err)
	}
	if _, err := r.Read(); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Time(); !got.Equal(time.Date(2027, 1, 1, 0, 0, 1, 500e6, time.UTC)) {
		t.Errorf("after GGA: %v", got)
	}
	if _, err := r.Read(); err != io.EOF {
		t.Errorf("end: %v", err)
	}
}

func TestMergeTime(t *testing.T) {
	last := time.Date(2026, 1, 1, 0, 0, 2, 0, time.UTC)
	late := time.Date(0, 1, 1, 23, 59, 59, 0, time.UTC) // arrives just after midnight
	if got := MergeTime(last, late); !got.Equal(time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)) {
		t.Errorf("got %v", got)
	}
	if !MergeTime(time.Time{}, late).IsZero() {
		t.Error("no date should give zero")
	}
}

var benchSentences = []string{
	"$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A",
	"$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47",
	"$GPGSV,2,1,08,01,40,083,46,02,17,308,41,12,07,344,39,14,22,228,45*75",
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := Parse(benchSentences[i%len(benchSentences)]); err != nil {
			b.Fatal(err)
		}
	}
}
