// 家里吃点啥 · 离线缓存
// 只缓存静态 app shell / 资源；/api/ 一律走网络，绝不把点菜数据缓存成旧快照。
// 带内容 hash 的 assets 用 cache-first，页面导航用 network-first，离线时回退缓存入口。
const CACHE = 'fm-shell-v1'
const SHELL = ['/', '/index.html', '/manifest.json', '/icon-192.png', '/icon-512.png']

self.addEventListener('install', (event) => {
  // 注意：这里不再 skipWaiting()。新版本下载完后进入 waiting，
  // 等页面上的「更新」按钮发来 SKIP_WAITING 才接管，避免无声切换打断正在点菜的人。
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(SHELL)))
})

// 收到页面「立即接管」指令后，让等待中的新版本生效
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return
  const url = new URL(req.url)
  if (url.origin !== self.location.origin) return
  if (url.pathname.startsWith('/api/')) return // 数据是实时的，不缓存

  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req)
        .then((res) => {
          const copy = res.clone()
          caches.open(CACHE).then((cache) => cache.put('/index.html', copy))
          return res
        })
        .catch(() => caches.match('/index.html')),
    )
    return
  }

  event.respondWith(
    caches.match(req).then((hit) => hit || fetch(req).then((res) => {
      if (res && res.status === 200 && res.type === 'basic') {
        const copy = res.clone()
        caches.open(CACHE).then((cache) => cache.put(req, copy))
      }
      return res
    }).catch(() => caches.match('/index.html'))),
  )
})
