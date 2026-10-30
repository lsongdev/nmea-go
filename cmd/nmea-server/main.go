// Command nmea-server reads NMEA 0183 from a serial port, a TCP stream or a
// recorded file, and serves the live state over HTTP with a web UI.
//
//	nmea-server -source /dev/ttyUSB0 -baud 9600
//	nmea-server -source tcp://192.168.1.10:10110
//	nmea-server -source testdata/sample.nmea -replay 1s
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.bug.st/serial"

	"github.com/lsongdev/nmea-go/api"
	"github.com/lsongdev/nmea-go/web"
)

func main() {
	source := flag.String("source", "", "serial port, tcp://host:port, or a file of recorded sentences")
	baud := flag.Int("baud", 9600, "serial baud rate")
	replay := flag.Duration("replay", time.Second, "delay between fixes (RMC sentences) when replaying a file, which loops at the end")
	listen := flag.String("listen", ":8080", "HTTP listen address")
	token := flag.String("token", os.Getenv("NMEA_TOKEN"), "require this token (Bearer, or basic auth password)")
	flag.Parse()
	if *source == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracker := api.NewTracker(*source)
	go run(ctx, tracker, func() (io.ReadCloser, error) { return open(*source, *baud, *replay) })

	srv := &http.Server{
		Addr:              *listen,
		Handler:           web.Standalone(api.NewHandler(tracker, api.Options{}), *token),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("reading %s; api and web ui on http://%s", *source, *listen)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// run feeds the tracker from the source, reopening it when it fails.
func run(ctx context.Context, t *api.Tracker, open func() (io.ReadCloser, error)) {
	for ctx.Err() == nil {
		rc, err := open()
		if err != nil {
			t.SetError(err)
			log.Printf("source: %v", err)
		} else {
			stop := context.AfterFunc(ctx, func() { rc.Close() })
			err = t.Consume(ctx, rc)
			stop()
			rc.Close()
			if ctx.Err() == nil {
				log.Printf("source closed: %v", err)
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}

func open(source string, baud int, replay time.Duration) (io.ReadCloser, error) {
	if addr, ok := strings.CutPrefix(source, "tcp://"); ok {
		return net.DialTimeout("tcp", addr, 5*time.Second)
	}
	if st, err := os.Stat(source); err == nil && st.Mode().IsRegular() {
		return replayFile(source, replay)
	}
	return serial.Open(source, &serial.Mode{BaudRate: baud})
}

// replayFile streams a recorded file, waiting delay before each RMC (one
// per fix, as a receiver would), and loops back to the start at the end.
func replayFile(name string, delay time.Duration) (io.ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		defer f.Close()
		for {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := sc.Text()
				if len(line) > 6 && line[3:6] == "RMC" {
					time.Sleep(delay)
				}
				if _, err := io.WriteString(pw, line+"\r\n"); err != nil {
					return // the reader closed the pipe
				}
			}
			if err := sc.Err(); err != nil {
				pw.CloseWithError(err)
				return
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()
	return pr, nil
}
