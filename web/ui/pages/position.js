import { html, useEffect, useRef, useState } from '../lib/html.js';
import { Page, Notice, Loading, Empty, KV, CopyButton } from '../lib/ui.js';
import { useLive, SourceNotice, FixBadge, FIX_MODES, QUALITY, fixed, dms, compass, utc } from '../nmea.js';

const TRACK_POINTS = 600;

export default function Position() {
  const { snap, error } = useLive();
  const track = useTrack(snap?.fix);
  if (!snap) return html`<${Page} title="Position">${error ? html`<${Notice} error=${error} />` : html`<${Loading} />`}<//>`;
  const { fix, status, satellites } = snap;
  const used = satellites.filter(s => s.used).length;
  const hasPos = fix.valid && fix.updated && (fix.lat || fix.lon);
  const accuracy = fix.lat_std || fix.lon_std ? Math.hypot(fix.lat_std, fix.lon_std) : null;
  return html`<${Page} title="Position" subtitle=${utc(fix.time)} actions=${html`<${FixBadge} fix=${fix} />`}>
    <${Notice} error=${error} />
    <${SourceNotice} status=${status} />
    <div class="grid">
      <${Stat} label="Fix" value=${fix.valid ? FIX_MODES[fix.mode] || 'Fix' : 'No fix'}
        sub=${fix.quality ? QUALITY[fix.quality] || `Quality ${fix.quality}` : 'Waiting for satellites'} />
      <${Stat} label="Satellites" value=${`${used || fix.num_sats} / ${satellites.length}`} sub="Used / in view" />
      <${Stat} label="Speed" value=${`${fixed(fix.speed_kmh)} km/h`} sub=${`${fixed(fix.speed)} kn · ${fixed(fix.course, 0)}° ${compass(fix.course)}`} />
      <${Stat} label="Altitude" value=${`${fixed(fix.altitude)} m`} sub=${`Above sea level · geoid ${fixed(fix.geoid_sep)} m`} />
      <${Stat} label="Accuracy" value=${accuracy == null ? '—' : `±${fixed(accuracy)} m`}
        sub=${fix.alt_std ? `Vertical ±${fixed(fix.alt_std)} m` : 'From GST, when sent'} />
      <${Stat} label="Dilution" value=${`HDOP ${fixed(fix.hdop)}`} sub=${`PDOP ${fixed(fix.pdop)} · VDOP ${fixed(fix.vdop)}`} />
    </div>
    <div class="grid-2 section">
      <div class="card">
        <h2>Coordinates</h2>
        ${hasPos ? html`
          <div class="coords mono">${fixed(fix.lat, 6)}, ${fixed(fix.lon, 6)}</div>
          <${KV} items=${[
            ['Latitude', dms(fix.lat, true)],
            ['Longitude', dms(fix.lon, false)],
            ['Course', html`<${Heading} deg=${fix.course} /> ${fixed(fix.course, 1)}° ${compass(fix.course)}`],
            ['UTC', utc(fix.time)],
          ]} />
          <div class="button-row section-sm">
            <${CopyButton} text=${`${fix.lat.toFixed(6)},${fix.lon.toFixed(6)}`} label="Copy coordinates" />
            <a class="button button-small" target="_blank" rel="noopener noreferrer"
              href=${`https://www.openstreetmap.org/?mlat=${fix.lat.toFixed(6)}&mlon=${fix.lon.toFixed(6)}#map=17/${fix.lat.toFixed(6)}/${fix.lon.toFixed(6)}`}>OpenStreetMap ↗</a>
          </div>` : html`<${Empty}>No position yet.<//>`}
      </div>
      <div class="card">
        <div class="card-head"><h2>Track</h2><span class="muted">${track.length} fixes</span></div>
        <${Track} points=${track} />
      </div>
    </div>
    <div class="card section">
      <h2>Receiver</h2>
      <${KV} items=${[
        ['Source', html`<code>${status.source}</code>`],
        ['State', status.connected ? 'Reading' : `Down${status.error ? `: ${status.error}` : ''}`],
        ['Sentences', `${status.total.toLocaleString()}${status.invalid ? ` (${status.invalid.toLocaleString()} invalid)` : ''}`],
        ['Messages', status.messages.length ? html`<div class="messages">${status.messages.slice(-5).map((m, i) => html`<div key=${i} class="mono">${m}</div>`)}</div>` : null],
      ]} />
    </div>
  <//>`;
}

function Stat({ label, value, sub }) {
  return html`<div class="card stat">
    <span class="stat-label">${label}</span>
    <span class="stat-value">${value}</span>
    ${sub ? html`<span class="stat-sub">${sub}</span>` : null}
  </div>`;
}

// Heading is a small arrow pointing along a course.
function Heading({ deg }) {
  return html`<svg class="heading" viewBox="-8 -8 16 16" aria-hidden="true">
    <path d="M0 -7 L4.5 6 L0 3.5 L-4.5 6 Z" transform=${`rotate(${deg || 0})`} />
  </svg>`;
}

// useTrack collects the positions seen while the page is open.
function useTrack(fix) {
  const [points, setPoints] = useState([]);
  const last = useRef(null);
  useEffect(() => {
    if (!fix?.valid || !fix.updated || fix.updated === last.current || (!fix.lat && !fix.lon)) return;
    last.current = fix.updated;
    setPoints(p => [...p.slice(-(TRACK_POINTS - 1)), [fix.lat, fix.lon]]);
  }, [fix?.updated]);
  return points;
}

const W = 400;
const H = 260;
const PAD = 16;

// Track draws positions on a local flat projection, north up, with a
// scale bar.
function Track({ points }) {
  if (points.length < 2) return html`<${Empty}>The track appears as the receiver moves.<//>`;
  const lat0 = points[points.length - 1][0] * Math.PI / 180;
  const xy = points.map(([lat, lon]) => [lon * 111320 * Math.cos(lat0), lat * 110540]);
  const xs = xy.map(p => p[0]);
  const ys = xy.map(p => p[1]);
  const minX = Math.min(...xs), maxX = Math.max(...xs), minY = Math.min(...ys), maxY = Math.max(...ys);
  const span = Math.max(maxX - minX, (maxY - minY) * (W / H), 10); // metres across, at least 10 m
  const scale = (W - 2 * PAD) / span;
  const cx = (minX + maxX) / 2, cy = (minY + maxY) / 2;
  const px = ([x, y]) => [W / 2 + (x - cx) * scale, H / 2 - (y - cy) * scale];
  const path = xy.map(px).map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`).join(' ');
  const [hx, hy] = px(xy[xy.length - 1]);
  const bar = niceLength(span / 4);
  return html`<svg class="track" viewBox=${`0 0 ${W} ${H}`} role="img" aria-label=${`Track of the last ${points.length} fixes`}>
    <path class="track-line" d=${path} />
    <circle class="track-head" cx=${hx} cy=${hy} r="5" />
    <g class="track-scale" transform=${`translate(${PAD} ${H - PAD})`}>
      <path d=${`M0 -4 V0 H${bar * scale} V-4`} />
      <text x="0" y="-8">${bar >= 1000 ? `${bar / 1000} km` : `${bar} m`}</text>
    </g>
    <text class="track-north" x=${W - PAD} y=${PAD + 4} text-anchor="end">N ↑</text>
  </svg>`;
}

// niceLength rounds a length down to 1, 2 or 5 times a power of ten.
function niceLength(m) {
  const p = 10 ** Math.floor(Math.log10(m));
  return [5, 2, 1].map(f => f * p).find(v => v <= m) || p;
}
