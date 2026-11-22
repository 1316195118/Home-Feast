import { reactive } from 'vue'

// 缩略图预取后保存为会话内 Blob URL。HTTP 缓存是否复用由浏览器策略决定，
// 而 Blob URL 能被当前页面直接解码，保证连续滚动时不会再次排队等网络。
const blobs = reactive(new Map())

export function cachedThumb(url) {
  return blobs.get(url) || ''
}

export function cacheThumb(url, blob) {
  if (!url || !blob || blobs.has(url)) return
  blobs.set(url, URL.createObjectURL(blob))
}

export function clearThumbCache() {
  for (const url of blobs.values()) URL.revokeObjectURL(url)
  blobs.clear()
}

export async function invalidateThumbCache() {
  clearThumbCache()
  try {
    if (!('caches' in window)) return
    const cache = await caches.open('fm-shell-v4')
    const keys = await cache.keys()
    await Promise.all(keys
      .filter((request) => new URL(request.url).pathname.startsWith('/__thumb-bundle/'))
      .map((request) => cache.delete(request)))
  } catch {
    // 浏览器禁用 Cache Storage 时只需清掉当前会话的 Blob。
  }
}
