<script setup>
import { inject, computed, reactive, ref } from 'vue'
import { avatarChoices, avatarGroups, colorChoices, memberOf, friendlyDate } from '../utils'
import { useModalBehavior } from '../modal'

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
const form = reactive({ name: '', avatar: '🙂', color: colorChoices[0] })

function openAdd() {
  editingId.value = null
  form.name = ''
  form.avatar = avatarChoices[Math.floor(Math.random() * avatarChoices.length)]
  form.color = colorChoices[state.members.length % colorChoices.length]
  editorOpen.value = true
}
function openEdit(member) {
  editingId.value = member.id
  form.name = member.name
  form.avatar = member.avatar
  form.color = member.color
  editorOpen.value = true
}
async function save() {
  if (!form.name.trim()) {
    actions.toast('总得有个称呼吧', 'warn')
    return
  }
  const payload = { name: form.name.trim(), avatar: form.avatar, color: form.color }
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
        <div class="member-stat"><span>掌勺 / 洗碗</span><span>{{ chefCount[m.id] || 0 }} / {{ washCount[m.id] || 0 }} 天</span></div>
        <div class="stat-bar">
          <div class="stat-bar-fill" :style="{ width: `${((chefCount[m.id] || 0) / maxChef) * 100}%`, background: m.color }"></div>
        </div>
        <div class="member-actions">
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
        <div v-for="p in state.plans.slice(0, 6)" :key="p.date" class="spread" style="font-size: 0.88rem">
          <span class="muted">{{ friendlyDate(p.date) }}</span>
          <span class="row" style="gap: 10px; flex-wrap: wrap">
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
      <div v-else class="empty" style="padding: 18px"><span class="empty-icon">📝</span>还没有记录</div>
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
          <input id="member-name" v-model="form.name" class="input" maxlength="20" placeholder="爸爸 / 奶奶 / 小宝" />
        </div>
        <div class="field" style="margin-top: 14px">
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
        <div class="field" style="margin-top: 14px">
          <label>挑个颜色</label>
          <div class="color-picker">
            <button
              v-for="c in colorChoices" :key="c" class="swatch" :class="{ active: form.color === c }"
              :style="{ background: c }" :aria-label="`颜色 ${c}`" @click="form.color = c"
            ></button>
          </div>
        </div>
        <div class="modal-foot">
          <button class="btn" :disabled="pending('member')" @click="editorOpen = false">取消</button>
          <button class="btn btn-primary" :disabled="pending('member')" @click="save">
            <span v-if="pending('member')" class="btn-spinner btn-spinner-light" aria-hidden="true" />
            {{ pending('member') ? '保存中…' : '保存' }}
          </button>
        </div>
      </div>
      </div>
    </Transition>
  </section>
</template>
