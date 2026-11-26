import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { build } from 'vite'
import { startSvelteFixture } from './svelte-app-fixture.mjs'

const output = mkdtempSync(join(tmpdir(), 'gowave-browser-'))
let browser
let server
try {
    const root = resolve(import.meta.dirname, '..')
    await build({
        configFile: join(root, 'vite.config.ts'),
        root,
        base: './',
        logLevel: 'error',
        define: {
            __BUILD_ID__: JSON.stringify('browser-test'),
            __DOMAIN_BACKEND__: JSON.stringify(''),
            __DOMAIN_OVERRIDE__: JSON.stringify('0'),
            __NODE_ENV__: JSON.stringify('production')
        },
        build: {
            outDir: output,
            emptyOutDir: true
        }
    })
    const fixture = startSvelteFixture(output)
    server = fixture.server
    const url = fixture.url
    browser = spawn(
        process.env.CHROMIUM_BIN || 'chromium',
        [
            '--headless',
            '--no-sandbox',
            '--disable-gpu',
            '--window-size=1440,900',
            '--disable-background-networking',
            '--remote-debugging-port=0',
            `--user-data-dir=${join(output, 'profile')}`,
            url
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
        const timeout = setTimeout(() => reject(new Error('Browser smoke tests timed out')), 90000)
        socket.onerror = () => {
            clearTimeout(timeout)
            reject(new Error('Chromium debugging connection failed'))
        }
        const poll = () => {
            if (socket.readyState !== WebSocket.OPEN) return
            socket.send(
                JSON.stringify({
                    id: 1,
                    method: 'Runtime.evaluate',
                    params: {
                        expression: `(() => {
                            const { result, editorInput } = document.body?.dataset || {}
                            if (result) return result + ': ' + document.body.textContent
                            if (!editorInput) return null
                            delete document.body.dataset.editorInput
                            return { editorInput: JSON.parse(editorInput) }
                        })()`,
                        returnByValue: true
                    }
                })
            )
        }
        socket.onopen = poll
        socket.onmessage = (event) => {
            const message = JSON.parse(event.data)
            if (message.id === 2) {
                if (message.error) {
                    clearTimeout(timeout)
                    reject(new Error(JSON.stringify(message.error)))
                } else {
                    setTimeout(poll, 50)
                }
                return
            }
            if (message.id === 3 || message.id === 4) return
            if (message.id !== 1) return
            const value = message.result?.result?.value
            if (value?.editorInput) {
                const { text, x, y } = value.editorInput
                for (const [id, type] of [
                    [3, 'mousePressed'],
                    [4, 'mouseReleased']
                ]) {
                    socket.send(
                        JSON.stringify({
                            id,
                            method: 'Input.dispatchMouseEvent',
                            params: { type, x, y, button: 'left', clickCount: 1 }
                        })
                    )
                }
                socket.send(
                    JSON.stringify({
                        id: 2,
                        method: 'Input.insertText',
                        params: { text }
                    })
                )
                return
            }
            // Chromium may replace the initial execution context while opening the test HTML.
            if (message.error?.code === -32000 || value === null) {
                setTimeout(poll, 50)
                return
            }
            clearTimeout(timeout)
            resolve(value || JSON.stringify(message))
        }
    }).finally(() => socket.close())
    if (!String(result).startsWith('passed:')) throw new Error(String(result))
    process.stdout.write('Svelte browser smoke tests passed\n')
} finally {
    if (browser?.pid !== undefined && browser.exitCode === null && browser.signalCode === null) {
        browser.kill()
        await once(browser, 'exit')
    }
    server?.stop(true)
    rmSync(output, { recursive: true, force: true })
}
