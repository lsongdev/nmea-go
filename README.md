# nmea-go

Pure Go NMEA 0183 protocol parser library with zero dependencies, supporting GPS/BeiDou/GLONASS standard sentences.

## Quick Start

```go
import "github.com/lsongdev/nmea-go/nmea"

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

## HTTP API and Web UI

`cmd/nmea-server` reads a serial port, a TCP stream or a recorded file and serves the live state with a browser UI (Preact + HTM, no build step):

```sh
go run ./cmd/nmea-server -source /dev/ttyUSB0 -baud 9600
go run ./cmd/nmea-server -source tcp://192.168.1.10:10110
go run ./cmd/nmea-server -source testdata/sample.nmea   # replay, 1 fix/s
```

Open http://localhost:8080. Set `-token` (or `NMEA_TOKEN`) to require a Bearer token / basic-auth password.

| Endpoint | Description |
|----------|-------------|
| `GET /api/state` | Everything below in one snapshot |
| `GET /api/fix` | Merged position fix (RMC/GGA/GNS/GSA/GST/VTG/ZDA) |
| `GET /api/satellites` | Satellites in view, with system and whether used in the fix |
| `GET /api/status` | Source state and sentence counters |
| `GET /api/sentences?after=&limit=&type=` | Recent raw sentences |
| `POST /api/parse` | Decode `{"sentence": "$..."}` without logging it |
| `GET /api/events?sentences=1` | Server-sent events: `state` on change, `sentence` per line |

The `api` package (`api.NewTracker`, `api.NewHandler`) and `web` package (`web.Standalone`, `web.Assets`) can be embedded in other programs.

## Get UTC Time

```go
t, ok := r.Time()                  // latest date+time, advanced by GGA/GLL/GNS/GST times of day
t, ok := nmea.TimeFromSentence(s)  // complete date+time: RMC (with a fix) or ZDA
tod, ok := nmea.TimeOfDay(s)       // time of day only, from any timed sentence
t = nmea.MergeTime(last, tod)      // put a time of day on a known date (handles midnight)
```

## Errors

`Parse` and `Reader.Read` return errors wrapping `nmea.ErrFormat` (malformed or truncated) or `nmea.ErrChecksum`; after one, `Read` can be called again. Proprietary sentences (`$PUBX`, `$PMTK…`) parse as `UnknownSentence` with talker `"P"`.

## Design Principles

- **Zero external dependencies** — Core parser relies on no third-party libraries
- **Value types** — All sentences return value types, no pointers
- **Pure parsing** — Not bundled with serial implementations, users are free to choose communication method

## NMEA Protocol Tutorial

[GPS/NMEA Protocol for Beginners](https://lsong.org/notes/gps.html)

## Other Language Versions

- Node.js: [lsongdev/node-gps](https://github.com/lsongdev/node-gps)
