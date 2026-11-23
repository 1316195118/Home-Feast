// 图片分享与复制工具：优先调用浏览器原生分享面板；
// 不支持文件分享的微信内置浏览器 / 局域网 HTTP 页面，由界面给出可靠降级。

export function isWeChatBrowser() {
  return typeof navigator !== 'undefined' && /MicroMessenger/i.test(navigator.userAgent)
}

export function canShareImageFile(file) {
  try {
    return !!file
      && typeof navigator?.share === 'function'
      && typeof navigator?.canShare === 'function'
      && navigator.canShare({ files: [file] })
  } catch {
    return false
  }
}

export function dataUrlToFile(dataUrl, filename) {
  const match = /^data:([^;,]+)(;base64)?,(.*)$/s.exec(String(dataUrl || ''))
  if (!match) throw new Error('图片数据无效，请重新生成')
  const [, mime = 'image/png', base64, payload] = match
  let bytes
  if (base64) {
    const binary = atob(payload)
    bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  } else {
    bytes = new TextEncoder().encode(decodeURIComponent(payload))
  }
  return new File([bytes], filename, { type: mime })
}

export async function shareImageFile(file, { title = '', text = '' } = {}) {
  if (!canShareImageFile(file)) return 'unsupported'
  try {
    await navigator.share({ files: [file], title, text })
    return 'shared'
  } catch (err) {
    if (err?.name === 'AbortError') return 'cancelled'
    throw err
  }
}

export async function copyPlainText(text) {
  const value = String(text || '')
  try {
    await navigator.clipboard.writeText(value)
    return true
  } catch {
    const area = document.createElement('textarea')
    area.value = value
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.left = '-9999px'
    area.style.top = '0'
    document.body.appendChild(area)
    area.select()
    area.setSelectionRange(0, area.value.length)
    let ok = false
    try { ok = document.execCommand('copy') } catch { /* 浏览器不支持时返回失败 */ }
    area.remove()
    return ok
  }
}

export function downloadDataUrl(dataUrl, filename) {
  const link = document.createElement('a')
  link.href = dataUrl
  link.download = filename
  link.rel = 'noopener'
  document.body.appendChild(link)
  link.click()
  link.remove()
}
