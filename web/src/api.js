// 轻量接口封装：同源 fetch、统一错误处理、可选访问口令。
const STORAGE_KEY = 'fm.accessCode'

let accessCode = localStorage.getItem(STORAGE_KEY) || ''

export function setAccessCode(value) {
  accessCode = (value || '').trim()
  if (accessCode) localStorage.setItem(STORAGE_KEY, accessCode)
  else localStorage.removeItem(STORAGE_KEY)
}

export function getAccessCode() {
  return accessCode
}

async function request(path, { method = 'GET', body, isForm = false, raw = false } = {}) {
  const headers = {}
  if (accessCode) headers['X-Access-Code'] = accessCode

  let payload
  if (body !== undefined) {
    if (isForm) {
      payload = body
    } else {
      headers['Content-Type'] = 'application/json'
      payload = JSON.stringify(body)
    }
  }

  const res = await fetch(path, { method, headers, body: payload })
  if (res.status === 401) {
    window.dispatchEvent(new Event('fm:locked'))
    throw new Error('访问口令不正确')
  }
  if (raw) {
    if (!res.ok) throw new Error(`请求失败 (${res.status})`)
    return res.blob()
  }
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || `请求失败 (${res.status})`)
  return data
}

export const api = {
  config: () => request('/api/config'),
  bootstrap: () => request('/api/bootstrap'),
  plans: () => request('/api/plans'),

  addDish: (dish) => request('/api/dishes', { method: 'POST', body: dish }),
  updateDish: (id, patch) => request(`/api/dishes/${id}`, { method: 'PUT', body: patch }),
  deleteDish: (id) => request(`/api/dishes/${id}`, { method: 'DELETE' }),

  addMember: (member) => request('/api/members', { method: 'POST', body: member }),
  updateMember: (id, patch) => request(`/api/members/${id}`, { method: 'PUT', body: patch }),
  deleteMember: (id) => request(`/api/members/${id}`, { method: 'DELETE' }),
  setMemberPin: (id, body) => request(`/api/members/${id}/pin`, { method: 'POST', body }),
  verifyMemberPin: (id, pin) => request(`/api/members/${id}/verify`, { method: 'POST', body: { pin } }),

  setPlan: (date, patch) => request(`/api/plans/${date}`, { method: 'PUT', body: patch }),
  addOrder: (date, body) => request(`/api/plans/${date}/orders`, { method: 'POST', body }),
  toggleOrder: (date, id) => request(`/api/plans/${date}/orders/${id}`, { method: 'PUT' }),
  removeOrder: (date, id) => request(`/api/plans/${date}/orders/${id}`, { method: 'DELETE' }),

  saveSettings: (body) => request('/api/settings', { method: 'PUT', body }),
  upload: (form) => request('/api/upload', { method: 'POST', body: form, isForm: true }),
  exportBackup: () => request('/api/backup', { raw: true }),
  exportExcel: () => request('/api/export/excel', { raw: true }),
  importBackup: (form) => request('/api/backup', { method: 'POST', body: form, isForm: true }),
}
