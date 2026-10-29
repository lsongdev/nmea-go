// Shared state and formatting for the pages.
import { html, useEffect, useRef, useState } from './lib/html.js';
import { usePanel } from './lib/panel.js';
import { Badge, Notice } from './lib/ui.js';

// useLive returns the latest /state snapshot, kept current by the /events
// stream. onSentence, when given, receives every raw sentence.
export function useLive(onSentence) {
  const { client } = usePanel();
  const [snap, setSnap] = useState(null);
  const [error, setError] = useState(null);
  const sentence = useRef(onSentence);
  sentence.current = onSentence;
  useEffect(() => {
    const handlers = {
      state: s => { setSnap(s); setError(null); },
      onerror: () => setError(new Error('The live stream closed; reload the page to reconnect.')),
    };
    if (onSentence) handlers.sentence = line => sentence.current?.(line);
    return client.stream('events', onSentence ? { sentences: 1 } : null, handlers);
  }, []);
  return { snap, error };
}

// SourceNotice explains a source that is down or silent.
export function SourceNotice({ status }) {
  if (!status) return null;
  if (!status.connected) {
    return html`<${Notice} tone="warn">Not reading <code>${status.source}</code>${status.error ? `: ${status.error}` : ''}. Retrying…<//>`;
  }
  const silent = status.last_line ? (Date.now() - new Date(status.last_line)) / 1000 : Infinity;
  if (silent > 5) return html`<${Notice} tone="info">Connected to <code>${status.source}</code>, but no sentence has arrived ${status.total ? 'for a while' : 'yet'}. Check the baud rate.<//>`;
  return null;
}

export const FIX_MODES = { 1: 'No fix', 2: '2D', 3: '3D' };

export const QUALITY = {
  0: 'Invalid', 1: 'GPS', 2: 'DGPS', 3: 'PPS', 4: 'RTK fixed', 5: 'RTK float', 6: 'Estimated', 7: 'Manual', 8: 'Simulation',
};

export function FixBadge({ fix }) {
  if (!fix.valid || fix.mode < 2) return html`<${Badge} tone="red" dot>No fix<//>`;
  return html`<${Badge} tone=${fix.mode === 3 ? 'green' : 'amber'} dot>${FIX_MODES[fix.mode] || 'Fix'}<//>`;
}

// SYSTEMS orders the constellations; nmea.css colours them (.sys-<key>).
export const SYSTEMS = ['GPS', 'GLONASS', 'Galileo', 'BeiDou', 'QZSS', 'NavIC', 'SBAS', 'Other'];

export const sysClass = system => `sys-${system.toLowerCase()}`;

export const fixed = (n, digits = 1) => (n == null || Number.isNaN(n) ? '—' : n.toFixed(digits));

// dms formats a coordinate as degrees, minutes and seconds.
export function dms(value, lat) {
  const hemi = lat ? (value >= 0 ? 'N' : 'S') : (value >= 0 ? 'E' : 'W');
  const v = Math.abs(value);
  const d = Math.floor(v);
  const m = Math.floor((v - d) * 60);
  const s = ((v - d) * 60 - m) * 60;
  return `${d}° ${String(m).padStart(2, '0')}′ ${s.toFixed(2).padStart(5, '0')}″ ${hemi}`;
}

const COMPASS = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW'];

export const compass = deg => COMPASS[Math.round(((deg % 360) + 360) % 360 / 22.5) % 16];

// utc formats a receiver time as "2006-01-02 15:04:05 UTC".
export function utc(value) {
  const d = value ? new Date(value) : null;
  if (!d || Number.isNaN(d.getTime()) || d.getUTCFullYear() < 1980) return '—';
  return d.toISOString().replace('T', ' ').replace(/\.\d+Z$/, ' UTC');
}
