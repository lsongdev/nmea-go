package nmea

import (
	"bufio"
	"io"
	"strings"
	"time"
)

// Reader reads sentences from a stream, one per line.
type Reader struct {
	scanner  *bufio.Scanner
	lastTime time.Time
}

func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 4096)
	return &Reader{scanner: scanner}
}

// Read returns the next sentence. Lines without a '$' are skipped, as is
// noise before it. A sentence that fails to parse returns an error wrapping
// ErrFormat or ErrChecksum, and reading can go on; other errors (and
// io.EOF) end the stream.
func (r *Reader) Read() (Sentence, error) {
	for r.scanner.Scan() {
		line := r.scanner.Text()
		i := strings.IndexByte(line, '$')
		if i < 0 {
			continue
		}
		s, err := Parse(line[i:])
		if err != nil {
			return nil, err
		}
		if t, ok := TimeFromSentence(s); ok {
			r.lastTime = t
		} else if tod, ok := TimeOfDay(s); ok && !r.lastTime.IsZero() {
			r.lastTime = MergeTime(r.lastTime, tod)
		}
		return s, nil
	}
	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Time returns the latest complete UTC time received: the date from the
// last RMC or ZDA, advanced by the times of day of later sentences.
// Returns false until an RMC with a fix or a ZDA has been received.
func (r *Reader) Time() (time.Time, bool) {
	return r.lastTime, !r.lastTime.IsZero()
}
