<script setup>
import { ref, computed, onUnmounted, inject, watch } from 'vue'
import { useModalBehavior } from '../modal'
import { isMuted, toggleMuted, playRoll, playWin, playLose, playSafe } from '../audio'

const props = defineProps({ members: { type: Array, required: true } })
const emit = defineEmits(['close', 'apply'])
const root = ref(null)
useModalBehavior(() => emit('close'), root)

const { state, actions } = inject('app')
const selfId = computed(() => state.currentUserId)
const selfName = computed(() => props.members.find((m) => m.id === selfId.value)?.name || '你')
const selfMember = computed(() => props.members.find((m) => m.id === selfId.value) || null)
const todayPlan = computed(() => (state.plan?.date === state.today ? state.plan : null))

// 音效开关只影响本机浏览器，存 localStorage；声音全部实时合成，无音频文件
const soundMuted = ref(isMuted())
function switchSound() { soundMuted.value = toggleMuted() }

// 3x3 骰面点位：1~9 对应左上到右下
const PIPS = {
  1: [5],
  2: [1, 9],
  3: [1, 5, 9],
  4: [1, 3, 7, 9],
  5: [1, 3, 5, 7, 9],
  6: [1, 3, 4, 6, 7, 9],
}

// phase: rolling（等人摇）→ tied（打平，自动加摇）→ done（出结果）
const phase = ref('rolling')
const values = ref({})       // id -> 点数，null/缺失表示还没摇
const rollingId = ref(null)  // 当前正在摇的那一位
const tiedIds = ref([])      // 打平需要加摇的人
const shownIds = ref([])     // 后端 pkShown：已看过结果动画的人
const showResultFx = ref(false) // 本次是否播放 PK 动画
const fxOwner = ref('')      // 结果动画归属的用户，只在「换人看」时重置
let tickTimer = null
let stopTimer = null
let applyTimer = null
let tiedTimer = null
let disposed = false

const ranked = computed(() => props.members.map((m) => ({ ...m, value: values.value[m.id] ?? null })))
const hi = computed(() => Math.max(...ranked.value.map((r) => r.value ?? 0)))
const lo = computed(() => Math.min(...ranked.value.map((r) => r.value ?? 6)))
const chef = computed(() => ranked.value.find((r) => r.value === hi.value))
const washer = computed(() => ranked.value.find((r) => r.value === lo.value))
const decided = computed(() =>
  phase.value === 'done' && chef.value && washer.value && chef.value.id !== washer.value.id
)
// 还需要摇的人：普通阶段是没摇过的；打平阶段只有打平的人
const needRoll = computed(() => {
  if (phase.value === 'done') return []
  if (phase.value === 'tied') return tiedIds.value
  return ranked.value.filter((r) => r.value == null).map((r) => r.id)
})
// 本人要摇的 / 别人要摇的（用于提示）
const needSelf = computed(() => needRoll.value.filter((id) => id === selfId.value))
const needOthers = computed(() => needRoll.value.filter((id) => id !== selfId.value))
const needOtherNames = computed(() => needOthers.value
  .map((id) => props.members.find((m) => m.id === id)?.name)
  .filter(Boolean).join('、'))
// 分工已落库且与本次结果一致：只展示结果，不重复提交、不自动关闭
const alreadyDecided = computed(() => {
  const p = todayPlan.value
  return !!(decided.value && p?.chefId && p?.dishwasherId &&
    p.chefId === chef.value?.id && p.dishwasherId === washer.value?.id)
})

// 只有「当前用户 + 轮到自己摇」才能点摇一摇；force 用于平局系统自动加摇
function canRoll(id) {
  return needRoll.value.includes(id) && id === selfId.value
}

// 把当前点数 + 阶段写回后端 Plan：切用户 / 刷新都不丢
async function persist() {
  const dice = {}
  for (const m of props.members) {
    const v = values.value[m.id]
    if (typeof v === 'number') dice[m.id] = v
  }
  await actions.saveDice({ dice, dicePhase: phase.value })
}

function evaluate() {
  const nums = ranked.value.map((r) => r.value)
  if (nums.some((v) => v == null)) return // 还有人没摇，继续等
  const hiCount = nums.filter((n) => n === hi.value).length
  const loCount = nums.filter((n) => n === lo.value).length
  if (hiCount === 1 && loCount === 1) {
    phase.value = 'done'
    tiedIds.value = []
    return
  }
  // 打平：只让点数并列在最大或最小的人自动加摇，其他人锁定
  const tied = ranked.value
    .filter((r) => (r.value === hi.value && hiCount > 1) || (r.value === lo.value && loCount > 1))
    .map((r) => r.id)
  const next = { ...values.value }
  for (const id of tied) next[id] = null
  values.value = next
  tiedIds.value = tied
  phase.value = 'tied'
  autoRerollTied()
}

// 平局时系统自动加摇（无需按钮），逐位顺序摇，全部摇完再判定
function autoRerollTied() {
  const ids = [...tiedIds.value]
  let i = 0
  const next = () => {
    if (disposed) return
    if (i >= ids.length) { evaluate(); return }
    const id = ids[i++]
    rollFor(id, true, next)
  }
  tiedTimer = setTimeout(next, 600)
}

function rollFor(id, force = false, onDone = null) {
  if (disposed || rollingId.value || (!force && !canRoll(id))) return
  playRoll() // 摇动过程音（合成，平局自动加摇时内部有节流）
  clearInterval(tickTimer)
  clearTimeout(stopTimer)
  rollingId.value = id
  tickTimer = setInterval(() => {
    values.value = { ...values.value, [id]: 1 + Math.floor(Math.random() * 6) }
  }, 80)
  stopTimer = setTimeout(async () => {
    clearInterval(tickTimer)
    if (disposed) return
    values.value = { ...values.value, [id]: 1 + Math.floor(Math.random() * 6) }
    rollingId.value = null
    evaluate()
    await persist()
    if (disposed) return
    if (onDone) onDone()
  }, 1050)
}

// 结果动画 + 音效：按「用户维度」每人只播一次，名单落库到 Plan.pkShown。
// 先摇的人当时没看到结果，重新上线时若名单里没有他，仍会补播一次。
function playResultOnce() {
  const id = selfId.value
  // 同一次结果可能被 watch 与初始化各触发一次：只有「换人看」才重置动画，
  // 否则重复触发会把刚设好的播放态又清掉。
  if (fxOwner.value !== id) { showResultFx.value = false; fxOwner.value = id }
  if (!id || shownIds.value.includes(id)) return
  showResultFx.value = true
  if (id === chef.value?.id) playWin()        // 点最大 → 掌勺，欢快
  else if (id === washer.value?.id) playLose() // 点最小 → 洗碗
  else playSafe()                              // 安全过关，轻轻两声
  const next = [...shownIds.value, id]
  shownIds.value = next
  actions.saveDice({ pkShown: next })
}

function handleDecided() {
  if (!decided.value) return
  playResultOnce()
  if (alreadyDecided.value) return // 分工已定，只展示，不再提交也不自动关闭
  clearTimeout(applyTimer)
  applyTimer = setTimeout(() => {
    if (!disposed) emit('apply', { chefId: chef.value.id, dishwasherId: washer.value.id })
  }, 1400)
}
watch(decided, (v) => { if (v) handleDecided() })
// 切换身份：先摇过的人重新上线、且还没看过动画，补播一次
watch(selfId, () => { if (decided.value) playResultOnce() })

async function restart() {
  clearInterval(tickTimer)
  clearTimeout(stopTimer)
  clearTimeout(applyTimer)
  clearTimeout(tiedTimer)
  rollingId.value = null
  values.value = {}
  phase.value = 'rolling'
  tiedIds.value = []
  shownIds.value = []
  showResultFx.value = false
  fxOwner.value = ''
  await actions.saveDice({ dice: {}, dicePhase: 'rolling', pkShown: [] })
}

// 打开弹窗时从后端恢复点数 / 阶段 / 已看名单：跨用户、刷新都不丢
function hydrate() {
  const p = todayPlan.value
  const dice = p?.dice || {}
  const next = {}
  for (const m of props.members) {
    const v = dice[m.id]
    if (typeof v === 'number' && v >= 1 && v <= 6) next[m.id] = v
  }
  values.value = next
  // 只有「标记为 done 且点数齐全」才恢复成结果态，否则退回等待摇，避免卡死
  phase.value = p?.dicePhase === 'done' && Object.keys(next).length === props.members.length
    ? 'done' : 'rolling'
  shownIds.value = Array.isArray(p?.pkShown) ? [...p.pkShown] : []
  if (phase.value !== 'done') evaluate()
}
hydrate()
if (decided.value) handleDecided() // 打开时结果已出（另一人摇完 / 刷新回来）

onUnmounted(() => {
  disposed = true
  clearInterval(tickTimer)
  clearTimeout(stopTimer)
  clearTimeout(applyTimer)
  clearTimeout(tiedTimer)
})
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal pk-modal" role="dialog" aria-modal="true" aria-label="骰子 PK">
      <div class="modal-head">
        <h2>🎲 骰子 PK</h2>
        <div class="pk-head-actions">
          <button
            class="icon-btn" :aria-label="soundMuted ? '开启音效' : '关闭音效'"
            :title="soundMuted ? '音效已关闭，点此开启' : '音效已开启，点此关闭'"
            @click="switchSound"
          >{{ soundMuted ? '🔇' : '🔊' }}</button>
          <button class="icon-btn" @click="emit('close')" aria-label="关闭">✕</button>
        </div>
      </div>
      <p class="setting-desc">
        每人点自己的「摇一摇」，<strong>都摇完自动定分工</strong>：点大的掌勺，点小的洗碗。点数会存下来，换人、刷新都不用重摇。
      </p>

      <!-- 只展示当前摇骰子的人；要换人请用右上角「我是」切换身份 -->
      <div class="pk-who">
        <span class="pk-who-label">正在摇</span>
        <span class="pk-who-current">
          <span class="avatar avatar-xs" :style="{ background: selfMember?.color }">{{ selfMember?.avatar }}</span>
          {{ selfName }}
        </span>
      </div>

      <div class="pk-list">
        <div
          v-for="r in ranked" :key="r.id" class="pk-row"
          :class="{
            'is-rolling': rollingId === r.id,
            'is-self': r.id === selfId,
            'is-chef': decided && r.id === chef.id,
            'is-washer': decided && r.id === washer.id,
          }"
        >
          <span class="avatar avatar-sm" :style="{ background: r.color }">{{ r.avatar }}</span>
          <span class="pk-name">{{ r.name }}<span v-if="r.id === selfId" class="pk-you">你</span></span>
          <div
            class="dice"
            :class="{ rolling: rollingId === r.id, empty: r.value == null }"
            :aria-label="r.value == null ? '还没摇' : `${r.value} 点`"
          >
            <span v-if="r.value == null" class="dice-q">?</span>
            <template v-else>
              <span v-for="i in 9" :key="i" class="pip" :class="{ on: PIPS[r.value].includes(i) }"></span>
            </template>
          </div>
          <button
            v-if="canRoll(r.id)"
            class="btn btn-sm pk-roll-btn"
            :class="rollingId === r.id ? 'btn-soft' : 'btn-primary'"
            :disabled="!!rollingId"
            @click="rollFor(r.id)"
          >
            {{ rollingId === r.id ? '摇动中…' : phase === 'tied' ? '再摇一次' : '🎲 摇一摇' }}
          </button>
          <span v-else-if="decided && r.id === chef.id" class="pk-tag chef-tag">👨‍🍳 掌勺</span>
          <span v-else-if="decided && r.id === washer.id" class="pk-tag wash-tag">🧽 洗碗</span>
          <span v-else-if="decided" class="pk-tag safe-tag">安全过关</span>
          <span v-else-if="r.value != null" class="pk-tag safe-tag">✓ 已摇 {{ r.value }} 点</span>
          <span v-else class="pk-hint-other">点右上角「我是」切到 {{ r.name }}</span>
        </div>
      </div>

      <Transition name="page" mode="out-in">
        <div
          v-if="decided" key="done" class="pk-result"
          :class="{ 'is-fx': showResultFx, 'is-win': selfId === chef.id, 'is-lose': selfId === washer.id }"
        >
          <div v-if="showResultFx" class="pk-clash" aria-hidden="true">
            <span class="pk-clash-a">🎲</span><span class="pk-clash-b">🎲</span>
          </div>
          <div class="pk-result-title">🎉 结果出来了</div>
          <div class="pk-result-line">
            <span class="pk-role chef-role">👨‍🍳 掌勺</span>
            <strong>{{ chef.avatar }} {{ chef.name }}</strong>
            <span class="pk-points">{{ chef.value }} 点</span>
          </div>
          <div class="pk-result-line">
            <span class="pk-role wash-role">🧽 洗碗</span>
            <strong>{{ washer.avatar }} {{ washer.name }}</strong>
            <span class="pk-points">{{ washer.value }} 点</span>
          </div>
          <div class="pk-auto-note">{{ alreadyDecided ? '↪ 分工已定好' : '↪ 正在自动定好分工…' }}</div>
          <button class="btn btn-soft btn-sm pk-restart-btn" @click="restart">🎲 再来一局</button>
        </div>
        <div v-else-if="phase === 'tied'" key="tied" class="pk-wait is-tied">
          🎲 点数打平，自动加摇…
        </div>
        <div v-else-if="needSelf.length" key="self" class="pk-wait is-self">
          轮到 <strong>{{ selfName }}</strong> 摇了，点你的「🎲 摇一摇」
        </div>
        <div v-else-if="needOthers.length" key="other" class="pk-wait is-other">
          还有 <strong>{{ needOthers.length }}</strong> 位没摇（{{ needOtherNames }}），点右上角「我是」切过去再摇 🔄
        </div>
        <div v-else key="wait" class="pk-wait">
          摇完自动出结果…
        </div>
      </Transition>
    </div>
  </div>
</template>
