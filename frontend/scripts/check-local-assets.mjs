import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('../', import.meta.url))
const vendor = path.join(root, 'public/vendor')
const manifest = JSON.parse(await readFile(path.join(vendor, 'manifest.json'), 'utf8'))
for (const asset of manifest) {
  const filename = path.resolve(vendor, asset.file)
  if (!filename.startsWith(vendor + path.sep)) throw new Error('Invalid vendor asset path')
  const data = await readFile(filename)
  const digest = createHash('sha384').update(data).digest('base64')
  if (digest !== asset.sha384) throw new Error(`Vendor checksum mismatch: ${asset.file}`)
  if (filename.endsWith('.woff2') && data.subarray(0, 4).toString() !== 'wOF2') throw new Error(`Invalid WOFF2 font: ${asset.file}`)
  await readFile(path.join(path.dirname(filename), 'LICENSE.txt'))
}

// These sources must never silently reintroduce mutable third-party assets.
for (const file of ['index.html', 'src/index.css', 'public/fonts.css', '../backend/cmd/server/ortu_portal_page.go', '../backend/cmd/server/ujian_online_page.go']) {
  const source = await readFile(path.resolve(root, file), 'utf8')
  if (/fonts\.googleapis\.com|fonts\.gstatic\.com|cdn\.jsdelivr\.net/.test(source)) throw new Error(`External font or chart asset in ${file}`)
}
const portal = await readFile(path.resolve(root, '../backend/cmd/server/ortu_portal_page.go'), 'utf8')
const chart = manifest.find(asset => asset.package === 'chart.js')
if (!chart || !portal.includes(`src="/vendor/${chart.file}" integrity="sha384-${chart.sha384}"`)) throw new Error('Parent portal chart URL or integrity differs from the vendor manifest')
console.log(`Verified ${manifest.length} local vendor assets, licenses, and public page references.`)

const sonner = await readFile(path.join(root, 'node_modules/sonner/dist/index.mjs'), 'utf8')
const injectedCSS = sonner.match(/^__insertCSS\(("(?:\\.|[^"\\])*")\);/m)
if (!injectedCSS) throw new Error('Sonner stylesheet changed; review the production CSP before upgrading')
const styleHash = createHash('sha256').update(JSON.parse(injectedCSS[1])).digest('base64')
const server = await readFile(path.resolve(root, '../backend/cmd/server/main.go'), 'utf8')
if (!server.includes(`'sha256-${styleHash}'`)) throw new Error('Production CSP does not match the bundled notification stylesheet')
