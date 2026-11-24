import { fileURLToPath } from 'node:url'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig, loadEnv } from 'vite'
import webfontDownload from 'vite-plugin-webfont-dl'

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), '')
    const telegramChatUrl = new URL(env['TELEGRAM_CHAT_URL']?.trim() || 'https://telegram.org')
    if (!['https:', 'http:'].includes(telegramChatUrl.protocol)) {
        throw new Error('TELEGRAM_CHAT_URL must be an HTTP(S) URL')
    }
    return {
        base: './',
        assetsInclude: ['**/*.lottie'],
        plugins: [svelte(), webfontDownload()],
        optimizeDeps: {
            include: [
                'monaco-editor/editor',
                'monaco-editor/features/register.all',
                'monaco-editor/languages/definitions/yaml/yaml',
                'monaco-editor/languages/features/json/register'
            ]
        },
        build: {
            target: 'esnext',
            outDir: 'dist',
            chunkSizeWarningLimit: 1000000,
            rollupOptions: {
                onwarn(warning, defaultHandler) {
                    if (warning.code === 'COMMONJS_VARIABLE_IN_ESM') return
                    defaultHandler(warning)
                },
                output: {
                    minify: { compress: { dropConsole: true } }
                }
            }
        },
        define: {
            __TELEGRAM_CHAT_URL__: JSON.stringify(telegramChatUrl.href),
            __DOMAIN_BACKEND__: JSON.stringify(env['DOMAIN_BACKEND']?.trim() || ''),
            __NODE_ENV__: JSON.stringify(process.env['NODE_ENV'] ?? mode),
            __DOMAIN_OVERRIDE__: JSON.stringify(env['DOMAIN_OVERRIDE'] || '0').trim(),
            // Busts the browser cache of unhashed files such as locales on each build.
            __BUILD_ID__: JSON.stringify(Date.now().toString(36))
        },
        server: {
            host: '0.0.0.0',
            port: 3333,
            cors: true,
            strictPort: true,
            allowedHosts: true,
            proxy: {
                '/api': {
                    target: 'http://127.0.0.1:3000',
                    changeOrigin: true,
                    headers: {
                        'x-forwarded-proto': 'https',
                        'x-forwarded-for': '127.0.0.1',
                        'X-Remnawave-Client-Type': 'browser'
                    }
                }
            },
            hmr: {
                overlay: false
            }
        },
        resolve: {
            tsconfigPaths: true,
            alias: [
                // The pinned contract ships declarations in build/backend and browser JS in build/frontend.
                {
                    find: '@remnawave/backend-contract/build/backend',
                    replacement: '@remnawave/backend-contract/build/frontend'
                },
                {
                    find: '@shared',
                    replacement: fileURLToPath(new URL('./src/shared', import.meta.url))
                },
                {
                    find: '@entities',
                    replacement: fileURLToPath(new URL('./src/entities', import.meta.url))
                },
                {
                    find: /^monaco-editor\/esm\/vs\/(.*)$/,
                    replacement: 'monaco-editor/$1'
                }
            ]
        }
    }
})
