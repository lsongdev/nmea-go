import { html, useEffect, useRef, useState } from '../lib/html.js';
import { usePanel } from '../lib/panel.js';
import { useAction } from '../lib/hooks.js';
import { Page, Notice, LogView, Table, Field } from '../lib/ui.js';
import { useLive, SourceNotice } from '../nmea.js';

// The view keeps this many lines.
const KEEP = 300;

const DESCRIPTIONS = {
  RMC: 'Recommended minimum: position, speed, date',
  GGA: 'Fix data: quality, satellites, altitude',
  GSA: 'DOP and satellites in the fix',
  GSV: 'Satellites in view',
  GNS: 'Multi-constellation fix data',
  GST: 'Position error statistics',
  VTG: 'Course and ground speed',
  GLL: 'Latitude and longitude',
  ZDA: 'Date and time',
  TXT: 'Text message',
};

export default function Sentences({ query, setQuery }) {
  const { api } = usePanel();
  const type = query.type || '';
  const [lines, setLines] = useState([]);
  const [paused, setPaused] = useState(false);
  const pausedRef = useRef(paused);
  pausedRef.current = paused;
  const typeRef = useRef(type);
  typeRef.current = type;

  // Start from the recent history, then follow the stream.
  useEffect(() => {
    const controller = new AbortController();
    api.get('sentences', { query: { limit: KEEP, type }, signal: controller.signal })
      .then(recent => setLines(old => merge(recent, old.filter(l => !type || l.type === type))))
      .catch(() => {});
    return () => controller.abort();
  }, [type]);

  const { snap, error } = useLive(line => {
    if (pausedRef.current || (typeRef.current && line.type !== typeRef.current)) return;
    setLines(old => merge(old, [line]));
  });

  const counts = Object.entries(snap?.status.counts || {}).sort((a, b) => b[1] - a[1]);
  const total = snap?.status.total || 0;
  return html`<${Page} title="Sentences" subtitle=${snap ? `${total.toLocaleString()} received${snap.status.invalid ? `, ${snap.status.invalid.toLocaleString()} invalid` : ''}` : null}>
    <${Notice} error=${error} />
    <${SourceNotice} status=${snap?.status} />
    <div class="toolbar">
      <select class="input" value=${type} onChange=${e => { setLines([]); setQuery({ type: e.currentTarget.value }); }} aria-label="Sentence type">
        <option value="">All types</option>
        ${counts.map(([t]) => html`<option key=${t} value=${t}>${t}</option>`)}
      </select>
      <button class="button" type="button" onClick=${() => setPaused(p => !p)}>${paused ? 'Resume' : 'Pause'}</button>
      <button class="button button-ghost" type="button" onClick=${() => setLines([])}>Clear</button>
      <span class="spacer"></span>
      <span class="muted">${paused ? 'Paused' : 'Live'} · ${lines.length} lines</span>
    </div>
    <${LogView} lines=${lines.map(l => (l.error ? { text: `${l.raw}    ← ${l.error}`, stream: 'stderr' } : l.raw))}
      follow=${!paused} empty="Waiting for sentences…" />
    <div class="grid-2 section">
      <div>
        <h2 class="section-title">By type</h2>
        <${Table}>
          <thead><tr><th>Type</th><th>Description</th><th class="num">Count</th><th class="num">Share</th></tr></thead>
          <tbody>${counts.map(([t, n]) => html`<tr key=${t}>
            <td><a href="#" onClick=${e => { e.preventDefault(); setLines([]); setQuery({ type: t }); }} class="mono">${t}</a></td>
            <td class="muted">${DESCRIPTIONS[t] || 'Not decoded'}</td>
            <td class="num">${n.toLocaleString()}</td>
            <td class="num">${total ? `${((n / total) * 100).toFixed(1)}%` : '—'}</td>
          </tr>`)}</tbody>
        <//>
      </div>
      <${ParseTool} />
    </div>
  <//>`;
}

// merge appends lines not yet shown, by sequence number, keeping the last KEEP.
function merge(old, add) {
  const last = old.length ? old[old.length - 1].seq : 0;
  const next = old.concat(add.filter(l => l.seq > last));
  return next.length > KEEP ? next.slice(-KEEP) : next;
}

function ParseTool() {
  const { api } = usePanel();
  const [sentence, setSentence] = useState('$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A');
  const [result, setResult] = useState(null);
  const { busy, error, run } = useAction();
  const submit = e => {
    e.preventDefault();
    setResult(null);
    run(async () => setResult(await api.post('parse', { sentence })));
  };
  return html`<form class="card form" onSubmit=${submit}>
    <h2>Decode a sentence</h2>
    <${Field} label="Sentence" hint="Checked and decoded on the server; not added to the log.">
      <textarea class="input mono" rows="2" value=${sentence} onInput=${e => setSentence(e.currentTarget.value)} spellcheck="false"></textarea>
    <//>
    <div class="form-actions">
      <button class="button button-primary" type="submit" disabled=${busy || !sentence.trim()}>Decode</button>
    </div>
    <${Notice} error=${error} />
    ${result ? html`<pre class="decoded">${JSON.stringify(result.data, null, 2)}</pre>` : null}
  </form>`;
}
