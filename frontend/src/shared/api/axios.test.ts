import assert from 'node:assert/strict'
import { test } from 'node:test'
import { AxiosError, type InternalAxiosRequestConfig } from 'axios'

import { logoutEvents } from '../emitters/emit-logout'

test('401 signs out the matching session and preserves a replacement session', async () => {
    // Vite normally injects these constants before the requester initializes.
    const defines = {
        __DOMAIN_BACKEND__: '',
        __DOMAIN_OVERRIDE__: '0',
        __NODE_ENV__: 'development'
    }
    const previous = Object.entries(defines).map(([name]) => ({
        name,
        descriptor: Object.getOwnPropertyDescriptor(globalThis, name)
    }))
    Object.assign(globalThis, defines)
    const { instance, setAuthorizationToken } = await import('./axios').finally(() => {
        for (const { name, descriptor } of previous) {
            if (descriptor) Object.defineProperty(globalThis, name, descriptor)
            else Reflect.deleteProperty(globalThis, name)
        }
    })
    let signOuts = 0
    const unsubscribe = logoutEvents.subscribe(() => {
        signOuts++
        setAuthorizationToken('')
    })

    try {
        for (const [sent, current, expected] of [
            ['old-token', 'new-token', 0],
            ['', 'new-token', 0],
            ['new-token', 'new-token', 1]
        ] as const) {
            const started = Promise.withResolvers<InternalAxiosRequestConfig>()
            const complete = Promise.withResolvers<void>()
            setAuthorizationToken(sent)
            const response = instance.get('/unauthorized-test', {
                adapter: async (config) => {
                    started.resolve(config)
                    await complete.promise
                    throw new AxiosError('Unauthorized', AxiosError.ERR_BAD_REQUEST, config, null, {
                        config,
                        data: null,
                        headers: {},
                        status: 401,
                        statusText: 'Unauthorized'
                    })
                }
            })
            const rejected = assert.rejects(response, /Unauthorized/)
            const config = await started.promise
            assert.equal(config.headers.get('Authorization'), sent ? `Bearer ${sent}` : null)
            setAuthorizationToken(current)
            complete.resolve()
            await rejected
            assert.equal(signOuts, expected)
        }
    } finally {
        unsubscribe()
        setAuthorizationToken('')
    }
})
