package nmea

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func Parse(raw string) (Sentence, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 4 || raw[0] != '$' {
		return nil, fmt.Errorf("nmea: invalid sentence: %s", raw)
	}
	if err := checksum(raw); err != nil {
		return nil, err
	}

	star := strings.LastIndexByte(raw, '*')
	body := raw[1:star]
	chk := raw[star+1:]

	fields := strings.Split(body, ",")
	if len(fields) < 1 {
		return nil, fmt.Errorf("nmea: empty body")
	}

	typeField := fields[0]
	if len(typeField) < 5 {
		return nil, fmt.Errorf("nmea: invalid type field: %s", typeField)
	}

	base := BaseSentence{raw: raw, talker: typeField[:2], stype: typeField[2:]}
	data := fields[1:]

	switch base.stype {
	case "RMC":
		return parseRMC(base, data, chk)
	case "GGA":
		return parseGGA(base, data)
	case "GSA":
		return parseGSA(base, data)
	case "GSV":
		return parseGSV(base, data)
	case "VTG":
		return parseVTG(base, data)
	case "GLL":
		return parseGLL(base, data)
	case "ZDA":
		return parseZDA(base, data)
	case "GNS":
		return parseGNS(base, data)
	case "GST":
		return parseGST(base, data)
	case "TXT":
		return parseTXT(base, data)
	default:
		return UnknownSentence{BaseSentence: base, Fields: data}, nil
	}
}

func checksum(s string) error {
	star := strings.LastIndexByte(s, '*')
	if star < 0 {
		return fmt.Errorf("nmea: missing checksum")
	}
	chk := s[star+1:]
	if len(chk) != 2 {
		return fmt.Errorf("nmea: invalid checksum: %s", chk)
	}
	var cs byte
	for i := 1; i < star; i++ {
		cs ^= s[i]
	}
	expect := fmt.Sprintf("%02X", cs)
	if !strings.EqualFold(chk, expect) {
		return fmt.Errorf("nmea: checksum mismatch: %s != %s", chk, expect)
	}
	return nil
}

func parseRMC(base BaseSentence, data []string, _ string) (RMC, error) {
	s := RMC{BaseSentence: base}
	if len(data) < 11 {
		return s, nil
	}
	s.Status = data[1]

	if len(data[0]) >= 6 {
		h, _ := strconv.Atoi(data[0][0:2])
		m, _ := strconv.Atoi(data[0][2:4])
		sec, _ := strconv.Atoi(data[0][4:6])
		s.Time = time.Date(0, 1, 1, h, m, sec, 0, time.UTC)
	}

	if s.Status == "A" {
		s.Lat = nmeaLatLon(data[2], data[3])
		s.Lon = nmeaLatLon(data[4], data[5])
		s.Speed = parseFloat(data[6])
		s.Course = parseFloat(data[7])
	}

	if len(data[8]) == 6 {
		d, _ := strconv.Atoi(data[8][0:2])
		m, _ := strconv.Atoi(data[8][2:4])
		y := strToInt(data[8][4:6], 0) + 2000
		s.Time = time.Date(y, time.Month(m), d, s.Time.Hour(), s.Time.Minute(), s.Time.Second(), 0, time.UTC)
	}

	if len(data) >= 11 && data[9] != "" {
		s.MagVar = nmeaLatLon(data[9], data[10])
	}
	if len(data) >= 12 {
		s.Mode = data[11]
	}
	return s, nil
}

func parseGGA(base BaseSentence, data []string) (GGA, error) {
	s := GGA{BaseSentence: base}
	if len(data) < 14 {
		return s, nil
	}
	if len(data[0]) >= 6 {
		h, _ := strconv.Atoi(data[0][0:2])
		m, _ := strconv.Atoi(data[0][2:4])
		sec, _ := strconv.Atoi(data[0][4:6])
		s.Time = time.Date(0, 1, 1, h, m, sec, 0, time.UTC)
	}
	s.Lat = nmeaLatLon(data[1], data[2])
	s.Lon = nmeaLatLon(data[3], data[4])
	s.Quality = parseInt(data[5])
	s.NumSats = parseInt(data[6])
	s.HDOP = parseFloat(data[7])
	s.Altitude = parseFloat(data[8])
	s.GeoidSep = parseFloat(data[10])
	s.Age = parseFloat(data[12])
	s.StationID = data[13]
	return s, nil
}

func parseGSA(base BaseSentence, data []string) (GSA, error) {
	s := GSA{BaseSentence: base}
	if len(data) < 17 {
		return s, nil
	}
	s.Mode = data[0]
	s.FixMode = parseInt(data[1])
	for i := 2; i < 14; i++ {
		if data[i] != "" {
			s.Satellites = append(s.Satellites, parseInt(data[i]))
		}
	}
	s.PDOP = parseFloat(data[14])
	s.HDOP = parseFloat(data[15])
	s.VDOP = parseFloat(data[16])
	return s, nil
}

func parseGSV(base BaseSentence, data []string) (GSV, error) {
	s := GSV{BaseSentence: base}
	if len(data) < 4 {
		return s, nil
	}
	s.NumMsg = parseInt(data[0])
	s.MsgNum = parseInt(data[1])
	s.NumSats = parseInt(data[2])
	for i := 3; i+3 < len(data); i += 4 {
		s.Satellites = append(s.Satellites, Satellite{
			PRN:       parseInt(data[i]),
			Elevation: parseInt(data[i+1]),
			Azimuth:   parseInt(data[i+2]),
			SNR:       parseInt(data[i+3]),
		})
	}
	return s, nil
}

func parseVTG(base BaseSentence, data []string) (VTG, error) {
	s := VTG{BaseSentence: base}
	if len(data) < 8 {
		return s, nil
	}
	s.CourseTrue = parseFloat(data[0])
	s.CourseMagnetic = parseFloat(data[2])
	s.SpeedKnots = parseFloat(data[4])
	s.SpeedKMH = parseFloat(data[6])
	if len(data) > 8 {
		s.Mode = data[8]
	}
	return s, nil
}

func parseGLL(base BaseSentence, data []string) (GLL, error) {
	s := GLL{BaseSentence: base}
	if len(data) < 6 {
		return s, nil
	}
	s.Lat = nmeaLatLon(data[0], data[1])
	s.Lon = nmeaLatLon(data[2], data[3])
	if len(data[4]) >= 6 {
		h, _ := strconv.Atoi(data[4][0:2])
		m, _ := strconv.Atoi(data[4][2:4])
		sec, _ := strconv.Atoi(data[4][4:6])
		s.Time = time.Date(0, 1, 1, h, m, sec, 0, time.UTC)
	}
	s.Status = data[5]
	if len(data) > 6 {
		s.Mode = data[6]
	}
	return s, nil
}

func parseZDA(base BaseSentence, data []string) (ZDA, error) {
	s := ZDA{BaseSentence: base}
	if len(data) < 6 {
		return s, nil
	}
	if len(data[0]) >= 6 {
		h, _ := strconv.Atoi(data[0][0:2])
		m, _ := strconv.Atoi(data[0][2:4])
		sec, _ := strconv.Atoi(data[0][4:6])
		d := parseInt(data[1])
		mo := parseInt(data[2])
		y := parseInt(data[3])
		s.Time = time.Date(y, time.Month(mo), d, h, m, sec, 0, time.UTC)
	}
	s.LocalZoneHours = parseInt(data[4])
	s.LocalZoneMinutes = parseInt(data[5])
	return s, nil
}

func parseTXT(base BaseSentence, data []string) (TXT, error) {
	s := TXT{BaseSentence: base}
	if len(data) < 4 {
		return s, nil
	}
	s.Total = parseInt(data[0])
	s.Num = parseInt(data[1])
	s.MsgType = parseInt(data[2])
	s.Text = data[3]
	return s, nil
}

func parseGNS(base BaseSentence, data []string) (GNS, error) {
	s := GNS{BaseSentence: base}
	if len(data) < 11 {
		return s, nil
	}
	if len(data[0]) >= 6 {
		h, _ := strconv.Atoi(data[0][0:2])
		m, _ := strconv.Atoi(data[0][2:4])
		sec, _ := strconv.Atoi(data[0][4:6])
		s.Time = time.Date(0, 1, 1, h, m, sec, 0, time.UTC)
	}
	s.Lat = nmeaLatLon(data[1], data[2])
	s.Lon = nmeaLatLon(data[3], data[4])
	s.Mode = data[5]
	s.NumSats = parseInt(data[6])
	s.HDOP = parseFloat(data[7])
	s.Altitude = parseFloat(data[8])
	s.GeoidSep = parseFloat(data[9])
	s.Age = parseFloat(data[10])
	if len(data) > 11 {
		s.StationID = data[11]
	}
	return s, nil
}

func parseGST(base BaseSentence, data []string) (GST, error) {
	s := GST{BaseSentence: base}
	if len(data) < 8 {
		return s, nil
	}
	if len(data[0]) >= 6 {
		h, _ := strconv.Atoi(data[0][0:2])
		m, _ := strconv.Atoi(data[0][2:4])
		sec, _ := strconv.Atoi(data[0][4:6])
		s.Time = time.Date(0, 1, 1, h, m, sec, 0, time.UTC)
	}
	s.RMS = parseFloat(data[1])
	s.MajorStd = parseFloat(data[2])
	s.MinorStd = parseFloat(data[3])
	s.Orient = parseFloat(data[4])
	s.LatStd = parseFloat(data[5])
	s.LonStd = parseFloat(data[6])
	s.AltStd = parseFloat(data[7])
	return s, nil
}

func nmeaLatLon(val, dir string) float64 {
	if val == "" || dir == "" {
		return 0
	}
	dot := strings.IndexByte(val, '.')
	if dot < 0 {
		return 0
	}
	degLen := 2
	if dir == "E" || dir == "W" {
		degLen = 3
	}
	deg, _ := strconv.ParseFloat(val[:degLen], 64)
	min, _ := strconv.ParseFloat(val[degLen:], 64)
	r := deg + min/60.0
	if dir == "S" || dir == "W" {
		r = -r
	}
	return r
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

func strToInt(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
