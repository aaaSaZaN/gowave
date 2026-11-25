<script lang="ts">
    import {
        BASE_MONACO_OPTIONS,
        MONACO_LIGHT_THEME_NAME,
        MONACO_THEME_NAME
    } from '@shared/constants/monaco-theme'
    import { describeJsonPath } from '@shared/utils/monaco/json-path'
    import { repairJsonInEditor } from '@shared/utils/monaco/repair-json'
    import { setupSuggestWidget } from '@shared/utils/setup-monaco/setup-suggest-monaco'
    import { parse } from 'jsonc-parser'
    import type { editor } from 'monaco-editor'
    import { onMount } from 'svelte'
    import { generateX25519 } from '../../widgets/dashboard/config-profiles/keypair-generator/keypair-utils'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { theme } from '../theme.svelte'

    interface Props {
        language: string
        modelUri: string
        label: string
        value: string
        readOnly?: boolean
        onchange: (text: string) => void
        onready?: (instance: editor.IStandaloneCodeEditor | null) => void
    }

    let { language, modelUri, label, value, readOnly = false, onchange, onready }: Props = $props()
    let container: HTMLDivElement
    let loading = $state(true)
    let error = $state('')
    let message = $state('')
    let path = $state<string[]>([])
    let current = $state.raw<editor.IStandaloneCodeEditor | null>(null)
    let runtime: typeof import('@shared/utils/setup-monaco/setup-monaco') | undefined
    let updatingModel = false

    function currentTheme(): string {
        return theme.scheme === 'light' ? MONACO_LIGHT_THEME_NAME : MONACO_THEME_NAME
    }

    onMount(() => {
        let active = true
        let dispose: (() => void) | undefined

        void import('@shared/utils/setup-monaco/setup-monaco')
            .then((module) => {
                if (!active) return
                runtime = module
                const { monaco } = module
                const model = monaco.editor.createModel(value, language, monaco.Uri.parse(modelUri))
                dispose = () => model.dispose()
                const instance = monaco.editor.create(container, {
                    ...BASE_MONACO_OPTIONS,
                    model,
                    readOnly,
                    ariaLabel: label,
                    theme: currentTheme()
                })
                dispose = () => {
                    instance.dispose()
                    model.dispose()
                }
                setupSuggestWidget(instance)

                let documentValue: unknown
                let documentText = ''
                function refreshPath(): void {
                    if (language !== 'json') return
                    const [visible] = instance.getVisibleRanges()
                    if (!visible) return
                    const offset = model.getOffsetAt({
                        lineNumber: visible.startLineNumber,
                        column: model.getLineMaxColumn(visible.startLineNumber)
                    })
                    path = describeJsonPath(documentText, offset, documentValue)
                }
                function updateSnapshot(): void {
                    documentText = model.getValue()
                    if (language === 'json') documentValue = parse(documentText)
                    refreshPath()
                }

                if (language === 'json') {
                    instance.addAction({
                        id: 'gowave.repairJson',
                        label: 'Repair JSON',
                        contextMenuGroupId: '1_modification',
                        contextMenuOrder: 1.32,
                        run: async () => {
                            const result = repairJsonInEditor(instance)
                            if (result === 'repaired') {
                                await instance.getAction('editor.action.formatDocument')?.run()
                                message = 'JSON repaired'
                            } else {
                                message =
                                    result === 'failed'
                                        ? 'Could not repair this JSON'
                                        : 'Nothing to repair'
                            }
                        }
                    })
                    if (modelUri.startsWith('xray-config://')) {
                        instance.addAction({
                            id: 'gowave.generateX25519KeyPair',
                            label: locale.t('keypair.widget.generate-key-pair'),
                            contextMenuGroupId: '1_modification',
                            contextMenuOrder: 1.5,
                            run: async () => {
                                const { privateKey, password } = generateX25519()
                                try {
                                    await navigator.clipboard.writeText(
                                        `Private key: ${privateKey}\nPublic key: ${password}`
                                    )
                                    message = locale.t('common.message.copied')
                                } catch {
                                    message = locale.t('common.message.error')
                                }
                            }
                        })
                    }
                    instance.onDidScrollChange(refreshPath)
                }
                instance.onDidChangeModelContent(() => {
                    updateSnapshot()
                    if (!updatingModel) onchange(documentText)
                })
                updateSnapshot()
                current = instance
                loading = false
                onready?.(instance)
            })
            .catch((reason: unknown) => {
                if (!active) return
                dispose?.()
                dispose = undefined
                current = null
                onready?.(null)
                error = reason instanceof Error ? reason.message : locale.t('common.message.error')
                loading = false
            })

        return () => {
            active = false
            current = null
            onready?.(null)
            dispose?.()
        }
    })

    $effect(() => {
        const activeTheme = currentTheme()
        if (current) runtime?.monaco.editor.setTheme(activeTheme)
    })

    $effect(() => {
        current?.updateOptions({ readOnly, ariaLabel: label })
    })

    $effect(() => {
        const instance = current
        const text = value
        if (instance && instance.getValue() !== text) {
            updatingModel = true
            try {
                instance.setValue(text)
            } finally {
                updatingModel = false
            }
        }
    })
</script>

<div class="code-editor" dir="ltr">
    {#if error}
        <p role="alert">{error}</p>
    {:else if loading}
        <RouteLoading compact />
    {/if}
    {#if language === 'json'}
        <div class="path" title={path.join(' › ')}>{path.join(' › ')}</div>
    {/if}
    <div class="monaco-host" bind:this={container}></div>
    {#if message}
        <p role="status">{message}</p>
    {/if}
</div>

<style>
    .code-editor {
        display: flex;
        flex-direction: column;
        flex: 1 1 auto;
        min-width: 0;
        min-height: 0;
    }

    .monaco-host {
        flex: 1 1 auto;
        min-height: 0;
    }

    .path {
        display: flex;
        flex: 0 0 32px;
        align-items: center;
        padding: 0 var(--mantine-spacing-md);
        border-bottom: 1px solid var(--mantine-color-dark-4);
        background: var(--gw-raised-bg);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-family: monospace;
    }
</style>
