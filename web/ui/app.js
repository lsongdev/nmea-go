// The GNSS panel: top navigation and the page for the location hash.
import { html, render } from './lib/html.js';
import { Navbar, DialogHost, Toaster } from './lib/ui.js';
import { createRoutes, href, useLocation } from './lib/router.js';

const nav = [
  { path: '/', title: 'Position' },
  { path: '/satellites', title: 'Satellites' },
  { path: '/sentences', title: 'Sentences' },
];

const Routes = createRoutes([
  { path: '/', load: () => import('./pages/position.js') },
  { path: '/satellites', load: () => import('./pages/satellites.js') },
  { path: '/sentences', load: () => import('./pages/sentences.js') },
], { notFound: 'Page not found' });

function App() {
  const { path, query } = useLocation();
  // The longest matching entry is active; "/" only matches exactly.
  const active = nav
    .filter(i => path === i.path || (i.path !== '/' && path.startsWith(i.path + '/')))
    .sort((a, b) => b.path.length - a.path.length)[0];
  return html`
    <${Navbar} brand="GNSS" mark="◆" label="Main"
      items=${nav.map(i => ({ href: href(i.path), title: i.title, active: i === active }))} />
    <main class="content"><${Routes} path=${path} query=${query} /></main>
    <${DialogHost} />
    <${Toaster} />`;
}

render(html`<${App} />`, document.getElementById('app'));
