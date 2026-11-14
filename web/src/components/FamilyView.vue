<script setup>
import { inject, computed, reactive, ref } from 'vue'
import { avatarChoices, avatarGroups, colorChoices, memberOf, friendlyDate, avoidChoices } from '../utils'
import { useModalBehavior } from '../modal'
import { api } from '../api'
import { shake } from '../feedback'

const { state, actions, confirmDialog, pending } = inject('app')

const chefCount = computed(() => {
  const map = {}
  for (const p of state.plans) if (p.chefId) map[p.chefId] = (map[p.chefId] || 0) + 1
  return map
})
const washCount = computed(() => {
  const map = {}
  for (const p of state.plans) if (p.dishwasherId) map[p.dishwasherId] = (map[p.dishwasherId] || 0) + 1
  return map
})
const orderCount = computed(() => {
  const map = {}
  for (const p of state.plans) for (const o of (p.orders || [])) map[o.userId] = (map[o.userId] || 0) + 1
  return map
})
const maxChef = computed(() => Math.max(1, ...Object.values(chefCount.value)))

const editorOpen = ref(false)
const editorRoot = ref(null)
useModalBehavior(() => { editorOpen.value = false }, editorRoot, editorOpen)
const editingId = ref(null)
const form = reactive({ name: '', avatar: '🙂', color: colorChoices[0], avoid: [] })
const customAvoid = ref('')

function toggleAvoid(a) {
  const i = form.avoid.indexOf(a)
  if (i >= 0) form.avoid.splice(i, 1)
  else if (form.avoid.length < 16) form.avoid.push(a)
}
function addCustomAvoid() {
  const v = customAvoid.value.trim().slice(0, 16)
  if (v && !form.avoid.includes(v) && form.avoid.length < 16) form.avoid.push(v)
  customAvoid.value = ''
}

function openAdd() {
  editingId.value = null
  form.name = ''
  form.avatar = avatarChoices[Math.floor(Math.random() * avatarChoices.length)]
  form.color = colorChoices[state.members.length % colorChoices.length]
  form.avoid = []
  customAvoid.value = ''
  editorOpen.value = true
}
function openEdit(member) {
  editingId.value = member.id
  form.name = member.name
  form.avatar = member.avatar
  form.color = member.color
  form.avoid = [...(member.avoid || [])]
  customAvoid.value = ''
  editorOpen.value = true
}
async function save() {
  if (!form.name.trim()) {
    actions.toast('总得有个称呼吧', 'warn')
    shake('#member-name')
    return
  }
  const payload = { name: form.name.trim(), avatar: form.avatar, color: form.color, avoid: [...form.avoid] }
  const ok = editingId.value
    ? await actions.updateMember(editingId.value, payload)
    : await actions.addMember(payload)
  if (ok) editorOpen.value = false
}
async function remove(id) {
  const m = memberOf(id, state.members)
  const ok = await confirmDialog({
    title: '移除成员',
    message: `确定移除「${m?.name}」吗？历史点菜记录仍会保留名字。`,
    confirmText: '移除', cancelText: '再想想', danger: true,
  })
  if (!ok) return
  await actions.deleteMember(id)
}

// ---------- 成员密码 ----------
// 明文只在输入框里出现一次，提交后由服务端做盐+哈希；前端只拿得到 hasPin 布尔。
const pinEditor = ref(null) // { id, name, avatar, hasPin }
const pinRoot = ref(null)
const pinForm = reactive({ oldPin: '', newPin: '', confirmPin: '' })
const pinBusy = ref(false)
const pinError = ref('')
useModalBehavior(() => { pinEditor.value = null }, pinRoot, computed(() => !!pinEditor.value), true)

function openPin(m) {
  pinEditor.value = { id: m.id, name: m.name, avatar: m.avatar, hasPin: !!m.hasPin }
  pinForm.oldPin = ''
  pinForm.newPin = ''
  pinForm.confirmPin = ''
  pinError.value = ''
}
async function savePin() {
  const e = pinEditor.value
  if (!e || pinBusy.value) return
  pinError.value = ''
  const np = pinForm.newPin
  if (np && (np.length < 4 || np.length > 16)) { pinError.value = '新密码需要 4–16 个字符'; return }
  if (np && np !== pinForm.confirmPin) { pinError.value = '两次输入的新密码不一致'; return }
  if (!np && !e.hasPin) { pinError.value = '先填一个新密码'; return }
  pinBusy.value = true
  try {
    await api.setMemberPin(e.id, { pin: np, oldPin: e.hasPin ? pinForm.oldPin : '' })
    await actions.refresh() // 刷新 members 才能拿到最新的 hasPin
    pinEditor.value = null
    actions.toast(np ? '密码设置好了' : '密码已取消', 'success')
  } catch (err) {
    pinError.value = err.message || '保存失败'
  } finally {
    pinBusy.value = false
  }
}
</script>

<template>
  <section>
    <div class="section-head">
      <div>
        <h2 class="section-title">👨‍👩‍👧 家里的人</h2>
        <div class="section-sub">每人一个身份，点菜、掌勺都算得清；不用注册、不填手机号</div>
      </div>
      <button class="btn btn-primary" @click="openAdd">＋ 添加成员</button>
    </div>

    <div class="member-grid">
      <div v-for="m in state.members" :key="m.id" class="card member-card">
        <div class="member-head">
          <span class="avatar avatar-lg" :style="{ background: m.color }">{{ m.avatar }}</span>
          <div>
            <div class="member-name">{{ m.name }}</div>
            <div class="tiny muted">{{ chefCount[m.id] || 0 }} 次掌勺 · {{ washCount[m.id] || 0 }} 次洗碗 · {{ orderCount[m.id] || 0 }} 次点菜</div>
          </div>
        </div>
        <div class="member-tags">
          <div v-if="(m.avoid || []).length" class="member-avoid tiny">⚠️ 不吃 {{ m.avoid.join('、') }}</div>
          <div v-if="m.hasPin" class="member-avoid tiny">🔒 切换身份需要密码</div>
        </div>
        <div class="member-stat"><span>掌勺 / 洗碗</span><span>{{ chefCount[m.id] || 0 }} / {{ washCount[m.id] || 0 }} 天</span></div>
        <div class="stat-bar">
          <div class="stat-bar-fill" :style="{ width: `${((chefCount[m.id] || 0) / maxChef) * 100}%`, background: m.color }"></div>
        </div>
        <div class="member-actions">
          <button v-if="m.id === state.currentUserId" class="btn btn-sm btn-soft" @click="openPin(m)">{{ m.hasPin ? '🔑 改密码' : '🔑 设密码' }}</button>
          <button class="btn btn-sm btn-ghost" @click="openEdit(m)">编辑</button>
          <button class="btn btn-sm btn-danger" @click="remove(m.id)">移除</button>
        </div>
      </div>
    </div>

    <div class="section-head">
      <div>
        <h2 class="section-title">🗓️ 最近谁做</h2>
        <div class="section-sub">按记录里的掌勺与洗碗统计，只用于家庭内部玩笑，别当真</div>
      </div>
    </div>
    <div class="panel">
      <div v-if="state.plans.length" class="history-list">
        <div v-for="p in state.plans.slice(0, 6)" :key="p.date" class="spread fs-88">
          <span class="muted">{{ friendlyDate(p.date) }}</span>
          <span class="row wrap">
            <span v-if="memberOf(p.chefId, state.members)">
              👨‍🍳 {{ memberOf(p.chefId, state.members).name }}
            </span>
            <span v-if="memberOf(p.dishwasherId, state.members)">
              🧽 {{ memberOf(p.dishwasherId, state.members).name }}
            </span>
            <span v-if="!p.chefId && !p.dishwasherId" class="muted">还没定</span>
          </span>
        </div>
      </div>
      <div v-else class="empty pad-18"><span class="empty-icon">📝</span>还没有记录</div>
    </div>

    <Transition name="modal">
      <div v-if="editorOpen" ref="editorRoot" class="modal-backdrop" @click.self="editorOpen = false">
      <div class="modal" role="dialog" aria-modal="true" :aria-label="editingId ? '编辑成员' : '添加一位家人'">
        <div class="modal-head">
          <h2>{{ editingId ? '编辑成员' : '添加一位家人' }}</h2>
          <button class="icon-btn" @click="editorOpen = false">✕</button>
        </div>
        <div class="field">
          <label for="member-name">怎么称呼</label>
          <input id="member-name" v-model="form.name" class="input" maxlength="20" placeholder="例如：爸爸、奶奶、小宝" />
        </div>
        <div class="field mt-14">
          <label>挑个头像</label>
          <div class="avatar-picker avatar-picker-grouped">
            <div v-for="g in avatarGroups" :key="g.label" class="avatar-group">
              <div class="avatar-group-label">{{ g.label }}</div>
              <div class="avatar-group-items">
                <button
                  v-for="a in g.items" :key="a" :class="{ active: form.avatar === a }"
                  :aria-label="`选头像 ${a}`" @click="form.avatar = a"
                >{{ a }}</button>
              </div>
            </div>
          </div>
        </div>
        <div class="field mt-14">
          <label>挑个颜色</label>
          <div class="color-picker">
            <button
              v-for="c in colorChoices" :key="c" class="swatch" :class="{ active: form.color === c }"
              :style="{ background: c }" :aria-label="`颜色 ${c}`" @click="form.color = c"
            ></button>
          </div>
        </div>
        <div class="field mt-14">
          <label>口味避雷（点菜时会提示）</label>
          <div class="avoid-picker">
            <button
              v-for="a in avoidChoices" :key="a" type="button" class="chip"
              :class="{ active: form.avoid.includes(a) }" @click="toggleAvoid(a)"
            >{{ a }}</button>
          </div>
          <div v-if="form.avoid.length" class="avoid-selected">
            <span v-for="a in form.avoid" :key="a" class="tag tag-avoid" @click="toggleAvoid(a)">{{ a }} ✕</span>
          </div>
          <div class="avoid-custom">
            <input
              v-model="customAvoid" class="input" maxlength="16" placeholder="自定义，如「折耳根」"
              @keydown.enter.prevent="addCustomAvoid"
            />
            <button class="btn btn-soft btn-sm" @click="addCustomAvoid">添加</button>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" :disabled="pending('member')" @click="editorOpen = false">取消</button>
          <button class="btn btn-primary" :disabled="pending('member')" @click="save">
            <span v-if="pending('member')" class="btn-spinner btn-spinner-light" aria-hidden="true" />
            {{ pending('member') ? '保存中…' : '保存成员' }}
          </button>
        </div>
      </div>
      </div>
    </Transition>

    <Transition name="modal">
      <div v-if="pinEditor" class="modal-backdrop pin-backdrop" @click.self="pinEditor = null">
        <div ref="pinRoot" class="modal pin-modal" role="dialog" aria-modal="true" aria-label="设置成员密码">
          <div class="modal-head">
            <h2>🔑 {{ pinEditor.avatar }} {{ pinEditor.name }} 的密码</h2>
            <button class="icon-btn" aria-label="取消" @click="pinEditor = null">✕</button>
          </div>
          <p class="setting-desc">
            设好后，别人切到这个身份点菜会先要密码。密码只存哈希，不进备份，也不会出现在页面数据里。
          </p>
          <div v-if="pinEditor.hasPin" class="field">
            <label for="pin-old">当前密码</label>
            <input id="pin-old" v-model="pinForm.oldPin" class="input" type="password" maxlength="16" autocomplete="current-password" />
          </div>
          <div class="field">
            <label for="pin-new">新密码（4–16 位；留空表示取消密码）</label>
            <input id="pin-new" v-model="pinForm.newPin" class="input" type="password" maxlength="16" autocomplete="new-password" />
          </div>
          <div v-if="pinForm.newPin" class="field">
            <label for="pin-confirm">再输一次新密码</label>
            <input id="pin-confirm" v-model="pinForm.confirmPin" class="input" type="password" maxlength="16" autocomplete="new-password" @keyup.enter="savePin" />
          </div>
          <p v-if="pinError" class="pin-error" role="alert">{{ pinError }}</p>
          <div class="modal-foot">
            <button class="btn btn-soft" @click="pinEditor = null">取消</button>
            <button class="btn btn-primary" :disabled="pinBusy" @click="savePin">
              {{ pinBusy ? '保存中…' : '保存密码' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </section>
</template>
