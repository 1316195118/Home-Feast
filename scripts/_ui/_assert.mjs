// UI 验收脚本的共享断言工具。
//
// 既有脚本大量用 console.log('标签:', 布尔表达式) 打印结果，但即使全部失败
// 也以 0 退出，回归无法被 CI / 发版门禁发现。新增 check() 显式记录断言，
// 任一失败则在进程结束时设置 exitCode=1。
//
// 用法：
//   import { check, finishAssertions } from './_assert.mjs'
//   check('密码修改需旧密码', wrongOld.status === 400)
//   ...
//   finishAssertions()   // 脚本末尾，可选（exit 钩子也会兜底）

const failures = []
let reported = false

export function check(label, ok, detail = '') {
  const passed = Boolean(ok)
  if (!passed) failures.push(`${label}${detail ? '：' + detail : ''}`)
  console.log(`${passed ? '  ✓' : '  ✗'} ${label}${detail ? '：' + detail : ''}`)
  return passed
}

function report() {
  if (reported || !failures.length) return
  reported = true
  console.log(`\n[断言失败] ${failures.length} 项检查未通过：`)
  failures.forEach((f, i) => console.log(`  ${i + 1}. ${f}`))
  process.exitCode = 1
}

export function finishAssertions() {
  report()
}

process.on('exit', report)
