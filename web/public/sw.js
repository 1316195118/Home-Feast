// 家里吃点啥 · 离线缓存
// /api/ 一律走网络；页面导航走 network-first；带 hash 的构建资源走 cache-first；
// 菜品图片走 stale-while-revalidate：先秒开缓存，再在后台校验并更新。
const CACHE = 'fm-shell-v3'
const SHELL = ['/', '/index.html', '/manifest.json', '/icon-192.png', '/icon-512.png', '/dishes/homestyle.webp']

function isHashedAsset(pathname) {
  return pathname.startsWith('/assets/') && /\.[a-z0-9_-]{8,}\.(?:js|css|woff2?)$/i.test(pathname)
}

function isDishImage(pathname) {
  return pathname.startsWith('/uploads/') ||
    pathname.startsWith('/dish-photo/') ||
    pathname.startsWith('/pantry/') ||
    pathname.startsWith('/dishes/')
}

async function putIfFresh(cache, request, response) {
  if (response && response.ok && response.status === 200 && response.type === 'basic') {
    await cache.put(request, response.clone())
  }
  return response
}

async function networkFirst(request, fallbackKey = request) {
  try {
    const response = await fetch(request)
    if (response.ok) {
      const cache = await caches.open(CACHE)
      await putIfFresh(cache, request, response)
    }
    return response
  } catch {
    return (await caches.match(fallbackKey)) || (await caches.match('/index.html')) || Response.error()
  }
}

async function cacheFirst(request) {
  const hit = await caches.match(request)
  if (hit) return hit
  const response = await fetch(request)
  const cache = await caches.open(CACHE)
  return putIfFresh(cache, request, response)
}

async function staleWhileRevalidate(request, event) {
  const cached = await caches.match(request)
  const network = fetch(request)
    .then(async (response) => {
      const cache = await caches.open(CACHE)
      return putIfFresh(cache, request, response)
    })
    .catch(() => null)
  // 缓存命中时立即返回，同时让浏览器把后台刷新任务跑完。
  if (cached) {
    event.waitUntil(network)
    return cached
  }
  return (await network) || Response.error()
}

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
    event.respondWith(networkFirst(req, '/index.html'))
    return
  }

  if (isDishImage(url.pathname)) {
    event.respondWith(staleWhileRevalidate(req, event))
    return
  }

  if (isHashedAsset(url.pathname)) {
    event.respondWith(cacheFirst(req))
    return
  }

  // manifest、图标等固定名称资源需要短周期校验，避免发版后长期命中旧文件。
  // 静态资源失败时只回退自身缓存，绝不能把 index.html 冒充图片或脚本。
  event.respondWith(networkFirst(req))
})
