#!/usr/bin/env bun
// Fetches the Xray config validator (Xray-core compiled to WebAssembly) and its
// JSON schemas, and precompresses the built dashboard so the 60 MB runtime is sent
// as ~12 MB of gzip. Everything runs in the administrator's browser; the server
// only serves the files.
//
//   bun scripts/editor-assets.cjs download [dir]   (default: public/assets)
//   bun scripts/editor-assets.cjs compress [dir]   (default: dist)
//
// Downloads are pinned by SHA-256 because wasm_exec.js runs in the dashboard
// origin. To update, download the new files, review them, and replace the hashes.

const crypto = require('node:crypto')
const fs = require('node:fs')
const path = require('node:path')
const zlib = require('node:zlib')

const BASE_URL = process.env.EDITOR_ASSETS_URL || 'https://remnawave.github.io/xray-monaco-editor'

const FILES = {
    'main.wasm': '45b31aca38f8de5febdb4ef032f691e033d0b7139ba04bf792ef3a57b7c595cc',
    'wasm_exec.js': '0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14',
    'xray.schema.json': '030b345365213d523d7bd7d2dd17c3916f8d2cb95f44fd555934758d632fefbe',
    'xray.schema.cn.json': 'c778deb9e7dd841faab10354935d772fcd656bd1f92d6797bf06015f0114e503'
}

// Text assets keep their originals for clients without gzip. The WebAssembly
// runtime is only stored compressed; the server decompresses it when needed.
const COMPRESSIBLE = new Set(['.css', '.js', '.json', '.svg', '.wasm'])
const COMPRESSED_ONLY = new Set(['.wasm'])
const MIN_SIZE = 1024

const sha256 = (data) => crypto.createHash('sha256').update(data).digest('hex')

async function download(dir) {
    fs.mkdirSync(dir, { recursive: true })

    for (const [name, expected] of Object.entries(FILES)) {
        const target = path.join(dir, name)

        if (fs.existsSync(target) && sha256(fs.readFileSync(target)) === expected) {
            console.log(`${name}: up to date`)
            continue
        }

        const response = await fetch(`${BASE_URL}/${name}`)
        if (!response.ok) {
            throw new Error(`${name}: HTTP ${response.status}`)
        }

        const data = Buffer.from(await response.arrayBuffer())
        const actual = sha256(data)
        if (actual !== expected) {
            throw new Error(`${name}: SHA-256 ${actual} does not match pinned ${expected}`)
        }

        fs.writeFileSync(`${target}.tmp`, data)
        fs.renameSync(`${target}.tmp`, target)
        console.log(`${name}: ${(data.length / 1048576).toFixed(1)} MB`)
    }
}

function walk(dir) {
    return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
        const full = path.join(dir, entry.name)
        return entry.isDirectory() ? walk(full) : [full]
    })
}

function compress(dir) {
    let saved = 0

    for (const file of walk(dir)) {
        const ext = path.extname(file)
        if (!COMPRESSIBLE.has(ext)) continue

        const data = fs.readFileSync(file)
        if (data.length < MIN_SIZE) continue

        const gz = zlib.gzipSync(data, { level: zlib.constants.Z_BEST_COMPRESSION })
        if (gz.length >= data.length) continue

        const stat = fs.statSync(file)
        fs.writeFileSync(`${file}.gz`, gz)
        fs.utimesSync(`${file}.gz`, stat.atime, stat.mtime)
        saved += data.length - gz.length

        if (COMPRESSED_ONLY.has(ext)) {
            fs.rmSync(file)
            console.log(`${path.relative(dir, file)}: ${data.length} -> ${gz.length} bytes`)
        }
    }

    console.log(`precompressed ${dir}, ${(saved / 1048576).toFixed(1)} MB smaller`)
}

const [command, dir] = process.argv.slice(2)

if (command === 'download') {
    download(dir || 'public/assets').catch((error) => {
        console.error(error.message)
        process.exit(1)
    })
} else if (command === 'compress') {
    compress(dir || 'dist')
} else {
    console.error('usage: editor-assets.cjs download|compress [dir]')
    process.exit(2)
}
