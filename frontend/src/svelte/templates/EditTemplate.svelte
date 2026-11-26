<script lang="ts">
    import { UpdateSubscriptionTemplateCommand } from '@remnawave/backend-contract'
    import { SUBSCRIPTION_TEMPLATE_LIST_LINK } from '@shared/constants/templates/template-list-links'
    import { configureMonaco, getTemplateModelPath } from '@shared/utils/monaco/template-schemas'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import type { editor } from 'monaco-editor'
    import { tick, untrack } from 'svelte'
    import { submitCommand } from '../commands'
    import CodeEditor, { type EditorDiagnostic } from '../editor/CodeEditor.svelte'
    import EditorActions from '../editor/EditorActions.svelte'
    import EditorFrame from '../editor/EditorFrame.svelte'
    import TemplatePicker from '../editor/TemplatePicker.svelte'
    import UnsavedChanges from '../editor/UnsavedChanges.svelte'
    import { hostsQuery } from '../hosts/queries'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { templateQuery } from '../subscriptions/queries'
    import { notifications } from '../ui/notification-state.svelte'
    import {
        type Template,
        type TemplateType,
        templateContent,
        templateText,
        templateTypes
    } from './templates'

    let { template, type }: { template: Template; type: TemplateType } = $props()
    // The YAML worker caches parses by URI and document version, which restarts for each editor.
    const modelId = $props.id()
    const initial = untrack(() => templateText(template, type))
    const hosts = createQuery(() => ({
        ...hostsQuery,
        enabled: templateTypes[type].language === 'json'
    }))
    let t = $derived(locale.t)
    let draft = $state(initial)
    let saved = $state(initial)
    let current = $state.raw<editor.IStandaloneCodeEditor | null>(null)
    let dirty = $derived(draft !== saved)
    let showCatalog = $state(false)
    let error = $state('')
    let schemaError = $state('')
    let diagnostics = $state.raw<EditorDiagnostic[]>([])
    let language = $derived(templateTypes[type].language)

    const save = createMutation(() => ({
        mutationFn: (text: string) =>
            submitCommand(UpdateSubscriptionTemplateCommand, {
                uuid: template.uuid,
                ...templateContent(text, type)
            }),
        onSuccess: (updated, text) => {
            queryClient.setQueryData(templateQuery(updated.uuid).queryKey, updated)
            saved = text
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        }
    }))

    $effect(() => {
        if (!current) return
        const snapshot = hosts.data ?? []
        const editorType = type
        const languageMode = language
        schemaError = ''
        void import('@shared/utils/setup-monaco/setup-monaco')
            .then(({ monaco }) => configureMonaco(monaco, languageMode, snapshot, editorType))
            .catch((reason: unknown) => {
                schemaError = reason instanceof Error ? reason.message : String(reason)
            })
    })

    function updateDraft(text: string): void {
        draft = text
        error = ''
    }

    function report(reason: unknown): void {
        error = reason instanceof Error ? reason.message : String(reason)
    }

    function submit(): void {
        if (!dirty || !draft.trim() || save.isPending) return
        error = ''
        save.mutate(draft)
    }

    function shortcut(event: KeyboardEvent): void {
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
            event.preventDefault()
            submit()
        }
    }
</script>

<UnsavedChanges {dirty} />

<svelte:window onkeydown={shortcut} />
<EditorFrame>
    {#snippet children()}
        <CodeEditor
            {language}
            value={draft}
            modelUri={getTemplateModelPath(type, modelId)}
            label={`${templateTypes[type].title} template`}
            onchange={updateDraft}
            onready={(instance) => (current = instance)}
            ondiagnostics={(markers) => (diagnostics = markers)}
        />
    {/snippet}
    {#snippet status()}
        {#if error || save.error || schemaError}
            <p class="editor-status" role="alert">
                <code>{error || save.error?.message || schemaError}</code>
            </p>
        {/if}
        {#if diagnostics.length}
            <ul class="editor-diagnostics" role="status">
                {#each diagnostics as diagnostic}
                    <li class:warning={diagnostic.warning}>
                        <code>{diagnostic.line}:{diagnostic.column} {diagnostic.message}</code>
                    </li>
                {/each}
            </ul>
        {/if}
    {/snippet}
    {#snippet actions()}
        <EditorActions
            {current}
            value={draft}
            saving={save.isPending}
            onsave={submit}
            oncopy={() =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: '',
                    color: 'teal'
                })}
            onerror={report}
            onload={() => (showCatalog = true)}
            saveDisabled={!draft.trim()}
        />
    {/snippet}
</EditorFrame>
{#if showCatalog}
    <TemplatePicker
        catalog={SUBSCRIPTION_TEMPLATE_LIST_LINK}
        {type}
        onclose={() => (showCatalog = false)}
        onload={async (text) => {
            updateDraft(text)
            await tick()
            await current?.getAction('editor.action.formatDocument')?.run()
        }}
    />
{/if}

<style>
    .editor-status,
    .editor-diagnostics {
        margin: 0;
        flex: none;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-md);
        border-top: 1px solid rgb(241 65 65);
        background: rgb(241 65 65 / 10%);
    }
    .editor-diagnostics {
        max-height: 8rem;
        overflow-y: auto;
        list-style: none;
        border-color: var(--mantine-color-default-border);
        background: var(--gw-raised-bg);
    }
    .editor-status code,
    .editor-diagnostics code {
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        color: var(--mantine-color-red-4);
    }
    .editor-diagnostics .warning code {
        color: light-dark(var(--mantine-color-orange-9), var(--mantine-color-yellow-4));
    }
</style>
