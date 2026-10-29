package nmea

import (
	"fmt"
	"strings"
	"time"
)

type Sentence interface {
	Raw() string
	Type() string
	Talker() string
}

type BaseSentence struct {
	raw    string
	talker string
	stype  string
}

func (s BaseSentence) Raw() string    { return s.raw }
func (s BaseSentence) Talker() string { return s.talker }
func (s BaseSentence) Type() string   { return s.stype }

// Times of day without a date (GGA, GLL, GNS, GST) are on January 1 of
// year 0; MergeTime puts them on a known date.

// RMC — Recommended Minimum Navigation Information
type RMC struct {
	BaseSentence
	Time      time.Time `json:"time"`   // UTC date and time; year 0 when no date was sent
	Status    string    `json:"status"` // A=active/valid, V=void/invalid
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Speed     float64   `json:"speed"`                // knots
	Course    float64   `json:"course"`               // degrees true
	MagVar    float64   `json:"mag_var"`              // magnetic variation in degrees, negative west
	Mode      string    `json:"mode"`                 // A=autonomous, D=differential, E=estimated, N=not valid, ...
	NavStatus string    `json:"nav_status,omitempty"` // NMEA 4.10: S=safe, C=caution, U=unsafe, V=not valid
}

func (s RMC) Valid() bool { return s.Status == "A" }

func (s RMC) String() string {
	if !s.Valid() {
		return fmt.Sprintf("RMC: INVALID (%s)", s.Status)
	}
	return fmt.Sprintf("RMC: %s | %.6f %.6f | %.1fkn %.1f°",
		s.Time.Format("2006-01-02 15:04:05"), s.Lat, s.Lon, s.Speed, s.Course)
}

// GGA — Global Positioning System Fix Data
type GGA struct {
	BaseSentence
	Time      time.Time `json:"time"` // time of day
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Quality   int       `json:"quality"` // 0=invalid, 1=GPS, 2=DGPS, 4=RTK fixed, 5=RTK float, 6=estimated
	NumSats   int       `json:"num_sats"`
	HDOP      float64   `json:"hdop"`
	Altitude  float64   `json:"altitude"`  // metres above mean sea level
	GeoidSep  float64   `json:"geoid_sep"` // metres
	Age       float64   `json:"age"`       // seconds since the last differential update
	StationID string    `json:"station_id"`
}

func (s GGA) Valid() bool { return s.Quality > 0 }

func (s GGA) String() string {
	if !s.Valid() {
		return fmt.Sprintf("GGA: INVALID (quality=%d)", s.Quality)
	}
	return fmt.Sprintf("GGA: %s | %.6f %.6f | sats=%d hdop=%.1f alt=%.1fm",
		s.Time.Format("15:04:05"), s.Lat, s.Lon, s.NumSats, s.HDOP, s.Altitude)
}

// GSA — GNSS DOP and Active Satellites
type GSA struct {
	BaseSentence
	Mode       string  `json:"mode"`     // M=manual, A=automatic
	FixMode    int     `json:"fix_mode"` // 1=no fix, 2=2D, 3=3D
	Satellites []int   `json:"satellites"`
	PDOP       float64 `json:"pdop"`
	HDOP       float64 `json:"hdop"`
	VDOP       float64 `json:"vdop"`
	SystemID   int     `json:"system_id,omitempty"` // NMEA 4.10: 1=GPS, 2=GLONASS, 3=Galileo, 4=BeiDou, 5=QZSS, 6=NavIC
}

func (s GSA) Valid() bool { return s.FixMode >= 2 }

func (s GSA) String() string {
	return fmt.Sprintf("GSA: fix=%d pdop=%.1f hdop=%.1f vdop=%.1f",
		s.FixMode, s.PDOP, s.HDOP, s.VDOP)
}

// GSV — GNSS Satellites in View
type GSV struct {
	BaseSentence
	NumMsg     int         `json:"num_msg"`
	MsgNum     int         `json:"msg_num"`
	NumSats    int         `json:"num_sats"`
	Satellites []Satellite `json:"satellites"`
	SignalID   int         `json:"signal_id,omitempty"` // NMEA 4.10; 0 when not sent
}

type Satellite struct {
	PRN       int `json:"prn"`
	Elevation int `json:"elevation"` // degrees
	Azimuth   int `json:"azimuth"`   // degrees true
	SNR       int `json:"snr"`       // dB-Hz; 0 when not tracked
}

func (s GSV) String() string {
	return fmt.Sprintf("GSV: (%d/%d) %d sats in view", s.MsgNum, s.NumMsg, s.NumSats)
}

func (s GSV) Details() string {
	var b strings.Builder
	b.WriteString(s.String())
	b.WriteByte('\n')
	for _, sv := range s.Satellites {
		snr := "--"
		if sv.SNR != 0 {
			snr = fmt.Sprint(sv.SNR)
		}
		fmt.Fprintf(&b, "  PRN %3d  el=%3d°  az=%3d°  snr=%s\n", sv.PRN, sv.Elevation, sv.Azimuth, snr)
	}
	return b.String()
}

// VTG — Course Over Ground and Ground Speed
type VTG struct {
	BaseSentence
	CourseTrue     float64 `json:"course_true"`
	CourseMagnetic float64 `json:"course_magnetic"`
	SpeedKnots     float64 `json:"speed_knots"`
	SpeedKMH       float64 `json:"speed_kmh"`
	Mode           string  `json:"mode"`
}

func (s VTG) String() string {
	return fmt.Sprintf("VTG: course=%.1f° speed=%.1fkn/%.1fkmh",
		s.CourseTrue, s.SpeedKnots, s.SpeedKMH)
}

// GLL — Geographic Position - Latitude/Longitude
type GLL struct {
	BaseSentence
	Lat    float64   `json:"lat"`
	Lon    float64   `json:"lon"`
	Time   time.Time `json:"time"` // time of day
	Status string    `json:"status"`
	Mode   string    `json:"mode"`
}

func (s GLL) Valid() bool { return s.Status == "A" }

func (s GLL) String() string {
	if !s.Valid() {
		return "GLL: INVALID"
	}
	return fmt.Sprintf("GLL: %.6f %.6f %s", s.Lat, s.Lon, s.Time.Format("15:04:05"))
}

// ZDA — Time and Date
type ZDA struct {
	BaseSentence
	Time             time.Time `json:"time"`
	LocalZoneHours   int       `json:"local_zone_hours"`
	LocalZoneMinutes int       `json:"local_zone_minutes"`
}

func (s ZDA) String() string {
	return fmt.Sprintf("ZDA: %s", s.Time.Format("2006-01-02 15:04:05"))
}

// TXT — Text Transmission
type TXT struct {
	BaseSentence
	Total   int    `json:"total"`
	Num     int    `json:"num"`
	MsgType int    `json:"msg_type"` // 0=error, 1=warning, 2=notice, 7=user
	Text    string `json:"text"`
}

func (s TXT) String() string {
	return fmt.Sprintf("TXT: %s", s.Text)
}

// GNS — GNSS Fix Data (multi-constellation version of GGA)
type GNS struct {
	BaseSentence
	Time      time.Time `json:"time"` // time of day
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Mode      string    `json:"mode"` // one character per system (GPS, GLONASS, Galileo, BeiDou, ...): N=no fix, A=autonomous, D=differential, R=RTK, ...
	NumSats   int       `json:"num_sats"`
	HDOP      float64   `json:"hdop"`
	Altitude  float64   `json:"altitude"`
	GeoidSep  float64   `json:"geoid_sep"`
	Age       float64   `json:"age"`
	StationID string    `json:"station_id"`
	NavStatus string    `json:"nav_status,omitempty"` // NMEA 4.10
}

// Valid reports whether any system has a fix.
func (s GNS) Valid() bool { return strings.Trim(s.Mode, "N") != "" }

func (s GNS) String() string {
	if !s.Valid() {
		return "GNS: INVALID"
	}
	return fmt.Sprintf("GNS: %s | %.6f %.6f | mode=%s sats=%d hdop=%.1f alt=%.1fm",
		s.Time.Format("15:04:05"), s.Lat, s.Lon, s.Mode, s.NumSats, s.HDOP, s.Altitude)
}

// GST — GNSS Pseudorange Error Statistics
type GST struct {
	BaseSentence
	Time     time.Time `json:"time"`      // time of day
	RMS      float64   `json:"rms"`       // RMS pseudorange residuals
	MajorStd float64   `json:"major_std"` // semi-major axis std dev, metres
	MinorStd float64   `json:"minor_std"` // semi-minor axis std dev, metres
	Orient   float64   `json:"orient"`    // orientation of semi-major axis (degrees)
	LatStd   float64   `json:"lat_std"`   // latitude error std dev, metres
	LonStd   float64   `json:"lon_std"`   // longitude error std dev, metres
	AltStd   float64   `json:"alt_std"`   // altitude error std dev, metres
}

func (s GST) String() string {
	return fmt.Sprintf("GST: rms=%.2f lat=%.2fm lon=%.2fm alt=%.2fm",
		s.RMS, s.LatStd, s.LonStd, s.AltStd)
}

// UnknownSentence — fallback for unrecognized and proprietary ($P...)
// sentences. Proprietary sentences have talker "P".
type UnknownSentence struct {
	BaseSentence
	Fields []string `json:"fields"`
}

func (s UnknownSentence) String() string {
	return s.raw
}
