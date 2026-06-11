# GPS-Go

Pure Go NMEA 0183 protocol parser library with zero dependencies, supporting GPS/BeiDou/GLONASS standard sentences.

## Quick Start

```go
import "github.com/lsongdev/gps-go/nmea"

s, _ := nmea.Parse("$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A")
rmc := s.(nmea.RMC)
fmt.Println(rmc.Lat, rmc.Lon, rmc.Speed)
```

Stream reading from serial port:

```go
port, _ := serial.Open("/dev/ttyUSB0", &serial.Mode{BaudRate: 9600})
defer port.Close()

r := nmea.NewReader(port)
s, _ := r.Read()
```

## Supported Sentences

| Sentence | Struct | Description |
|----------|--------|-------------|
| RMC | `nmea.RMC` | Recommended minimum navigation info (position, speed, time) |
| GGA | `nmea.GGA` | GPS fix data (altitude, satellite count, fix quality) |
| GSA | `nmea.GSA` | Dilution of precision and active satellites (PDOP/HDOP/VDOP) |
| GSV | `nmea.GSV` | Satellites in view (PRN/elevation/azimuth/SNR) |
| GNS | `nmea.GNS` | Multi-constellation fix data |
| GST | `nmea.GST` | Pseudorange error statistics (position accuracy estimate) |
| VTG | `nmea.VTG` | Course over ground and ground speed |
| GLL | `nmea.GLL` | Geographic position (latitude/longitude) |
| ZDA | `nmea.ZDA` | Date and time |
| TXT | `nmea.TXT` | Text message |

## Get UTC Time

```go
t, ok := r.Time()             // Reader caches latest time automatically
t, ok := nmea.TimeFromSentence(s) // Extract time from a single sentence
```

## Design Principles

- **Zero external dependencies** — Core parser relies on no third-party libraries
- **Value types** — All sentences return value types, no pointers
- **Pure parsing** — Not bundled with serial implementations, users are free to choose communication method

## NMEA Protocol Tutorial

[GPS/NMEA Protocol for Beginners](https://lsong.org/notes/gps.html)

## Other Language Versions

- Node.js: [lsongdev/node-gps](https://github.com/lsongdev/node-gps)
