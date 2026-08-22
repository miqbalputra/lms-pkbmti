import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { Resvg } from '@resvg/resvg-js'

const scriptDirectory = dirname(fileURLToPath(import.meta.url))
const publicDirectory = resolve(scriptDirectory, '..', 'public')
const svg = await readFile(resolve(publicDirectory, 'pkbmti-lms-icon.svg'))

const assets = [
  ['pkbmti-lms-book-192.png', 192],
  ['pkbmti-lms-book-512.png', 512],
  ['pkbmti-lms-book-maskable.png', 512],
]

await Promise.all(
  assets.map(async ([fileName, width]) => {
    const image = new Resvg(svg, {
      fitTo: { mode: 'width', value: width },
      shapeRendering: 2,
    }).render()
    await writeFile(resolve(publicDirectory, fileName), image.asPng())
  })
)

console.log('PWA icons generated.')
