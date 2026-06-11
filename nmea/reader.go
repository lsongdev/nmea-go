package nmea

import (
	"bufio"
	"io"
	"time"
)

type Reader struct {
	scanner  *bufio.Scanner
	lastTime time.Time
}

func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 4096)
	return &Reader{scanner: scanner}
}

func (r *Reader) Read() (Sentence, error) {
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if len(line) == 0 || line[0] != '$' {
			continue
		}
		s, err := Parse(line)
		if err == nil {
			if t, ok := TimeFromSentence(s); ok {
				r.lastTime = t
			}
		}
		return s, err
	}
	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Time returns the latest complete UTC time received from parsed sentences.
// Returns false if no time information has been received yet.
func (r *Reader) Time() (time.Time, bool) {
	return r.lastTime, !r.lastTime.IsZero()
}
