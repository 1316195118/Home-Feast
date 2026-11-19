// 直接跑 isLanHost 判定逻辑（utils 是 ESM，其中 IS LAN 部分不依赖 DOM）
import { isLanHost, imageSize } from './src/utils.js'
const cases = [
  ['10.10.10.251', true], ['192.168.1.1', true], ['172.16.0.1', true],
  ['172.31.255.255', true], ['172.32.0.1', false], ['8.8.8.8', false],
  ['family.example.com', false], ['localhost', true], ['127.0.0.1', true],
]
let pass = 0
for (const [host, want] of cases) {
  const got = isLanHost(host)
  if (got === want) pass++; else console.log('FAIL', host, got, 'want', want)
}
console.log(`isLanHost: ${pass}/${cases.length}`)
console.log('imageSize(full):', JSON.stringify(imageSize('full')))
console.log('imageSize(thumb):', JSON.stringify(imageSize('thumb')))
