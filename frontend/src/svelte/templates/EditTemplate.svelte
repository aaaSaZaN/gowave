<script lang="ts">
    import { UpdateSubscriptionTemplateCommand } from '@remnawave/backend-contract'
    import { SUBSCRIPTION_TEMPLATE_LIST_LINK } from '@shared/constants/templates/template-list-links'
    import { configureMonaco, getTemplateModelPath } from '@shared/utils/monaco/template-schemas'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import type { editor } from 'monaco-editor'
    import { tick, untrack } from 'svelte'
    import { submitCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import EditorActions from '../editor/EditorActions.svelte'
    import EditorFrame from '../editor/EditorFrame.svelte'
    import TemplatePicker from '../editor/TemplatePicker.svelte'
    import UnsavedChanges from '../editor/UnsavedChanges.svelte'
    import { hostsQuery } from '../hosts/queries'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { templateQuery } from '../subscriptions/queries'
    import {
        type Template,
        type TemplateType,
        templateContent,
        templateText,
        templateTypes
    } from './templates'

    let { template, type }: { template: Template; type: TemplateType } = $props()
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
    let notice = $state('')
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
            notice = t('common.message.operation-completed')
        }
    }))

    $effect(() => {
        if (!current) return
        const snapshot = hosts.data ?? []
        const editorType = type
        const languageMode = language
        void import('@shared/utils/setup-monaco/setup-monaco')
            .then(({ monaco }) => configureMonaco(monaco, languageMode, snapshot, editorType))
            .catch(report)
    })

    function updateDraft(text: string): void {
        draft = text
        error = ''
        notice = ''
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
            modelUri={getTemplateModelPath(type)}
            label={`${templateTypes[type].title} template`}
            onchange={updateDraft}
            onready={(instance) => (current = instance)}
        />
    {/snippet}
    {#snippet status()}
        {#if error || save.error}
            <p class="editor-status" role="alert">{error || save.error?.message}</p>
        {:else if notice}
            <p class="editor-status" role="status">{notice}</p>
        {/if}
    {/snippet}
    {#snippet actions()}
        <EditorActions
            {current}
            value={draft}
            saving={save.isPending}
            onsave={submit}
            oncopy={() => (notice = t('common.message.copied'))}
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
    .editor-status {
        margin: 0;
        flex: none;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-md);
        border-top: 1px solid var(--mantine-color-default-border);
    }
</style>
