#!/usr/bin/env bun
// Fetches the Xray config validator (Xray-core compiled to WebAssembly) and
// editor schemas, and precompresses the built dashboard so the 60 MB runtime is
// sent as ~12 MB of gzip. The server only serves these files.
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
    'main.wasm': {
        sha256: 'ef9f3d3bd566daeae2687d26302b21c1db8d56261839e91dc8b7744339a93e70'
    },
    'wasm_exec.js': {
        sha256: '0c949f4996f9a89698e4b5c586de32249c3b69b7baadb64d220073cc04acba14'
    },
    'xray.schema.json': {
        sha256: '561299d0461425132b46c4305c5847ffe827b74a0853e14751f070ef28eb55de'
    },
    'xray.schema.cn.json': {
        sha256: '75a6da5a2a27533de204e3cb18d1ea6f5cb57141ca99fee524bce2abb5780207'
    },
    'singbox.schema.json': {
        url: 'https://github.com/BlackDuty/sing-box-schema/releases/download/v1.13.13/schema.json',
        sha256: 'a8e691ed3565f6ae02af19a992c0cb8d3c0e98e79e8e921819740d66f75ed9a8'
    },
    'mihomo.schema.json': {
        url: 'https://github.com/dongchengjie/meta-json-schema/releases/download/v1.19.29/meta-json-schema.json',
        sha256: '04368aef934be14b5392e71ec4c33cd56999545918f462de1c1eee750815cc75'
    }
}

// Text assets keep their originals for clients without gzip. The WebAssembly
// runtime is only stored compressed; the server decompresses it when needed.
const COMPRESSIBLE = new Set(['.css', '.js', '.json', '.svg', '.wasm'])
const COMPRESSED_ONLY = new Set(['.wasm'])
const MIN_SIZE = 1024

const sha256 = (data) => crypto.createHash('sha256').update(data).digest('hex')

// The asset host occasionally answers 5xx; retry those and network errors so a
// transient outage does not fail release and Docker builds.
const ATTEMPTS = 5

async function fetchAsset(name, url = `${BASE_URL}/${name}`) {
    for (let attempt = 1; ; attempt++) {
        let failure
        try {
            const response = await fetch(url)
            if (response.ok) {
                return Buffer.from(await response.arrayBuffer())
            }
            failure = new Error(`${name}: HTTP ${response.status}`)
            if (response.status < 500 && response.status !== 429) throw failure
        } catch (error) {
            if (error === failure) throw error
            failure = new Error(`${name}: ${error.message}`)
        }

        if (attempt >= ATTEMPTS) throw failure

        const delay = 2000 * 2 ** (attempt - 1)
        console.warn(
            `${failure.message}; retrying in ${delay / 1000}s (${attempt}/${ATTEMPTS - 1})`
        )
        await new Promise((resolve) => setTimeout(resolve, delay))
    }
}

async function download(dir) {
    fs.mkdirSync(dir, { recursive: true })

    for (const [name, { sha256: expected, url }] of Object.entries(FILES)) {
        const target = path.join(dir, name)

        if (fs.existsSync(target) && sha256(fs.readFileSync(target)) === expected) {
            console.log(`${name}: up to date`)
            continue
        }

        const data = await fetchAsset(name, url)
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
