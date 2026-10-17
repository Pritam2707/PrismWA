const CACHE_NAME = 'prismwa-shell-v1';
const APP_SHELL = [
  '/',
  '/index.html',
  '/styles.css',
  '/theme.css',
  '/app.js',
  '/ux.js',
  '/logo.png',
  '/favicon.png',
  '/manifest.json',
  '/icons/icon-192.png',
  '/icons/icon-512.png',
  '/icons/icon-maskable-192.png',
  '/icons/icon-maskable-512.png'
];

// Install: precache app shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(APP_SHELL);
    }).then(() => self.skipWaiting())
  );
});

// Activate: purge outdated caches and take control
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.map((key) => {
          if (key !== CACHE_NAME) {
            return caches.delete(key);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch: Network-first for dynamic / API routes, Stale-while-revalidate for static shell
self.addEventListener('fetch', (event) => {
  const request = event.request;
  const url = new URL(request.url);

  // Never intercept non-GET requests or WebSocket upgrades
  if (request.method !== 'GET') {
    return;
  }

  // Live telemetry and backend APIs: Strictly Network-Only (no caching)
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/ws')) {
    return;
  }

  // Navigation requests (HTML document): Network-First, fallback to cached index.html
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response && response.status === 200) {
            const copy = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
          }
          return response;
        })
        .catch(async () => {
          const cached = await caches.match(request);
          if (cached) return cached;
          return caches.match('/index.html');
        })
    );
    return;
  }

  // Google Fonts or static assets: Stale-While-Revalidate
  const isStaticAsset =
    url.origin === self.location.origin &&
    (url.pathname.match(/\.(css|js|png|jpg|jpeg|svg|webp|woff2?|ttf|ico|json)$/) ||
      APP_SHELL.includes(url.pathname));

  const isFont = url.hostname === 'fonts.googleapis.com' || url.hostname === 'fonts.gstatic.com';

  if (isStaticAsset || isFont) {
    event.respondWith(
      caches.match(request).then((cachedResponse) => {
        const fetchPromise = fetch(request)
          .then((networkResponse) => {
            if (networkResponse && networkResponse.status === 200) {
              const responseToCache = networkResponse.clone();
              caches.open(CACHE_NAME).then((cache) => {
                cache.put(request, responseToCache);
              });
            }
            return networkResponse;
          })
          .catch(() => cachedResponse);

        return cachedResponse || fetchPromise;
      })
    );
    return;
  }

  // Default: Network with cache fallback
  event.respondWith(
    fetch(request).catch(() => caches.match(request))
  );
});
