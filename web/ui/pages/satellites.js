import { html } from '../lib/html.js';
import { Page, Notice, Loading, Empty, Table, Badge } from '../lib/ui.js';
import { useLive, SourceNotice, SYSTEMS, sysClass } from '../nmea.js';

// SNR is drawn on a scale up to this many dB-Hz.
const SNR_MAX = 55;

export default function Satellites() {
  const { snap, error } = useLive();
  if (!snap) return html`<${Page} title="Satellites">${error ? html`<${Notice} error=${error} />` : html`<${Loading} />`}<//>`;
  const sats = snap.satellites;
  const systems = SYSTEMS.filter(s => sats.some(v => v.system === s));
  const used = sats.filter(s => s.used).length;
  return html`<${Page} title="Satellites" subtitle=${`${used} used of ${sats.length} in view`}>
    <${Notice} error=${error} />
    <${SourceNotice} status=${snap.status} />
    ${sats.length === 0 ? html`<${Empty}>No satellites in view yet (GSV sentences).<//>` : html`
      <div class="grid-2">
        <div class="card">
          <h2>Sky</h2>
          <${SkyPlot} sats=${sats} />
          <${Legend} systems=${systems} sats=${sats} />
        </div>
        <div class="card">
          <h2>Signal strength</h2>
          <${SignalBars} sats=${sats} />
        </div>
      </div>
      <div class="section">
        <${Table}>
          <thead><tr><th>System</th><th>PRN</th><th class="num">Elevation</th><th class="num">Azimuth</th><th class="num">SNR</th><th>In fix</th></tr></thead>
          <tbody>${sats.map(s => html`<tr key=${s.talker + s.prn}>
            <td><span class=${`sys-dot ${sysClass(s.system)}`}></span>${s.system}</td>
            <td class="mono">${s.prn}</td>
            <td class="num">${s.elevation}°</td>
            <td class="num">${s.azimuth}°</td>
            <td class="num">${s.snr ? `${s.snr} dB-Hz` : html`<span class="muted">not tracked</span>`}</td>
            <td>${s.used ? html`<${Badge} tone="green">Used<//>` : html`<span class="muted">—</span>`}</td>
          </tr>`)}</tbody>
        <//>
      </div>`}
  <//>`;
}

function Legend({ systems, sats }) {
  return html`<div class="legend sky-legend">
    ${systems.map(s => {
      const all = sats.filter(v => v.system === s);
      return html`<span key=${s}><i class=${`sys-dot ${sysClass(s)}`}></i>${s} ${all.filter(v => v.used).length}/${all.length}</span>`;
    })}
    <span class="muted">Filled: used in the fix</span>
  </div>`;
}

const R = 100;

// SkyPlot draws satellites by azimuth (north up, clockwise) and elevation
// (zenith at the centre, horizon at the edge).
function SkyPlot({ sats }) {
  const pos = s => {
    const r = R * (1 - Math.max(0, Math.min(90, s.elevation)) / 90);
    const a = s.azimuth * Math.PI / 180;
    return [r * Math.sin(a), -r * Math.cos(a)];
  };
  return html`<svg class="sky" viewBox="-118 -118 236 236" role="img" aria-label="Sky plot of satellites in view">
    ${[0, 30, 60].map(el => html`<circle key=${el} class="sky-ring" r=${R * (1 - el / 90)} />`)}
    <line class="sky-ring" x1="0" y1=${-R} x2="0" y2=${R} />
    <line class="sky-ring" x1=${-R} y1="0" x2=${R} y2="0" />
    ${[['N', 0, -R - 8], ['E', R + 8, 0], ['S', 0, R + 8], ['W', -R - 8, 0]].map(([t, x, y]) =>
      html`<text key=${t} class="sky-label" x=${x} y=${y} text-anchor="middle" dominant-baseline="central">${t}</text>`)}
    <text class="sky-el" x="3" y=${-R * 2 / 3 + 9}>30°</text>
    <text class="sky-el" x="3" y=${-R / 3 + 9}>60°</text>
    ${sats.map(s => {
      const [x, y] = pos(s);
      return html`<g key=${s.talker + s.prn} class=${`sky-sat ${sysClass(s.system)} ${s.used ? 'used' : ''} ${s.snr ? '' : 'untracked'}`}>
        <title>${s.system} ${s.prn}: elevation ${s.elevation}°, azimuth ${s.azimuth}°, ${s.snr ? `${s.snr} dB-Hz` : 'not tracked'}${s.used ? ', used' : ''}</title>
        <circle cx=${x} cy=${y} r="7" />
        <text x=${x} y=${y} text-anchor="middle" dominant-baseline="central">${s.prn}</text>
      </g>`;
    })}
  </svg>`;
}

function SignalBars({ sats }) {
  return html`<div class="bars" role="img" aria-label="Signal-to-noise ratio of each satellite">
    ${sats.map(s => html`<div key=${s.talker + s.prn} class=${`bar ${sysClass(s.system)} ${s.used ? 'used' : ''}`}
      title=${`${s.system} ${s.prn}: ${s.snr ? `${s.snr} dB-Hz` : 'not tracked'}`}>
      <span class="bar-value">${s.snr || ''}</span>
      <span class="bar-fill" style=${{ height: `${Math.min(100, (s.snr / SNR_MAX) * 100)}%` }}></span>
      <span class="bar-label">${s.prn}</span>
    </div>`)}
  </div>`;
}
