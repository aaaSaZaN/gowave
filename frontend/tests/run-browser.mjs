import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { build } from 'vite'

const output = mkdtempSync(join(tmpdir(), 'gowave-browser-'))
let browser
try {
    const root = resolve(import.meta.dirname, '..')
    await build({
        configFile: join(root, 'vite.config.ts'),
        root,
        base: './',
        logLevel: 'error',
        define: { __BUILD_ID__: JSON.stringify('browser-test') },
        build: {
            outDir: output,
            emptyOutDir: true,
            copyPublicDir: false,
            rollupOptions: { input: join(root, 'tests/browser.html') }
        }
    })
    const html = join(output, 'tests/browser.html')
    const pageHtml = readFileSync(html, 'utf8')
        .replace(
            /(src|href)="([^"]+)"/g,
            (_match, attribute, value) =>
                `${attribute}="${new URL(value, pathToFileURL(html)).href}"`
        )
        .replace('<head>', '<head><base href="https://gowave.invalid/panel/">')
    writeFileSync(html, pageHtml)
    browser = spawn(
        process.env.CHROMIUM_BIN || 'chromium',
        [
            '--headless',
            '--no-sandbox',
            '--disable-gpu',
            '--disable-background-networking',
            '--allow-file-access-from-files',
            '--remote-debugging-port=0',
            `--user-data-dir=${join(output, 'profile')}`,
            pathToFileURL(html).href
        ],
        { stdio: ['ignore', 'ignore', 'pipe'] }
    )
    // Poll through the debugging protocol using real browser time so layout,
    // ResizeObserver and Web Animations all get their normal rendering frames.
    const port = await new Promise((resolve, reject) => {
        let errors = ''
        const timeout = setTimeout(
            () => reject(new Error(errors || 'Chromium startup timed out')),
            10000
        )
        browser.once('error', (error) => {
            clearTimeout(timeout)
            reject(error)
        })
        browser.once('exit', () => {
            clearTimeout(timeout)
            reject(new Error(errors || 'Chromium exited'))
        })
        browser.stderr.on('data', (chunk) => {
            errors += String(chunk)
            const match = errors.match(/DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)/)
            if (match) {
                clearTimeout(timeout)
                resolve(match[1])
            }
        })
    })
    const pages = await fetch(`http://127.0.0.1:${port}/json/list`).then((response) =>
        response.json()
    )
    const page = pages.find((target) => target.type === 'page')
    if (!page) throw new Error('Chromium did not create a page')
    const socket = new WebSocket(page.webSocketDebuggerUrl)
    const result = await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('Browser smoke tests timed out')), 20000)
        socket.onerror = () => {
            clearTimeout(timeout)
            reject(new Error('Chromium debugging connection failed'))
        }
        socket.onopen = () =>
            socket.send(
                JSON.stringify({
                    id: 1,
                    method: 'Runtime.evaluate',
                    params: {
                        expression: `new Promise((resolve) => {
                    const poll = () => document.body?.dataset.result
                        ? resolve(document.body.dataset.result + ': ' + document.body.textContent)
                        : setTimeout(poll, 50)
                    poll()
                })`,
                        awaitPromise: true,
                        returnByValue: true
                    }
                })
            )
        socket.onmessage = (event) => {
            const message = JSON.parse(event.data)
            if (message.id !== 1) return
            clearTimeout(timeout)
            resolve(message.result?.result?.value || JSON.stringify(message))
        }
    }).finally(() => socket.close())
    if (!String(result).startsWith('passed:')) throw new Error(String(result))
    process.stdout.write('Browser smoke tests passed\n')
} finally {
    if (browser && browser.exitCode === null) {
        browser.kill()
        await once(browser, 'exit')
    }
    rmSync(output, { recursive: true, force: true })
}
