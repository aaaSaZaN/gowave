import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'
import webfontDownload from 'vite-plugin-webfont-dl'

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), '')
    return {
        base: './',
        assetsInclude: ['**/*.lottie'],
        plugins: [react(), webfontDownload()],
        optimizeDeps: {
            include: [
                'html-parse-stringify',
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
                    minify: { compress: { dropConsole: true } },
                    codeSplitting: {
                        groups: [
                            {
                                name: 'react',
                                test: /node_modules[\\/](react|react-dom|react-router)[\\/]/
                            },
                            {
                                name: 'markdown',
                                test: /node_modules[\\/](react-markdown|remark-gfm|rehype-raw)[\\/]/
                            },
                            {
                                name: 'icons',
                                test: /node_modules[\\/](react-icons|@lottiefiles[\\/]dotlottie-react)[\\/]/
                            },
                            {
                                name: 'zod',
                                test: /node_modules[\\/](axios|zod|zustand|zod-to-json-schema)[\\/]/
                            },
                            {
                                name: 'utils',
                                test: /node_modules[\\/](jsonc-parser|json-edit-react|dayjs)[\\/]/
                            },
                            {
                                name: 'mantine',
                                test: /node_modules[\\/]@mantine[\\/](core|hooks|dates|nprogress|notifications|modals)[\\/]/
                            },
                            {
                                name: 'gowave',
                                test: /node_modules[\\/]@gowave[\\/](backend-contract|subscription-page-types)[\\/]/
                            },
                            {
                                name: 'i18n',
                                test: /node_modules[\\/](i18next)[\\/]/
                            },
                            {
                                name: 'motion',
                                test: /node_modules[\\/](motion|motion-dom|motion-utils)[\\/]/
                            },
                            {
                                name: 'charts',
                                test: /node_modules[\\/](recharts|highcharts)[\\/]/
                            },
                            {
                                name: 'dnd',
                                test: /node_modules[\\/]@dnd-kit[\\/](abstract|dom|helpers|react)[\\/]/
                            },
                            {
                                name: 'mantinetable',
                                test: /node_modules[\\/](@kastov[\\/]mantine-react-table-open|mantine-datatable)[\\/]/
                            },
                            {
                                name: 'prettier',
                                test: /node_modules[\\/](prettier|vscode-languageserver-types)[\\/]/
                            },
                            {
                                name: 'monaco',
                                test: /node_modules[\\/](monaco-editor|monaco-yaml)[\\/]/
                            },
                            {
                                name: 'tanstack',
                                test: /node_modules[\\/]@tanstack[\\/](react-query|react-table|react-virtual)[\\/]/
                            },
                            {
                                name: 'xterm',
                                test: /node_modules[\\/]@xterm[\\/]/
                            }
                        ]
                    }
                }
            }
        },
        define: {
            __DOMAIN_BACKEND__: JSON.stringify(env.DOMAIN_BACKEND || 'example.com').trim(),
            __NODE_ENV__: JSON.stringify(process.env.NODE_ENV).trim(),
            __DOMAIN_OVERRIDE__: JSON.stringify(env.DOMAIN_OVERRIDE || '0').trim(),
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
                {
                    find: /^monaco-editor\/esm\/vs\/(.*)$/,
                    replacement: 'monaco-editor/$1'
                }
            ]
        }
    }
})
