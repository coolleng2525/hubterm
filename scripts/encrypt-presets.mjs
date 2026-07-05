#!/usr/bin/env node

import { createCipheriv, pbkdf2Sync, randomBytes } from 'node:crypto'
import { gzipSync } from 'node:zlib'
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..')
const presetsDir = path.join(root, 'presets')
const configsDir = path.join(root, 'configs')

const args = process.argv.slice(2)
const password = readArg('--password') || process.env.HUBTERM_PRESETS_PASSWORD
const output = readArg('--out') || path.join(configsDir, `hubterm-presets-${new Date().toISOString().slice(0, 10)}-enc.tar.gz`)

if (!password) {
  console.error('Usage: HUBTERM_PRESETS_PASSWORD=secret scripts/encrypt-presets.mjs')
  console.error('   or: scripts/encrypt-presets.mjs --password secret [--out configs/name-enc.tar.gz]')
  process.exit(2)
}

mkdirSync(configsDir, { recursive: true })

const bundle = {
  version: '1.0',
  exported_at: new Date().toISOString(),
  description: 'HubTerm encrypted local presets',
  scripts: [],
}
const files = new Map()
const referenced = new Set()

for (const entry of readdirSync(presetsDir, { withFileTypes: true })) {
  if (!entry.isFile() || !entry.name.toLowerCase().endsWith('.json')) continue
  const jsonPath = path.join(presetsDir, entry.name)
  const preset = JSON.parse(readFileSync(jsonPath, 'utf8'))
  for (const script of preset.scripts || []) {
    const item = { ...script }
    if (item.source_file) {
      const cleanName = cleanBundlePath(item.source_file)
      referenced.add(cleanName)
      files.set(cleanName, readFileSync(path.join(presetsDir, cleanName)))
      item.source_file = cleanName
    }
    bundle.scripts.push(item)
  }
}

for (const entry of readdirSync(presetsDir, { withFileTypes: true })) {
  if (!entry.isFile() || !isScriptFile(entry.name)) continue
  const cleanName = cleanBundlePath(entry.name)
  files.set(cleanName, readFileSync(path.join(presetsDir, cleanName)))
  if (!referenced.has(cleanName)) {
    bundle.scripts.push({
      name: path.basename(cleanName, path.extname(cleanName)),
      description: `Loaded from presets/${cleanName}`,
      language: inferLanguage(cleanName),
      source_file: cleanName,
      timeout: 30,
    })
  }
}

const createdAt = new Date().toISOString()
const innerFiles = [
  ['hubterm-package.json', Buffer.from(JSON.stringify({
    package_version: '1.0',
    bundle_version: bundle.version,
    created_at: createdAt,
    format: 'hubterm-presets',
    encrypted: false,
  }, null, 2))],
  ['manifest.json', Buffer.from(JSON.stringify(bundle, null, 2))],
  ...Array.from(files.entries()),
]

const innerTar = createTar(innerFiles)
const salt = randomBytes(16)
const iv = randomBytes(12)
const key = pbkdf2Sync(Buffer.from(password), salt, 120000, 32, 'sha256')
const cipher = createCipheriv('aes-256-gcm', key, iv)
const encrypted = Buffer.concat([cipher.update(innerTar), cipher.final(), cipher.getAuthTag()])

const outerInfo = {
  package_version: '1.0',
  bundle_version: bundle.version,
  created_at: createdAt,
  format: 'hubterm-presets',
  encrypted: true,
  cipher: 'AES-256-GCM',
  kdf: 'PBKDF2-SHA256',
  iterations: 120000,
  salt: salt.toString('base64'),
  iv: iv.toString('base64'),
  payload_file: 'payload.enc',
  payload_format: 'tar',
}
const outerTar = createTar([
  ['hubterm-package.json', Buffer.from(JSON.stringify(outerInfo, null, 2))],
  ['payload.enc', encrypted],
])

writeFileSync(output, gzipSync(outerTar))
console.log(`Wrote ${path.relative(root, output)}`)
console.log(`Scripts: ${bundle.scripts.length}`)

function readArg(name) {
  const idx = args.indexOf(name)
  return idx >= 0 ? args[idx + 1] : ''
}

function isScriptFile(name) {
  return ['.sh', '.bash', '.py', '.txt'].includes(path.extname(name).toLowerCase())
}

function inferLanguage(name) {
  switch (path.extname(name).toLowerCase()) {
    case '.py':
      return 'python'
    case '.sh':
    case '.bash':
      return 'shell'
    default:
      return 'text'
  }
}

function cleanBundlePath(name) {
  const normalized = name.replaceAll('\\', '/')
  const clean = path.posix.normalize(normalized)
  if (!clean || clean === '.' || clean.startsWith('../') || clean.startsWith('/')) {
    throw new Error(`Invalid preset path: ${name}`)
  }
  return clean
}

function createTar(entries) {
  const chunks = []
  for (const [name, data] of entries) {
    chunks.push(tarHeader(name, data.length))
    chunks.push(Buffer.from(data))
    const padding = (512 - (data.length % 512)) % 512
    if (padding) chunks.push(Buffer.alloc(padding))
  }
  chunks.push(Buffer.alloc(1024))
  return Buffer.concat(chunks)
}

function tarHeader(name, size) {
  const header = Buffer.alloc(512)
  writeString(header, 0, 100, name)
  writeString(header, 100, 8, '0000600')
  writeString(header, 108, 8, '0000000')
  writeString(header, 116, 8, '0000000')
  writeOctal(header, 124, 12, size)
  writeOctal(header, 136, 12, Math.floor(Date.now() / 1000))
  header.fill(0x20, 148, 156)
  header[156] = '0'.charCodeAt(0)
  writeString(header, 257, 6, 'ustar')
  writeString(header, 263, 2, '00')
  let sum = 0
  for (const byte of header) sum += byte
  writeOctal(header, 148, 8, sum)
  return header
}

function writeString(buffer, offset, length, value) {
  Buffer.from(value).copy(buffer, offset, 0, Math.min(Buffer.byteLength(value), length))
}

function writeOctal(buffer, offset, length, value) {
  const text = value.toString(8).padStart(length - 1, '0')
  writeString(buffer, offset, length - 1, text)
  buffer[offset + length - 1] = 0
}
