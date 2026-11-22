// 上传图缩略图并发回归：从真实 bootstrap 收集图片，32 路并发请求，
// 同时验证状态码、内容类型、响应体积与耗时；不会修改 /data 或 /config。
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const CONCURRENCY = Number(process.env.CONCURRENCY || 32)

const bootstrapResponse = await fetch(`${BASE}/api/bootstrap`, {
  headers: { Accept: 'application/json' },
})
if (!bootstrapResponse.ok) {
  throw new Error(`bootstrap 失败：HTTP ${bootstrapResponse.status}`)
}
const data = await bootstrapResponse.json()
const uploads = [...new Set(
  (data.dishes || [])
    .map((dish) => dish.image || '')
    .filter((url) => url.startsWith('/uploads/'))
    .map((url) => url.slice('/uploads/'.length))
    .filter((name) => name && !name.includes('/') && !name.includes('\\')),
)]

function thumbURL(name) {
  const dot = name.lastIndexOf('.')
  return `${BASE}/uploads/thumb/${dot > 0 ? name.slice(0, dot) : name}.jpg`
}

const results = new Array(uploads.length)
let next = 0
const started = performance.now()

async function worker() {
  while (true) {
    const index = next++
    if (index >= uploads.length) return
    const name = uploads[index]
    const begin = performance.now()
    try {
      const response = await fetch(thumbURL(name), {
        signal: AbortSignal.timeout(20_000),
      })
      const body = await response.arrayBuffer()
      results[index] = {
        name,
        status: response.status,
        type: response.headers.get('content-type') || '',
        bytes: body.byteLength,
        ms: performance.now() - begin,
      }
    } catch (error) {
      results[index] = {
        name,
        status: 0,
        type: '',
        bytes: 0,
        ms: performance.now() - begin,
        error: error.message,
      }
    }
  }
}

await Promise.all(Array.from({ length: Math.min(CONCURRENCY, uploads.length) }, worker))
const elapsed = performance.now() - started
const durations = results.map((item) => item.ms).sort((a, b) => a - b)
const percentile = (value) => Math.round(durations[Math.min(durations.length - 1, Math.floor(durations.length * value))] || 0)
const failures = results.filter(
  (item) => item.status !== 200 || item.bytes <= 0 || !item.type.startsWith('image/'),
)
const totalBytes = results.reduce((sum, item) => sum + item.bytes, 0)

console.log(JSON.stringify({
  pass: uploads.length > 0 && failures.length === 0,
  base: BASE,
  concurrency: Math.min(CONCURRENCY, uploads.length),
  images: uploads.length,
  failed: failures.length,
  elapsedMs: Math.round(elapsed),
  averageMs: Math.round(elapsed / Math.max(1, uploads.length)),
  p50Ms: percentile(0.5),
  p95Ms: percentile(0.95),
  maxMs: Math.round(durations.at(-1) || 0),
  totalKb: Math.round(totalBytes / 1024),
  failures: failures.slice(0, 10),
}, null, 2))

if (uploads.length === 0 || failures.length) process.exit(1)
