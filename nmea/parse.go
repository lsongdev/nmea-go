package nmea

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrFormat is returned (wrapped) for a malformed sentence.
	ErrFormat = errors.New("nmea: invalid sentence")
	// ErrChecksum is returned (wrapped) when the checksum is missing or wrong.
	ErrChecksum = errors.New("nmea: bad checksum")
)

// minFields is the number of data fields (after the address) each decoded
// type needs. Later, optional fields (NMEA 4.10 additions) are read when
// present.
var minFields = map[string]int{
	"RMC": 11, "GGA": 14, "GSA": 17, "GSV": 3, "VTG": 8,
	"GLL": 6, "ZDA": 6, "GNS": 11, "GST": 8, "TXT": 4,
}

// Parse decodes one sentence, "$<address>,<fields>*<checksum>". Errors
// wrap ErrFormat or ErrChecksum.
func Parse(raw string) (Sentence, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 4 || raw[0] != '$' {
		return nil, fmt.Errorf("%w: %q", ErrFormat, raw)
	}
	star := strings.LastIndexByte(raw, '*')
	if star < 0 {
		return nil, fmt.Errorf("%w: missing in %q", ErrChecksum, raw)
	}
	if err := verify(raw[1:star], raw[star+1:]); err != nil {
		return nil, err
	}

	fields := strings.Split(raw[1:star], ",")
	addr, data := fields[0], fields[1:]
	var base BaseSentence
	switch {
	case len(addr) >= 2 && addr[0] == 'P': // proprietary: $PUBX, $PMTK001, $PGRME, ...
		base = BaseSentence{raw: raw, talker: "P", stype: addr[1:]}
		return UnknownSentence{BaseSentence: base, Fields: data}, nil
	case len(addr) >= 5:
		base = BaseSentence{raw: raw, talker: addr[:2], stype: addr[2:]}
	default:
		return nil, fmt.Errorf("%w: address %q", ErrFormat, addr)
	}
	if n, ok := minFields[base.stype]; ok && len(data) < n {
		return nil, fmt.Errorf("%w: %s has %d fields, want %d", ErrFormat, base.stype, len(data), n)
	}

	switch base.stype {
	case "RMC":
		return parseRMC(base, data), nil
	case "GGA":
		return parseGGA(base, data), nil
	case "GSA":
		return parseGSA(base, data), nil
	case "GSV":
		return parseGSV(base, data), nil
	case "VTG":
		return parseVTG(base, data), nil
	case "GLL":
		return parseGLL(base, data), nil
	case "ZDA":
		return parseZDA(base, data), nil
	case "GNS":
		return parseGNS(base, data), nil
	case "GST":
		return parseGST(base, data), nil
	case "TXT":
		return parseTXT(base, data), nil
	default:
		return UnknownSentence{BaseSentence: base, Fields: data}, nil
	}
}

// verify checks that hex is the XOR of every byte of body.
func verify(body, hex string) error {
	want, ok := parseHexByte(hex)
	if !ok {
		return fmt.Errorf("%w: invalid %q", ErrChecksum, hex)
	}
	var cs byte
	for i := 0; i < len(body); i++ {
		cs ^= body[i]
	}
	if cs != want {
		return fmt.Errorf("%w: %s != %02X", ErrChecksum, hex, cs)
	}
	return nil
}

func parseHexByte(s string) (byte, bool) {
	if len(s) != 2 {
		return 0, false
	}
	hi, ok1 := hexDigit(s[0])
	lo, ok2 := hexDigit(s[1])
	return hi<<4 | lo, ok1 && ok2
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func parseRMC(base BaseSentence, data []string) RMC {
	s := RMC{BaseSentence: base, Status: data[1]}
	s.Time = parseTime(data[0])
	if s.Status == "A" {
		s.Lat = parseLatLon(data[2], data[3])
		s.Lon = parseLatLon(data[4], data[5])
		s.Speed = parseFloat(data[6])
		s.Course = parseFloat(data[7])
	}
	if !s.Time.IsZero() {
		if y, m, d, ok := parseDate(data[8]); ok {
			s.Time = time.Date(y, m, d, s.Time.Hour(), s.Time.Minute(), s.Time.Second(), s.Time.Nanosecond(), time.UTC)
		}
	}
	s.MagVar = parseFloat(data[9])
	if data[10] == "W" {
		s.MagVar = -s.MagVar
	}
	s.Mode = field(data, 11)
	s.NavStatus = field(data, 12)
	return s
}

func parseGGA(base BaseSentence, data []string) GGA {
	return GGA{
		BaseSentence: base,
		Time:         parseTime(data[0]),
		Lat:          parseLatLon(data[1], data[2]),
		Lon:          parseLatLon(data[3], data[4]),
		Quality:      parseInt(data[5]),
		NumSats:      parseInt(data[6]),
		HDOP:         parseFloat(data[7]),
		Altitude:     parseFloat(data[8]),
		GeoidSep:     parseFloat(data[10]),
		Age:          parseFloat(data[12]),
		StationID:    data[13],
	}
}

func parseGSA(base BaseSentence, data []string) GSA {
	s := GSA{BaseSentence: base, Mode: data[0], FixMode: parseInt(data[1])}
	for _, f := range data[2:14] {
		if f != "" {
			s.Satellites = append(s.Satellites, parseInt(f))
		}
	}
	s.PDOP = parseFloat(data[14])
	s.HDOP = parseFloat(data[15])
	s.VDOP = parseFloat(data[16])
	s.SystemID = parseHex(field(data, 17))
	return s
}

func parseGSV(base BaseSentence, data []string) GSV {
	s := GSV{BaseSentence: base}
	s.NumMsg = parseInt(data[0])
	s.MsgNum = parseInt(data[1])
	s.NumSats = parseInt(data[2])
	sats := data[3:]
	if len(sats)%4 == 1 { // NMEA 4.10 signal ID after the last satellite
		s.SignalID = parseHex(sats[len(sats)-1])
		sats = sats[:len(sats)-1]
	}
	for i := 0; i+3 < len(sats); i += 4 {
		if sats[i] == "" {
			continue // padding in the last message
		}
		s.Satellites = append(s.Satellites, Satellite{
			PRN:       parseInt(sats[i]),
			Elevation: parseInt(sats[i+1]),
			Azimuth:   parseInt(sats[i+2]),
			SNR:       parseInt(sats[i+3]),
		})
	}
	return s
}

func parseVTG(base BaseSentence, data []string) VTG {
	return VTG{
		BaseSentence:   base,
		CourseTrue:     parseFloat(data[0]),
		CourseMagnetic: parseFloat(data[2]),
		SpeedKnots:     parseFloat(data[4]),
		SpeedKMH:       parseFloat(data[6]),
		Mode:           field(data, 8),
	}
}

func parseGLL(base BaseSentence, data []string) GLL {
	return GLL{
		BaseSentence: base,
		Lat:          parseLatLon(data[0], data[1]),
		Lon:          parseLatLon(data[2], data[3]),
		Time:         parseTime(data[4]),
		Status:       data[5],
		Mode:         field(data, 6),
	}
}

func parseZDA(base BaseSentence, data []string) ZDA {
	s := ZDA{BaseSentence: base}
	tod := parseTime(data[0])
	d, m, y := parseInt(data[1]), parseInt(data[2]), parseInt(data[3])
	if !tod.IsZero() && y > 0 && m >= 1 && m <= 12 && d >= 1 && d <= 31 {
		s.Time = time.Date(y, time.Month(m), d, tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), time.UTC)
	}
	s.LocalZoneHours = parseInt(data[4])
	s.LocalZoneMinutes = parseInt(data[5])
	return s
}

func parseTXT(base BaseSentence, data []string) TXT {
	return TXT{
		BaseSentence: base,
		Total:        parseInt(data[0]),
		Num:          parseInt(data[1]),
		MsgType:      parseInt(data[2]),
		Text:         strings.Join(data[3:], ","),
	}
}

func parseGNS(base BaseSentence, data []string) GNS {
	return GNS{
		BaseSentence: base,
		Time:         parseTime(data[0]),
		Lat:          parseLatLon(data[1], data[2]),
		Lon:          parseLatLon(data[3], data[4]),
		Mode:         data[5],
		NumSats:      parseInt(data[6]),
		HDOP:         parseFloat(data[7]),
		Altitude:     parseFloat(data[8]),
		GeoidSep:     parseFloat(data[9]),
		Age:          parseFloat(data[10]),
		StationID:    field(data, 11),
		NavStatus:    field(data, 12),
	}
}

func parseGST(base BaseSentence, data []string) GST {
	return GST{
		BaseSentence: base,
		Time:         parseTime(data[0]),
		RMS:          parseFloat(data[1]),
		MajorStd:     parseFloat(data[2]),
		MinorStd:     parseFloat(data[3]),
		Orient:       parseFloat(data[4]),
		LatStd:       parseFloat(data[5]),
		LonStd:       parseFloat(data[6]),
		AltStd:       parseFloat(data[7]),
	}
}

// parseTime decodes "hhmmss" or "hhmmss.sss" as a time of day on January 1
// of year 0, or returns the zero time.
func parseTime(s string) time.Time {
	if len(s) < 6 {
		return time.Time{}
	}
	h, ok1 := atoi2(s[0:2])
	m, ok2 := atoi2(s[2:4])
	sec, ok3 := atoi2(s[4:6])
	if !ok1 || !ok2 || !ok3 || h > 23 || m > 59 || sec > 60 {
		return time.Time{}
	}
	ns := 0
	if len(s) > 6 {
		if s[6] != '.' {
			return time.Time{}
		}
		scale := int(time.Second)
		for i := 7; i < len(s) && scale > 1; i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return time.Time{}
			}
			scale /= 10
			ns += int(c-'0') * scale
		}
	}
	return time.Date(0, 1, 1, h, m, sec, ns, time.UTC)
}

// parseDate decodes "ddmmyy". Two-digit years 80-99 are 1980-1999 (the
// first GPS epoch), others 2000-2079.
func parseDate(s string) (int, time.Month, int, bool) {
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	d, ok1 := atoi2(s[0:2])
	m, ok2 := atoi2(s[2:4])
	y, ok3 := atoi2(s[4:6])
	if !ok1 || !ok2 || !ok3 || d < 1 || d > 31 || m < 1 || m > 12 {
		return 0, 0, 0, false
	}
	if y >= 80 {
		y += 1900
	} else {
		y += 2000
	}
	return y, time.Month(m), d, true
}

func atoi2(s string) (int, bool) {
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}

// parseLatLon decodes "ddmm.mmmm" or "dddmm.mmmm" with a hemisphere into
// signed decimal degrees.
func parseLatLon(val, dir string) float64 {
	if val == "" || dir == "" {
		return 0
	}
	// Minutes are the two digits before the point; split the text rather
	// than the float so 4807.038 gives exactly 48 + 7.038/60.
	split := strings.IndexByte(val, '.')
	if split < 0 {
		split = len(val)
	}
	split -= 2
	if split < 0 {
		return 0
	}
	deg, err1 := strconv.ParseFloat(val[:split], 64)
	min, err2 := strconv.ParseFloat(val[split:], 64)
	if split == 0 {
		deg, err1 = 0, nil
	}
	if err1 != nil || err2 != nil || deg < 0 || min < 0 || min >= 60 {
		return 0
	}
	r := deg + min/60
	if dir == "S" || dir == "W" {
		r = -r
	}
	return r
}

// field returns data[i], or "" when the sentence is shorter.
func field(data []string, i int) string {
	if i < len(data) {
		return data[i]
	}
	return ""
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func parseHex(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 16, 0)
	return int(v)
}
