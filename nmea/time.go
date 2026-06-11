package nmea

import "time"

// TimeFromSentence extracts UTC time from a parsed NMEA sentence.
// Returns the time and true if the sentence carries a complete date+time.
// Supports: RMC, ZDA (full date+time), GGA, GLL (time only, date is zero).
func TimeFromSentence(s Sentence) (time.Time, bool) {
	switch v := s.(type) {
	case RMC:
		if v.Valid() && !v.Time.IsZero() {
			return v.Time, true
		}
	case ZDA:
		if !v.Time.IsZero() {
			return v.Time, true
		}
	case GNS:
		return v.Time, !v.Time.IsZero()
	case GGA:
		return v.Time, !v.Time.IsZero()
	case GLL:
		return v.Time, !v.Time.IsZero()
	}
	return time.Time{}, false
}
