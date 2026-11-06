// The GNSS panel: navigation and the page for the location hash.
import { html, render, lazy, useRoute, RouterView, Navbar, Alert, DialogHost, Toaster } from '@lsongdev/scripts/react/index.js';
import { createRouter } from '@lsongdev/scripts/navigation/router.js';

const router = createRouter([
  ['/', lazy(() => import('./pages/position.js'))],
  ['/satellites', lazy(() => import('./pages/satellites.js'))],
  ['/sentences', lazy(() => import('./pages/sentences.js'))],
], { hash: true });
router.start();

const nav = [
  { href: '#/', title: 'Position' },
  { href: '#/satellites', title: 'Satellites' },
  { href: '#/sentences', title: 'Sentences' },
];

function App() {
  const route = useRoute(router);
  return html`
    <${Navbar} className="container" brand="GNSS" mark="◆" label="Main" items=${nav} current=${router.href(route.path)} />
    <main class="container stack">
      <${RouterView} router=${router} notFound=${html`<${Alert} variant="warning">Page not found<//>`} />
    </main>
    <${DialogHost} />
    <${Toaster} />`;
}

render(html`<${App} />`, document.getElementById('app'));
