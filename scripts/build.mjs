// 统一构建脚本：前端 -> Go embed 目录 -> 单文件可执行程序。
// 只用 Node 内置模块；直接调 Vite 的 JS 入口，绕开不同系统上 npm.cmd 的差异。
import { execFileSync, execSync } from 'node:child_process'
import { copyFileSync, mkdirSync, readdirSync, rmSync, writeFileSync, existsSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('..', import.meta.url)))
const webDir = join(root, 'web')
const viteOut = join(webDir, 'dist')
const embedDir = join(root, 'webapp', 'dist')
const viteBin = join(webDir, 'node_modules', 'vite', 'bin', 'vite.js')
const binary = process.platform === 'win32' ? 'menu.exe' : 'menu'

function title(text) {
  console.log(`\n> ${text}`)
}

// 自己递归复制，避免依赖 fs.cpSync（部分 Windows 环境下会直接终止进程）。
function copyDir(from, to) {
  mkdirSync(to, { recursive: true })
  for (const entry of readdirSync(from, { withFileTypes: true })) {
    const src = join(from, entry.name)
    const dest = join(to, entry.name)
    if (entry.isDirectory()) copyDir(src, dest)
    else if (entry.isFile()) copyFileSync(src, dest)
  }
}

if (!existsSync(join(webDir, 'node_modules'))) {
  title('安装前端依赖 npm install')
  execSync('npm install --no-audit --no-fund', { cwd: webDir, stdio: 'inherit' })
}

title('构建前端 vite build')
execFileSync(process.execPath, [viteBin, 'build'], { cwd: webDir, stdio: 'inherit' })

title('同步前端产物到 Go embed 目录')
rmSync(embedDir, { recursive: true, force: true })
copyDir(viteOut, embedDir)
writeFileSync(
  join(embedDir, 'build.json'),
  JSON.stringify({ app: 'family-menu', builtAt: new Date().toISOString(), by: 'scripts/build.mjs' }, null, 2) + '\n',
)
console.log(`已复制 ${readdirSync(embedDir).length} 项到 webapp/dist`)

title('编译 Go 单文件服务端')
execFileSync('go', ['build', '-trimpath', '-ldflags', '-s -w', '-o', join(root, binary), './cmd/menu'], {
  cwd: root, stdio: 'inherit',
})
console.log(`\n构建完成：${join(root, binary)}`)
