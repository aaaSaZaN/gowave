<script lang="ts">
    import {
        CreateSnippetCommand,
        DeleteSnippetCommand,
        type GetSnippetsCommand,
        SyncSnippetCommand,
        UpdateSnippetCommand
    } from '@remnawave/backend-contract'
    import { MonacoSetupSnippetsFeature } from '@shared/utils/monaco/xray-schemas'
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { submitCommand, submitVoidCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import { notifications } from '../ui/notification-state.svelte'
    import { snippetsQuery } from './queries'

    type Snippet = GetSnippetsCommand.Response['response']['snippets'][number]
    let { close }: { close: () => void } = $props()
    let dialog: HTMLDialogElement
    let t = $derived(locale.t)
    const title = $props.id()
    const snippets = createQuery(() => snippetsQuery)
    let editing = $state<Snippet | null | undefined>(undefined)
    let name = $state('')
    let draft = $state('')
    let busy = $state(false)
    let error = $state('')
    let pendingSync = $state('')

    onMount(() => dialog.showModal())
    $effect(() => {
        if (editing !== undefined) void MonacoSetupSnippetsFeature.setup(locale.language)
    })

    function edit(snippet: Snippet | null): void {
        editing = snippet
        name = snippet?.name ?? ''
        draft = JSON.stringify(snippet?.snippet ?? [], null, 4)
        error = ''
    }

    async function run(action: () => Promise<void>): Promise<void> {
        if (busy) return
        error = ''
        busy = true
        try {
            await action()
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        } catch (cause) {
            error = cause instanceof Error ? cause.message : String(cause)
        } finally {
            busy = false
        }
    }

    async function save(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        await run(async () => {
            const snippet: unknown = JSON.parse(draft)
            if (!Array.isArray(snippet) || !snippet.length) {
                throw new Error(t('snippets.drawer.widget.snippet-cannot-be-empty'))
            }
            if (
                snippet.some(
                    (item: unknown) =>
                        !item ||
                        typeof item !== 'object' ||
                        Array.isArray(item) ||
                        !Object.keys(item).length
                )
            ) {
                throw new Error(t('snippets.drawer.widget.snippet-cannot-contain-empty-objects'))
            }
            const command = editing ? UpdateSnippetCommand : CreateSnippetCommand
            const saved = await submitCommand(command, { name: name.trim(), snippet })
            queryClient.setQueryData(snippetsQuery.queryKey, saved)
            if (editing) pendingSync = name.trim()
            editing = undefined
        })
    }

    async function remove(name: string): Promise<void> {
        if (!window.confirm(`${t('common.action.delete')} ${name}?`)) return
        await run(async () => {
            await submitVoidCommand(DeleteSnippetCommand, { name })
            await queryClient.invalidateQueries({ queryKey: snippetsQuery.queryKey })
            pendingSync = name
        })
    }

    async function sync(name: string): Promise<void> {
        await run(async () => {
            await submitVoidCommand(SyncSnippetCommand, { name })
            if (name === pendingSync) pendingSync = ''
        })
    }
</script>

<dialog
    bind:this={dialog}
    onclose={close}
    oncancel={(event) => cancelDialog(event, busy)}
    aria-labelledby={title}
>
    <div class="button-row">
        <h2 id={title}>Xray snippets</h2>
        <button type="button" disabled={busy} onclick={() => dismissDialog(dialog)}>
            {t('common.action.close')}
        </button>
    </div>
    {#if editing !== undefined}
        <form onsubmit={(event) => void save(event)}>
            <label>
                {t('common.field.name')}
                <input required bind:value={name} readonly={Boolean(editing)} disabled={busy}>
            </label>
            <div class="snippet-editor">
                <CodeEditor
                    language="json"
                    modelUri="snippet://editor/fragment.json"
                    label="Snippet JSON"
                    value={draft}
                    readOnly={busy}
                    onchange={(value) => (draft = value)}
                />
            </div>
            <div class="button-row">
                <button
                    type="button"
                    disabled={busy}
                    onclick={() => {
                        editing = undefined
                        error = ''
                    }}
                >
                    {t('common.action.cancel')}
                </button>
                <button type="submit" disabled={busy || !name.trim()}>
                    {t('common.action.save')}
                </button>
            </div>
        </form>
    {:else}
        <button type="button" disabled={busy || !snippets.isSuccess} onclick={() => edit(null)}>
            {t('snippets.drawer.widget.create-snippet')}
        </button>
        {#if snippets.isPending}
            <RouteLoading compact />
        {:else if snippets.error}
            <p role="alert">{snippets.error.message}</p>
            <button type="button" onclick={() => void snippets.refetch()}>
                {t('common.action.try-again')}
            </button>
        {:else if !snippets.data.snippets.length}
            <p>{t('common.message.no-data-available')}</p>
        {:else}
            <ul>
                {#each snippets.data.snippets as snippet (snippet.name)}
                    <li>
                        <strong>{snippet.name}</strong>
                        <div class="button-row">
                            <button type="button" disabled={busy} onclick={() => edit(snippet)}>
                                {t('common.action.edit')}
                            </button>
                            <button
                                type="button"
                                disabled={busy}
                                onclick={() => void sync(snippet.name)}
                            >
                                Sync
                            </button>
                            <button
                                type="button"
                                disabled={busy}
                                onclick={() => void remove(snippet.name)}
                            >
                                {t('common.action.delete')}
                            </button>
                        </div>
                    </li>
                {/each}
            </ul>
        {/if}
    {/if}
    {#if pendingSync}
        <p>{t('apply-to-nodes.title')} · {pendingSync}</p>
        <p>{t('apply-to-nodes.apply-description')}</p>
        <div class="button-row">
            <button type="button" disabled={busy} onclick={() => void sync(pendingSync)}>
                {t('apply-to-nodes.apply')}
            </button>
            <button type="button" disabled={busy} onclick={() => (pendingSync = '')}>
                {t('apply-to-nodes.later')}
            </button>
        </div>
    {/if}
    {#if error}
        <p role="alert">{error}</p>
    {/if}
</dialog>

<style>
    dialog {
        width: min(58rem, calc(100vw - 2rem));
    }
    .snippet-editor {
        height: 40dvh;
        margin-block: 0.75rem;
    }
    li {
        margin-block: 0.75rem;
    }
</style>
