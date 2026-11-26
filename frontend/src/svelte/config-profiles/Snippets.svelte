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
    import { submitCommand, submitVoidCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import Overlay from '../ui/Overlay.svelte'
    import TextInput from '../ui/TextInput.svelte'
    import { snippetsQuery } from './queries'

    const confirmAction = useConfirmation()

    type Snippet = GetSnippetsCommand.Response['response']['snippets'][number]
    let { close }: { close: () => void } = $props()
    let t = $derived(locale.t)
    const snippets = createQuery(() => snippetsQuery)
    let editing = $state<Snippet | null | undefined>(undefined)
    let name = $state('')
    let draft = $state('')
    let busy = $state(false)
    let error = $state('')
    let saved = $state(false)
    let pendingSync = $state('')

    $effect(() => {
        if (editing !== undefined) void MonacoSetupSnippetsFeature.setup(locale.language)
    })

    function edit(snippet: Snippet | null): void {
        saved = false
        editing = snippet
        name = snippet?.name ?? ''
        draft = JSON.stringify(snippet?.snippet ?? [], null, 4)
        error = ''
    }

    async function run(action: () => Promise<void>): Promise<void> {
        if (busy) return
        saved = false
        error = ''
        busy = true
        try {
            await action()
            saved = true
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
        if (!(await confirmAction(`${t('common.action.delete')} ${name}?`))) return
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

<Overlay
    title="Xray snippets"
    icon="TbCode"
    width={928}
    {close}
    preventClose={busy}
    fullscreenOnMobile={false}
>
    {#if editing !== undefined}
        <form onsubmit={(event) => void save(event)}>
            <TextInput
                label={t('common.field.name')}
                required
                bind:value={name}
                readonly={Boolean(editing)}
                disabled={busy}
            />
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
                <Button
                    disabled={busy}
                    onclick={() => {
                        editing = undefined
                        error = ''
                    }}
                >
                    {t('common.action.cancel')}
                </Button>
                <Button type="submit" disabled={busy || !name.trim()}>
                    {t('common.action.save')}
                </Button>
            </div>
        </form>
    {:else}
        <Button disabled={busy || !snippets.isSuccess} onclick={() => edit(null)}>
            {t('snippets.drawer.widget.create-snippet')}
        </Button>
        {#if snippets.isPending}
            <RouteLoading compact />
        {:else if snippets.error}
            <p role="alert">{snippets.error.message}</p>
            <Button onclick={() => void snippets.refetch()}>
                {t('common.action.try-again')}
            </Button>
        {:else if !snippets.data.snippets.length}
            <p>{t('common.message.no-data-available')}</p>
        {:else}
            <ul>
                {#each snippets.data.snippets as snippet (snippet.name)}
                    <li>
                        <strong>{snippet.name}</strong>
                        <div class="button-row">
                            <Button disabled={busy} onclick={() => edit(snippet)}>
                                {t('common.action.edit')}
                            </Button>
                            <Button disabled={busy} onclick={() => void sync(snippet.name)}>
                                Sync
                            </Button>
                            <Button disabled={busy} onclick={() => void remove(snippet.name)}>
                                {t('common.action.delete')}
                            </Button>
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
            <Button disabled={busy} onclick={() => void sync(pendingSync)}>
                {t('apply-to-nodes.apply')}
            </Button>
            <Button disabled={busy} onclick={() => (pendingSync = '')}>
                {t('apply-to-nodes.later')}
            </Button>
        </div>
    {/if}
    {#if error}
        <p role="alert">{error}</p>
    {:else if saved}
        <p class="save-status" role="status">{t('common.message.operation-completed')}</p>
    {/if}
</Overlay>

<style>
    .save-status {
        color: var(--mantine-color-teal-light-color);
        font-size: var(--mantine-font-size-sm);
    }

    .snippet-editor {
        height: 40dvh;
        margin-block: 0.75rem;
    }
    li {
        margin-block: 0.75rem;
    }
</style>
