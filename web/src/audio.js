// 骰子 PK 音效：全部用浏览器原生 Web Audio API 实时合成。
// 不加载、不分发、不引用任何外部音频文件，也不新增依赖（版权硬约束）。
// 只有振荡器 / 带通白噪声 + 包络，声音在用户点击的那一刻才被合成出来。

const STORAGE_KEY = 'fm.sound'
const MASTER_VOLUME = 0.22

let ctx = null
let master = null
let noiseBuffer = null
let unlockBound = false
let lastRollAt = 0
let muted = readMuted()

function readMuted() {
  try { return localStorage.getItem(STORAGE_KEY) === 'off' } catch { return false }
}

export function isMuted() {
  return muted
}

export function setMuted(value) {
  muted = !!value
  try { localStorage.setItem(STORAGE_KEY, muted ? 'off' : 'on') } catch { /* 隐私模式忽略 */ }
  if (master) master.gain.value = muted ? 0 : MASTER_VOLUME
  return muted
}

export function toggleMuted() {
  return setMuted(!muted)
}

// 自动播放策略要求 AudioContext 在用户手势里创建/恢复。全局只解锁一次。
export function installAudioUnlock() {
  if (unlockBound) return
  unlockBound = true
  const unlock = () => {
    ensureContext()
    window.removeEventListener('pointerdown', unlock, true)
    window.removeEventListener('keydown', unlock, true)
  }
  window.addEventListener('pointerdown', unlock, true)
  window.addEventListener('keydown', unlock, true)
}

function ensureContext() {
  const AudioContextClass = window.AudioContext || window.webkitAudioContext
  if (!AudioContextClass) return null
  if (!ctx) {
    ctx = new AudioContextClass()
    master = ctx.createGain()
    master.gain.value = muted ? 0 : MASTER_VOLUME
    master.connect(ctx.destination)
  }
  if (ctx.state === 'suspended') ctx.resume().catch(() => {})
  return ctx
}

function ready() {
  if (muted) return false
  return !!ensureContext()
}

function whiteNoise() {
  if (noiseBuffer) return noiseBuffer
  const length = Math.floor(ctx.sampleRate * 0.5)
  noiseBuffer = ctx.createBuffer(1, length, ctx.sampleRate)
  const data = noiseBuffer.getChannelData(0)
  for (let i = 0; i < length; i++) data[i] = Math.random() * 2 - 1
  return noiseBuffer
}

// 单个带衰减包络的振荡器音
function tone(freq, at, dur, { type = 'sine', gain = 0.3, attack = 0.008, detune = 0 } = {}) {
  const osc = ctx.createOscillator()
  const env = ctx.createGain()
  osc.type = type
  osc.frequency.setValueAtTime(freq, at)
  if (detune) osc.detune.setValueAtTime(detune, at)
  env.gain.setValueAtTime(0.0001, at)
  env.gain.exponentialRampToValueAtTime(gain, at + attack)
  env.gain.exponentialRampToValueAtTime(0.0001, at + dur)
  osc.connect(env)
  env.connect(master)
  osc.start(at)
  osc.stop(at + dur + 0.02)
}

// 短促带通噪声：骰子磕在桌面上的「哒」声
function knock(at, { freq = 1400, gain = 0.22, dur = 0.055, q = 1.4 } = {}) {
  const src = ctx.createBufferSource()
  src.buffer = whiteNoise()
  const band = ctx.createBiquadFilter()
  band.type = 'bandpass'
  band.frequency.setValueAtTime(freq, at)
  band.Q.value = q
  const env = ctx.createGain()
  env.gain.setValueAtTime(0.0001, at)
  env.gain.exponentialRampToValueAtTime(gain, at + 0.006)
  env.gain.exponentialRampToValueAtTime(0.0001, at + dur)
  src.connect(band)
  band.connect(env)
  env.connect(master)
  src.start(at)
  src.stop(at + dur + 0.02)
}

// 摇骰子：由密到疏的一串碰撞声，末尾一声闷响收住
export function playRoll() {
  if (!ready()) return
  const now = Date.now()
  if (now - lastRollAt < 260) return // 平局连续自动加摇时不要叠成一团
  lastRollAt = now
  const t = ctx.currentTime + 0.01
  const beats = [0, 0.07, 0.15, 0.24, 0.35, 0.48, 0.63, 0.8]
  beats.forEach((offset, i) => {
    const fade = 1 - i / (beats.length + 1)
    knock(t + offset, {
      freq: 900 + Math.random() * 1100,
      gain: 0.1 + 0.14 * fade,
      dur: 0.045 + Math.random() * 0.025,
    })
  })
  tone(190, t + 0.87, 0.12, { type: 'triangle', gain: 0.12, attack: 0.005 })
}

// 胜利：C 大三和弦上行琶音 + 收尾和弦，明亮欢快
export function playWin() {
  if (!ready()) return
  const t = ctx.currentTime + 0.02
  const notes = [523.25, 659.25, 783.99, 1046.5] // C5 E5 G5 C6
  notes.forEach((f, i) => {
    const at = t + i * 0.085
    tone(f, at, 0.3, { type: 'triangle', gain: 0.2 })
    tone(f * 2, at, 0.18, { type: 'sine', gain: 0.055 })
  })
  const chord = [523.25, 659.25, 783.99]
  chord.forEach((f) => tone(f, t + 0.38, 0.5, { type: 'sine', gain: 0.11 }))
}

// 失败：A 小调下行 + 低音收尾，短而不刺耳
export function playLose() {
  if (!ready()) return
  const t = ctx.currentTime + 0.02
  const notes = [440, 392, 329.63] // A4 G4 E4
  notes.forEach((f, i) => {
    const at = t + i * 0.13
    tone(f, at, 0.26, { type: 'sine', gain: 0.19 - i * 0.03 })
    tone(f / 2, at, 0.22, { type: 'triangle', gain: 0.06 })
  })
  tone(220, t + 0.42, 0.36, { type: 'sine', gain: 0.13 })
}

// 安全过关（既没掌勺也没洗碗的人）：轻轻两声，不抢戏
export function playSafe() {
  if (!ready()) return
  const t = ctx.currentTime + 0.02
  tone(783.99, t, 0.2, { type: 'sine', gain: 0.12 })
  tone(1046.5, t + 0.11, 0.28, { type: 'sine', gain: 0.11 })
}
