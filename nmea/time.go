package nmea

import "time"

// TimeFromSentence returns the complete UTC date and time a sentence
// carries: RMC with a valid fix and a date, or ZDA.
func TimeFromSentence(s Sentence) (time.Time, bool) {
	switch v := s.(type) {
	case RMC:
		if v.Valid() && v.Time.Year() > 0 {
			return v.Time, true
		}
	case ZDA:
		if !v.Time.IsZero() {
			return v.Time, true
		}
	}
	return time.Time{}, false
}

// TimeOfDay returns the UTC time of day a sentence carries (RMC, ZDA, GGA,
// GLL, GNS, GST), on January 1 of year 0. Use MergeTime to date it.
func TimeOfDay(s Sentence) (time.Time, bool) {
	var t time.Time
	switch v := s.(type) {
	case RMC:
		t = v.Time
	case ZDA:
		t = v.Time
	case GGA:
		t = v.Time
	case GLL:
		t = v.Time
	case GNS:
		t = v.Time
	case GST:
		t = v.Time
	}
	if t.IsZero() {
		return time.Time{}, false
	}
	return time.Date(0, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC), true
}

// MergeTime returns the time of day tod on the date of last, the latest
// known complete time, moving to the next or previous day when tod is more
// than 12 hours away (midnight passed between the two).
func MergeTime(last, tod time.Time) time.Time {
	if last.IsZero() {
		return time.Time{}
	}
	last = last.UTC()
	y, m, d := last.Date()
	t := time.Date(y, m, d, tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), time.UTC)
	switch diff := t.Sub(last); {
	case diff < -12*time.Hour:
		t = t.AddDate(0, 0, 1)
	case diff > 12*time.Hour:
		t = t.AddDate(0, 0, -1)
	}
	return t
}
