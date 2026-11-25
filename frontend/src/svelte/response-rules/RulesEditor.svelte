<script lang="ts">
    import {
        REMNAWAVE_CLIENT_TYPE_BROWSER,
        REMNAWAVE_CLIENT_TYPE_HEADER,
        TestSrrMatcherCommand,
        type TSubscriptionTemplateType,
        UpdateSubscriptionSettingsCommand
    } from '@remnawave/backend-contract'
    import { getAuthorizationToken, getBackendDomain } from '@shared/api/axios'
    import {
        type SubscriptionSettings,
        SubscriptionSettingsResponseSchema,
        UpdateSubscriptionSettingsBodySchema
    } from '@shared/api/schemas/subscription-settings.schema'
    import { SRR_TEMPLATES_LIST_LINK } from '@shared/constants/templates'
    import { setupResponseRulesSchema } from '@shared/utils/monaco/response-rules-schema'
    import { createMutation } from '@tanstack/svelte-query'
    import type { editor } from 'monaco-editor'
    import { onMount, tick, untrack } from 'svelte'
    import { ZodError } from 'zod'
    import {
        readMiscPreferences,
        writeMiscPreferences
    } from '../../entities/dashboard/misc-store/storage'
    import { submitCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import EditorActions from '../editor/EditorActions.svelte'
    import EditorFrame from '../editor/EditorFrame.svelte'
    import TemplatePicker from '../editor/TemplatePicker.svelte'
    import UnsavedChanges from '../editor/UnsavedChanges.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import Button from '../ui/Button.svelte'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import Icon from '../ui/Icon.svelte'

    let {
        settings,
        groupedTemplates
    }: {
        settings: SubscriptionSettings
        groupedTemplates: Partial<Record<TSubscriptionTemplateType, string[]>>
    } = $props()

    const initial = untrack(() => JSON.stringify(settings.responseRules, null, 2))
    let saved = $state(initial)
    let draft = $state(initial)
    let dirty = $derived(draft !== saved)
    let current = $state.raw<editor.IStandaloneCodeEditor | null>(null)
    let importOpen = $state(false)
    let notice = $state('')
    let error = $state('')
    let curlCommand = $state('')
    let warning: HTMLDialogElement
    let debuggerDialog: HTMLDialogElement
    let t = $derived(locale.t)

    const save = createMutation(() => ({
        mutationFn: (responseRules: unknown) =>
            submitCommand(
                {
                    ...UpdateSubscriptionSettingsCommand,
                    RequestBodySchema: UpdateSubscriptionSettingsBodySchema,
                    ResponseSchema: SubscriptionSettingsResponseSchema
                },
                { uuid: settings.uuid, responseRules }
            ),
        onSuccess: (updated) => {
            saved = JSON.stringify(updated.responseRules, null, 2)
            draft = saved
            queryClient.setQueryData(['subscriptionSettings', 'getSubscriptionSettings'], updated)
            notice = t('response-rules-editor-actions.feature.saved')
        },
        onError: (reason) => {
            if (reason instanceof ZodError) {
                error = reason.issues
                    .map((issue) => `${issue.path.join('.')}: ${issue.message}`)
                    .join('\n')
            } else {
                error = reason.message
            }
        }
    }))

    onMount(() => {
        try {
            if (!readMiscPreferences(localStorage).srrAdvancedModalClosed) {
                warning.showModal()
                warning.focus({ preventScroll: true })
            }
        } catch {
            warning.showModal()
            warning.focus({ preventScroll: true })
        }
    })

    $effect(() => setupResponseRulesSchema(groupedTemplates))

    function dismissWarning(): void {
        try {
            writeMiscPreferences(localStorage, { srrAdvancedModalClosed: true })
        } catch {
            // Closing the dialog also works when browser storage is disabled.
        }
    }

    function changeDraft(value: string): void {
        draft = value
        notice = ''
        error = ''
    }

    function parseDraft(): { value: unknown } | undefined {
        notice = ''
        error = ''
        try {
            const value: unknown = JSON.parse(draft)
            return { value }
        } catch {
            error = t('config-editor-actions.feature.failed-to-save-invalid-json')
            return undefined
        }
    }

    function saveRules(): void {
        if (!dirty || save.isPending) return
        const parsed = parseDraft()
        if (parsed) save.mutate(parsed.value)
    }

    function report(reason: unknown): void {
        error = reason instanceof Error ? reason.message : t('common.message.error')
    }

    async function formatDocument(): Promise<void> {
        await current?.getAction('editor.action.formatDocument')?.run()
    }

    async function loadTemplate(text: string): Promise<void> {
        changeDraft(text)
        await tick()
        await formatDocument()
        notice = t('use-download-template.template-loaded')
    }

    function copy(text: string): void {
        void navigator.clipboard.writeText(text).then(() => {
            notice = t('common.message.copied')
        }, report)
    }

    function openDebugger(): void {
        const parsed = parseDraft()
        if (!parsed) return
        const quote = (value: string) => `'${value.replaceAll("'", "'\\''")}'`
        const endpoint = new URL(
            TestSrrMatcherCommand.TSQ_url,
            getBackendDomain() || window.location.origin
        )
        curlCommand = [
            `curl -X POST ${quote(endpoint.href)}`,
            `-H ${quote('accept: application/json')}`,
            `-H ${quote('Content-Type: application/json')}`,
            `-H ${quote(`Authorization: Bearer ${getAuthorizationToken()}`)}`,
            `-H ${quote(`${REMNAWAVE_CLIENT_TYPE_HEADER}: ${REMNAWAVE_CLIENT_TYPE_BROWSER}`)}`,
            `-d ${quote(JSON.stringify({ responseRules: parsed.value }))}`
        ].join(' \\\n')
        debuggerDialog.showModal()
    }
</script>

<UnsavedChanges {dirty} />

<EditorFrame>
    <CodeEditor
        value={draft}
        language="json"
        modelUri="response-rules://config.json"
        label={t('constants.response-rules')}
        readOnly={save.isPending}
        onchange={changeDraft}
        onready={(instance) => (current = instance)}
    />
    {#snippet status()}
        {#if error}
            <p class="editor-output" role="alert">{error}</p>
        {:else if notice}
            <p class="editor-output" role="status">{notice}</p>
        {/if}
    {/snippet}
    {#snippet actions()}
        <EditorActions
            {current}
            value={draft}
            saving={save.isPending}
            saveDisabled={!dirty}
            saveTone={dirty ? 'teal' : 'gray'}
            onsave={saveRules}
            oncopy={() => (notice = t('common.message.copied'))}
            onload={() => (importOpen = true)}
            onerror={report}
        >
            {#snippet extra()}
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    disabled={!current}
                    onclick={() => {
                        openDebugger()
                    }}
                >
                    <Icon name="TbBug" size={16} />
                    {t('response-rules-editor-actions.feature.debugger')}
                </button>
            {/snippet}
        </EditorActions>
    {/snippet}
</EditorFrame>

<dialog
    oncancel={cancelDialog}
    class="rules-warning"
    bind:this={warning}
    tabindex="-1"
    closedby="any"
    onclose={dismissWarning}
    aria-labelledby="rules-warning-title"
>
    <div class="warning-content">
        <span class="warning-icon"><Icon name="TbAlertTriangle" size={40} /></span>
        <div class="warning-copy">
            <h2 id="rules-warning-title">{t('common.message.warning')}</h2>
            <p>{t('srr-advanced-warning-overlay.warning-line-1')}</p>
            <ul>
                <li>
                    <span><Icon name="TbAlertTriangle" size={12} /></span>
                    <div>
                        <strong
                            >{t(
                                'srr-advanced-warning-overlay.incorrect-configuration-line-1'
                            )}</strong
                        > {t('srr-advanced-warning-overlay.incorrect-configuration-line-2')}
                    </div>
                </li>
                <li>
                    <span><Icon name="TbAlertTriangle" size={12} /></span>
                    <div>
                        <strong
                            >{t(
                                'srr-advanced-warning-overlay.review-settings-carefully-line-1'
                            )}</strong
                        > {t('srr-advanced-warning-overlay.review-settings-carefully-line-2')}
                    </div>
                </li>
            </ul>
        </div>
        <Button
            color="orange"
            variant="light"
            icon="TbCheck"
            iconSize={18}
            size="md"
            onclick={() => dismissDialog(warning)}
            >{t('common.action.continue')}</Button
        >
    </div>
</dialog>

<dialog
    oncancel={cancelDialog}
    bind:this={debuggerDialog}
    onclose={() => {
        curlCommand = ''
    }}
    aria-labelledby="rules-debugger-title"
>
    <h2 id="rules-debugger-title">
        {t('response-rules-editor-actions.feature.response-rules-debugger')}
    </h2>
    <p>{t('response-rules-editor-actions.feature.response-rules-debugger-description-1')}</p>
    <label>
        {t('response-rules-editor-actions.feature.curl-command')}
        <textarea readonly rows="9" value={curlCommand} dir="ltr"></textarea>
    </label>
    <div class="button-row">
        <button type="button" onclick={() => dismissDialog(debuggerDialog)}>
            {t('common.action.close')}
        </button>
        <button type="button" onclick={() => copy(curlCommand)}>
            {t('response-rules-editor-actions.feature.copy-curl-command')}
        </button>
    </div>
</dialog>

{#if importOpen}
    <TemplatePicker
        catalog={SRR_TEMPLATES_LIST_LINK}
        type="SRR"
        onload={loadTemplate}
        onclose={() => {
            importOpen = false
        }}
    />
{/if}

<style>
    .rules-warning {
        width: min(calc(100% - 40px), 440px);
        padding: 12px;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-body);
        outline: none;
    }
    .rules-warning::backdrop {
        background: rgb(0 0 0 / 60%);
    }
    .warning-content,
    .warning-copy {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-xl);
    }
    .warning-content {
        gap: 32px;
    }
    .warning-copy {
        gap: var(--mantine-spacing-md);
        color: var(--mantine-color-dimmed);
    }
    .warning-icon {
        width: 80px;
        height: 80px;
        display: grid;
        place-items: center;
        border: 3px solid rgb(251 146 60 / 30%);
        border-radius: var(--mantine-radius-xl);
        background: linear-gradient(135deg, rgb(251 146 60 / 15%), rgb(251 146 60 / 5%));
        color: var(--mantine-color-orange-light-color);
        box-shadow: 0 8px 32px rgb(251 146 60 / 20%);
    }
    .warning-copy h2 {
        margin: 0;
        color: var(--mantine-color-orange-4);
        font-size: var(--mantine-h2-font-size);
        font-weight: var(--mantine-h2-font-weight);
        line-height: var(--mantine-h2-line-height);
    }
    .warning-copy p {
        margin: 0;
        font-weight: 500;
        text-align: center;
        line-height: var(--mantine-line-height-md);
    }
    .warning-copy ul {
        width: 100%;
        margin: 0;
        padding: 0;
        list-style: none;
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    .warning-copy li {
        display: flex;
        align-items: center;
        gap: 12px;
        font-size: var(--mantine-font-size-sm);
        line-height: 1;
    }
    .warning-copy li strong {
        font-size: var(--mantine-font-size-md);
        line-height: var(--mantine-line-height);
    }
    .warning-copy li span {
        width: 20px;
        height: 20px;
        flex: none;
        display: grid;
        place-items: center;
        border-radius: 50%;
        background: var(--mantine-color-orange-light);
        color: var(--mantine-color-orange-light-color);
    }
    .editor-output {
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-md);
        max-height: 8rem;
        margin: 0;
        overflow: auto;
        white-space: pre-wrap;
    }

    textarea {
        display: block;
        width: 100%;
        margin-block: 0.5rem;
        resize: vertical;
    }
</style>
