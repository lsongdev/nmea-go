package nmea

import (
	"fmt"
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

// RMC — Recommended Minimum Navigation Information
type RMC struct {
	BaseSentence
	Time   time.Time
	Status string // A=active/valid, V=void/invalid
	Lat    float64
	Lon    float64
	Speed  float64 // knots
	Course float64 // degrees true
	MagVar float64
	Mode   string
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
	Time      time.Time
	Lat       float64
	Lon       float64
	Quality   int // 0=invalid, 1=GPS, 2=DGPS
	NumSats   int
	HDOP      float64
	Altitude  float64
	GeoidSep  float64
	Age       float64
	StationID string
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
	Mode       string // M=manual, A=automatic
	FixMode    int    // 1=no fix, 2=2D, 3=3D
	Satellites []int
	PDOP       float64
	HDOP       float64
	VDOP       float64
}

func (s GSA) Valid() bool { return s.FixMode >= 2 }

func (s GSA) String() string {
	return fmt.Sprintf("GSA: fix=%d pdop=%.1f hdop=%.1f vdop=%.1f",
		s.FixMode, s.PDOP, s.HDOP, s.VDOP)
}

// GSV — GNSS Satellites in View
type GSV struct {
	BaseSentence
	NumMsg    int
	MsgNum    int
	NumSats   int
	Satellites []Satellite
}

type Satellite struct {
	PRN       int
	Elevation int
	Azimuth   int
	SNR       int
}

func (s GSV) String() string {
	return fmt.Sprintf("GSV: (%d/%d) %d sats in view", s.MsgNum, s.NumMsg, s.NumSats)
}

func (s GSV) Details() string {
	out := fmt.Sprintf("%s\n", s)
	for _, sv := range s.Satellites {
		snr := fmt.Sprintf("%d", sv.SNR)
		if sv.SNR == 0 {
			snr = "--"
		}
		out += fmt.Sprintf("  PRN %3d  el=%3d°  az=%3d°  snr=%s\n", sv.PRN, sv.Elevation, sv.Azimuth, snr)
	}
	return out
}

// VTG — Course Over Ground and Ground Speed
type VTG struct {
	BaseSentence
	CourseTrue     float64
	CourseMagnetic float64
	SpeedKnots     float64
	SpeedKMH       float64
	Mode           string
}

func (s VTG) String() string {
	return fmt.Sprintf("VTG: course=%.1f° speed=%.1fkn/%.1fkmh",
		s.CourseTrue, s.SpeedKnots, s.SpeedKMH)
}

// GLL — Geographic Position - Latitude/Longitude
type GLL struct {
	BaseSentence
	Lat    float64
	Lon    float64
	Time   time.Time
	Status string
	Mode   string
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
	Time             time.Time
	LocalZoneHours   int
	LocalZoneMinutes int
}

func (s ZDA) String() string {
	return fmt.Sprintf("ZDA: %s", s.Time.Format("2006-01-02 15:04:05"))
}

// TXT — Text Transmission
type TXT struct {
	BaseSentence
	Total   int
	Num     int
	MsgType int
	Text    string
}

func (s TXT) String() string {
	return fmt.Sprintf("TXT: %s", s.Text)
}

// GNS — GNSS Fix Data (multi-constellation version of GGA)
type GNS struct {
	BaseSentence
	Time     time.Time
	Lat      float64
	Lon      float64
	Mode     string // characters: G=GPS, R=GLONASS, E=Galileo, C=BeiDou, Q=QZSS, I=IRNSS
	NumSats  int
	HDOP     float64
	Altitude float64
	GeoidSep float64
	Age      float64
	StationID string
}

func (s GNS) Valid() bool { return s.Mode != "" && s.Mode != "N" }

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
	Time        time.Time
	RMS         float64 // RMS pseudorange residuals
	MajorStd    float64 // semi-major axis std dev
	MinorStd    float64 // semi-minor axis std dev
	Orient      float64 // orientation of semi-major axis (degrees)
	LatStd      float64 // latitude error std dev
	LonStd      float64 // longitude error std dev
	AltStd      float64 // altitude error std dev
}

func (s GST) String() string {
	return fmt.Sprintf("GST: rms=%.2f lat=%.2fm lon=%.2fm alt=%.2fm",
		s.RMS, s.LatStd, s.LonStd, s.AltStd)
}

// UnknownSentence — fallback for unrecognized types
type UnknownSentence struct {
	BaseSentence
	Fields []string
}

func (s UnknownSentence) String() string {
	return s.raw
}
