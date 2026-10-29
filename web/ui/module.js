// nmea-go entry point (panel module contract, version 1). The host is a
// panel shell, or standalone.js on its own.
import { createMount } from './lib/mount.js';

const page = load => ({ load });

const routes = [
  { path: '/', ...page(() => import('./pages/position.js')) },
  { path: '/satellites', ...page(() => import('./pages/satellites.js')) },
  { path: '/sentences', ...page(() => import('./pages/sentences.js')) },
];

// The API answers plain JSON bodies, not {"data": …}.
export const mount = createMount({ routes, envelope: false });
