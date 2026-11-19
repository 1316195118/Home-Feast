function isLanHost(host) {
  if (typeof window === 'undefined') return true
  const h = String(host || window.location.hostname || '').toLowerCase()
  if (h === 'localhost' || h === '127.0.0.1' || h === '::1') return true
  const p = h.split('.').map(Number)
  if (p.length !== 4 || p.some((n) => !Number.isInteger(n) || n < 0 || n > 255)) return false
  if (p[0] === 10) return true
  if (p[0] === 192 && p[1] === 168) return true
  if (p[0] === 172 && p[1] >= 16 && p[1] <= 31) return true
  return false
}
function imageSize(mode) {
  const m = String(mode || 'auto')
  if (m === 'full') return ''
  if (m === 'thumb') return 'thumb'
  return isLanHost() ? '' : 'thumb'
}
console.log(isLanHost('10.10.10.251'), isLanHost('192.168.1.1'), isLanHost('172.16.0.1'), isLanHost('172.31.255.255'), isLanHost('172.32.0.1'), isLanHost('8.8.8.8'), isLanHost('family.example.com'), isLanHost('localhost'), isLanHost('127.0.0.1'));
console.log(imageSize('full'), imageSize('thumb'), imageSize('auto'));
