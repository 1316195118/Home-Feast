// 统一构建脚本：前端 -> Go embed 目录 -> 单文件可执行程序。
// 只用 Node 内置模块；直接调 Vite 的 JS 入口，绕开不同系统上 npm.cmd 的差异。
import { execFileSync, execSync } from 'node:child_process'
import { brotliCompressSync, constants as zlibConstants, gzipSync } from 'node:zlib'
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('..', import.meta.url)))
const webDir = join(root, 'web')
const viteOut = join(webDir, 'dist')
const embedDir = join(root, 'webapp', 'dist')
const viteBin = join(webDir, 'node_modules', 'vite', 'bin', 'vite.js')
const binary = process.platform === 'win32' ? 'menu.exe' : 'menu'
const frontendOnly = process.argv.includes('--frontend-only')

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

const precompressExts = new Set(['.html', '.js', '.css', '.json', '.svg'])

// 构建时用最高级别预压缩一次，浏览器请求时直接下发 .br / .gz。
// 小于 1 KiB 的文件不生成，避免压缩头比正文还大。
function precompressDir(dir) {
  let generated = 0
  let originalBytes = 0
  let brotliBytes = 0
  let gzipBytes = 0
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const target = join(dir, entry.name)
    if (entry.isDirectory()) {
      const nested = precompressDir(target)
      generated += nested.generated
      originalBytes += nested.originalBytes
      brotliBytes += nested.brotliBytes
      gzipBytes += nested.gzipBytes
      continue
    }
    if (!entry.isFile() || !precompressExts.has(extname(entry.name).toLowerCase())) continue
    const data = readFileSync(target)
    if (data.length < 1024) continue
    const compressed = brotliCompressSync(data, {
      params: {
        [zlibConstants.BROTLI_PARAM_QUALITY]: zlibConstants.BROTLI_MAX_QUALITY,
        [zlibConstants.BROTLI_PARAM_SIZE_HINT]: data.length,
      },
    })
    const gzipped = gzipSync(data, { level: 9 })
    writeFileSync(target + '.br', compressed)
    writeFileSync(target + '.gz', gzipped)
    generated += 1
    originalBytes += data.length
    brotliBytes += compressed.length
    gzipBytes += gzipped.length
  }
  return { generated, originalBytes, brotliBytes, gzipBytes }
}

if (!existsSync(join(webDir, 'node_modules'))) {
  title('安装前端依赖 npm install')
  execSync('npm install --no-audit --no-fund', { cwd: webDir, stdio: 'inherit' })
}

title('构建前端 vite build')
execFileSync(process.execPath, [viteBin, 'build'], { cwd: webDir, stdio: 'inherit' })

title('预压缩可缓存文本资源 Brotli')
const br = precompressDir(viteOut)
const brotliSaving = br.originalBytes ? ((1 - br.brotliBytes / br.originalBytes) * 100).toFixed(1) : '0.0'
const gzipSaving = br.originalBytes ? ((1 - br.gzipBytes / br.originalBytes) * 100).toFixed(1) : '0.0'
console.log(
  `${br.generated} 个文件：${(br.originalBytes / 1024).toFixed(1)} KB -> `
  + `Brotli ${(br.brotliBytes / 1024).toFixed(1)} KB（节省 ${brotliSaving}%） / `
  + `gzip ${(br.gzipBytes / 1024).toFixed(1)} KB（节省 ${gzipSaving}%）`,
)

title('同步前端产物到 Go embed 目录')
rmSync(embedDir, { recursive: true, force: true })
copyDir(viteOut, embedDir)
writeFileSync(
  join(embedDir, 'build.json'),
  JSON.stringify({ app: 'family-menu', builtAt: new Date().toISOString(), by: 'scripts/build.mjs' }, null, 2) + '\n',
)
console.log(`已复制 ${readdirSync(embedDir).length} 项到 webapp/dist`)

if (frontendOnly) {
  console.log('\n前端产物已同步，跳过 Go 编译')
  process.exit(0)
}

title('编译 Go 单文件服务端')
execFileSync('go', ['build', '-trimpath', '-ldflags', '-s -w', '-o', join(root, binary), './cmd/menu'], {
  cwd: root, stdio: 'inherit',
})
console.log(`\n构建完成：${join(root, binary)}`)
