#!/usr/bin/env bun
import { Database } from 'bun:sqlite'
import { execFileSync } from 'node:child_process'
import { createHmac, randomBytes, randomUUID, scryptSync } from 'node:crypto'
import {
    chmodSync,
    closeSync,
    existsSync,
    lstatSync,
    mkdirSync,
    mkdtempSync,
    openSync,
    readFileSync,
    renameSync,
    rmSync,
    writeFileSync
} from 'node:fs'
import { createServer } from 'node:net'
import { join, resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const base = join(root, '.dev')
const directory = join(base, 'mock')
const databasePath = join(directory, 'gowave.db')
const statePath = join(directory, 'stand.json')
const executable = join(directory, 'gowave-server')
const flags = new Set(process.argv.slice(2))

function environment(secret, database = databasePath) {
    return {
        PATH: process.env['PATH'],
        HOME: process.env['HOME'],
        TZ: 'UTC',
        APP_SECRET: secret,
        DB_DRIVER: 'sqlite',
        DATABASE_URL: database,
        APP_HOST: '127.0.0.1',
        APP_PORT: '3000',
        METRICS_ENABLED: 'false',
        PANEL_DOMAIN: '127.0.0.1:3333',
        SUB_PUBLIC_DOMAIN: '127.0.0.1:3333/api/sub',
        FRONT_END_DOMAIN: 'http://127.0.0.1:3333',
        DOMAIN_BACKEND: '',
        DOMAIN_OVERRIDE: '0',
        NODE_ENV: 'development'
    }
}

function lockStand() {
    const path = join(base, 'mock.lock')
    if (existsSync(path)) {
        const pid = Number(readFileSync(path, 'utf8'))
        if (!Number.isInteger(pid) || pid < 1) throw new Error(`Invalid mock lock: ${path}`)
        try {
            process.kill(pid, 0)
            throw new Error('The mock stand is running; stop it before modifying its database.')
        } catch (error) {
            if (error.code !== 'ESRCH') throw error
            rmSync(path)
        }
    }
    const file = openSync(path, 'wx', 0o600)
    writeFileSync(file, String(process.pid))
    return () => {
        closeSync(file)
        rmSync(path)
    }
}

function seed(path, secret) {
    const database = new Database(path, { strict: true })
    const timestamp = (date = new Date()) =>
        date.toISOString().replace('T', ' ').replace('Z', '+00:00')
    const now = timestamp()
    const insert = (table, values) => {
        const columns = Object.keys(values)
        return database
            .query(
                `INSERT INTO ${table} (${columns.join(',')}) VALUES (${columns.map(() => '?').join(',')})`
            )
            .run(...Object.values(values))
    }
    try {
        database.exec('PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;')
        database.transaction(() => {
            // Match the backend's HMAC + scrypt format, including its hex-encoded salt.
            const salt = randomBytes(16).toString('hex')
            const password = createHmac('sha256', secret).update('root').digest('hex')
            const hash = scryptSync(password, salt, 64, { N: 16384, r: 8, p: 1 }).toString('hex')
            insert('admin', {
                uuid: randomUUID(),
                username: 'root',
                password_hash: `${salt}:${hash}`,
                role: 'ADMIN',
                created_at: now,
                updated_at: now
            })
            const suffix = randomUUID().slice(0, 8)
            const profile = randomUUID()
            const inbound = randomUUID()
            const squad = randomUUID()
            const rawInbound = {
                tag: `mock-${suffix}`,
                listen: '0.0.0.0',
                port: 10000,
                protocol: 'vless',
                settings: { clients: [], decryption: 'none' },
                streamSettings: {
                    network: 'ws',
                    security: 'none',
                    wsSettings: { path: `/mock-${suffix}` }
                }
            }
            insert('config_profiles', {
                uuid: profile,
                view_position: 1,
                name: `Mock ${suffix}`,
                tags: '[]',
                config: JSON.stringify({
                    log: { loglevel: 'warning' },
                    inbounds: [rawInbound],
                    outbounds: [{ tag: 'DIRECT', protocol: 'freedom' }]
                }),
                created_at: now,
                updated_at: now
            })
            insert('config_profile_inbounds', {
                uuid: inbound,
                profile_uuid: profile,
                tag: rawInbound.tag,
                type: 'vless',
                network: 'ws',
                security: 'none',
                port: 10000,
                raw_inbound: JSON.stringify(rawInbound)
            })
            insert('internal_squads', {
                uuid: squad,
                view_position: 1,
                name: 'Mock squad',
                tags: '[]',
                created_at: now,
                updated_at: now
            })
            insert('internal_squad_inbounds', { internal_squad_uuid: squad, inbound_uuid: inbound })
            insert('hosts', {
                uuid: randomUUID(),
                view_position: 1,
                remark: 'Mock host',
                address: `mock-${suffix}.example.test`,
                port: 10000,
                tags: '["MOCK"]',
                config_profile_uuid: profile,
                config_profile_inbound_uuid: inbound
            })
            for (const status of ['ACTIVE', 'DISABLED', 'EXPIRED', 'LIMITED']) {
                const expiry = new Date()
                expiry.setUTCDate(expiry.getUTCDate() + (status === 'EXPIRED' ? -1 : 30))
                const { lastInsertRowid: user } = insert('users', {
                    username: `demo_${status.toLowerCase()}`,
                    status,
                    short_uuid: randomBytes(12).toString('base64url'),
                    traffic_limit_bytes: 100 * 2 ** 30,
                    traffic_limit_strategy: 'NO_RESET',
                    expire_at: timestamp(expiry),
                    tag: 'MOCK',
                    vless_uuid: randomUUID(),
                    trojan_password: randomBytes(24).toString('base64url'),
                    ss_password: randomBytes(24).toString('base64url'),
                    created_at: now,
                    updated_at: now
                })
                const used = (status === 'LIMITED' ? 100 : 25) * 2 ** 30
                insert('user_traffic', {
                    id: user,
                    used_traffic_bytes: used,
                    lifetime_used_traffic_bytes: used
                })
                insert('internal_squad_members', { internal_squad_uuid: squad, user_id: user })
            }
        })()
        if (database.query('PRAGMA foreign_key_check').all().length)
            throw new Error('Invalid fixture relations')
        database.exec('PRAGMA wal_checkpoint(TRUNCATE)')
    } finally {
        database.close()
    }
}

function createDatabase(secret) {
    const temporary = mkdtempSync(join(base, 'mock-seed-'))
    const nextDatabase = join(temporary, 'gowave.db')
    const runtime = join(temporary, 'run')
    mkdirSync(runtime)
    try {
        // This noninteractive CLI command runs the real migrations and default seeding.
        execFileSync(executable, ['rescue', '--list-admins'], {
            cwd: runtime,
            env: environment(secret, nextDatabase),
            stdio: 'pipe'
        })
        seed(nextDatabase, secret)
        chmodSync(nextDatabase, 0o600)
        writeFileSync(join(temporary, 'stand.json'), JSON.stringify({ version: 1, secret }), {
            mode: 0o600
        })
        for (const suffix of ['-wal', '-shm', '-journal'])
            rmSync(databasePath + suffix, { force: true })
        renameSync(nextDatabase, databasePath)
        renameSync(join(temporary, 'stand.json'), statePath)
    } finally {
        rmSync(temporary, { recursive: true, force: true })
    }
}

async function requireFreePorts() {
    for (const port of [3000, 3333]) {
        await new Promise((resolve, reject) => {
            const probe = createServer()
            probe.once('error', () =>
                reject(
                    new Error(
                        `Port ${port} is busy; stop dev before starting or recreating the mock stand.`
                    )
                )
            )
            probe.listen(port, '127.0.0.1', () => probe.close(resolve))
        })
    }
}

async function serve(secret) {
    const runtime = join(directory, 'run')
    mkdirSync(runtime, { recursive: true, mode: 0o700 })
    const children = []
    let interrupted = false
    let timer
    const stop = () => {
        if (timer) return
        for (const child of children) if (child.exitCode === null) child.kill('SIGINT')
        timer = setTimeout(() => {
            for (const child of children) if (child.exitCode === null) child.kill('SIGKILL')
        }, 5000)
        timer.unref()
    }
    const onSignal = () => {
        interrupted = true
        stop()
    }
    process.on('SIGINT', onSignal)
    process.on('SIGTERM', onSignal)
    const options = {
        env: environment(secret),
        stdin: 'inherit',
        stdout: 'inherit',
        stderr: 'inherit'
    }
    try {
        children.push(Bun.spawn([executable], { ...options, cwd: runtime }))
        children.push(
            Bun.spawn(
                [process.execPath, 'run', '--no-orphans', 'dev:frontend', '--host', '127.0.0.1'],
                {
                    ...options,
                    cwd: join(root, 'frontend')
                }
            )
        )
        console.log('Mock panel: http://127.0.0.1:3333 (Ctrl+C stops both servers)')
        const code = await Promise.race(children.map((child) => child.exited))
        if (!interrupted) throw new Error(`Mock dev server exited unexpectedly (${code})`)
    } finally {
        stop()
        await Promise.all(children.map((child) => child.exited))
        clearTimeout(timer)
        process.off('SIGINT', onSignal)
        process.off('SIGTERM', onSignal)
    }
}

async function main() {
    for (const flag of flags) {
        if (!['--recreate', '--serve', '--help'].includes(flag))
            throw new Error(`Unknown option: ${flag}`)
    }
    if (flags.has('--help')) {
        console.log(
            'Usage: bun scripts/mock-db.mjs [--recreate] [--serve]\nWithout --serve, only generate/reuse the mock database.'
        )
        return
    }
    mkdirSync(directory, { recursive: true, mode: 0o700 })
    const unlock = lockStand()
    try {
        if (flags.has('--serve') || flags.has('--recreate')) await requireFreePorts()
        for (const path of [directory, databasePath, statePath]) {
            if (existsSync(path) && lstatSync(path).isSymbolicLink())
                throw new Error(`Refusing mock symlink: ${path}`)
        }
        let state = null
        if (existsSync(statePath)) {
            try {
                state = JSON.parse(readFileSync(statePath, 'utf8'))
            } catch {
                if (!flags.has('--recreate'))
                    throw new Error('Invalid mock marker; use --recreate.')
            }
        }
        const validState = state?.version === 1 && /^[a-f0-9]{64}$/.test(state.secret)
        const generate = flags.has('--recreate') || !existsSync(databasePath)
        if (existsSync(databasePath) && !validState && !flags.has('--recreate')) {
            throw new Error(
                'Existing database has no valid mock marker; use --recreate to replace it.'
            )
        }
        const secret = validState ? state.secret : randomBytes(32).toString('hex')
        if (generate || flags.has('--serve')) {
            execFileSync(
                'go',
                [
                    'build',
                    '-ldflags',
                    '-X gowave/internal/system.AppVersion=dev -X gowave/internal/system.Channel=dev',
                    '-o',
                    executable,
                    './cmd/server'
                ],
                { cwd: join(root, 'backend'), stdio: 'inherit' }
            )
        }
        if (generate) createDatabase(secret)
        console.log(
            `${generate ? 'Seeded' : 'Reusing'} mock database: ${databasePath}\nInitial admin login: root:root`
        )
        if (flags.has('--serve')) await serve(secret)
    } finally {
        unlock()
    }
}

try {
    await main()
} catch (error) {
    console.error(`mock dev: ${error.message}`)
    process.exitCode = 1
}
