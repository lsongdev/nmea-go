package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"go.bug.st/serial"

	"github.com/lsongdev/nmea-go/nmea"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <port> [baud]", os.Args[0])
	}
	portName := os.Args[1]
	baud := 9600
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &baud)
	}

	port, err := serial.Open(portName, &serial.Mode{BaudRate: baud})
	if err != nil {
		log.Fatalf("serial open: %v", err)
	}
	defer port.Close()

	r := nmea.NewReader(port)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func() {
		<-sig
		os.Exit(0)
	}()

	log.Printf("listening on %s @ %d baud", portName, baud)
	for {
		s, err := r.Read()
		if err != nil {
			log.Fatalf("read: %v", err)
		}

		if t, ok := r.Time(); ok {
			fmt.Printf("⏱ %s  ", t.Format("15:04:05.000"))
		}

		switch v := s.(type) {
		case nmea.RMC:
			if v.Valid() {
				fmt.Printf("📍 RMC: %.6f %.6f  s=%.1fkn  c=%.1f°\n",
					v.Lat, v.Lon, v.Speed, v.Course)
			}
		case nmea.GGA:
			if v.Valid() {
				fmt.Printf("🛰 GGA: sats=%d  hdop=%.1f  alt=%.1fm\n",
					v.NumSats, v.HDOP, v.Altitude)
			}
		case nmea.GSA:
			fix := map[int]string{1: "no fix", 2: "2D", 3: "3D"}[v.FixMode]
			fmt.Printf("📡 GSA: fix=%s  pdop=%.1f  hdop=%.1f  vdop=%.1f\n",
				fix, v.PDOP, v.HDOP, v.VDOP)
		case nmea.GSV:
			fmt.Print(v.Details())
		case nmea.VTG:
			fmt.Printf("🚗 VTG: course=%.1f°  speed=%.1fkn/%.1fkmh\n",
				v.CourseTrue, v.SpeedKnots, v.SpeedKMH)
		case nmea.GNS:
			if v.Valid() {
				fmt.Printf("🛰 GNS: mode=%s sats=%d hdop=%.1f alt=%.1fm\n",
					v.Mode, v.NumSats, v.HDOP, v.Altitude)
			}
		case nmea.GST:
			fmt.Printf("📏 GST: rms=%.2f lat=%.2fm lon=%.2fm alt=%.2fm\n",
				v.RMS, v.LatStd, v.LonStd, v.AltStd)
		case nmea.TXT:
			fmt.Printf("ℹ️ %s\n", v.Text)
		case nmea.ZDA:
			fmt.Printf("📅 ZDA: %s  offset=%+dh%dm\n",
				v.Time.Format("2006-01-02 15:04:05"), v.LocalZoneHours, v.LocalZoneMinutes)
		case nmea.GLL:
			if v.Valid() {
				fmt.Printf("📍 GLL: %.6f %.6f\n", v.Lat, v.Lon)
			}
		default:
			fmt.Printf("💬 %s\n", s)
		}
	}
}
