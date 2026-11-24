<script module lang="ts">
    import { GetNodeMetadataCommand } from '@remnawave/backend-contract'
    import { callCommand } from './commands'

    export function nodeMetadataQuery(uuid: string) {
        const command = GetNodeMetadataCommand
        return {
            queryKey: ['metadata', { kind: 'node', uuid }],
            queryFn: ({ signal }: { signal: AbortSignal }) => {
                const params = command.RequestParamsSchema.parse({ uuid })
                return callCommand({ ...command, TSQ_url: command.url(params.uuid) }, { signal })
            }
        }
    }
</script>

<script lang="ts">
    import { GetUserMetadataCommand } from '@remnawave/backend-contract'
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { locale } from './i18n.svelte'

    type Target = { kind: 'node'; uuid: string } | { kind: 'user'; userId: number }
    let { target, close }: { target: Target; close: () => void } = $props()
    let dialog: HTMLDialogElement
    const title = $props.id()
    let t = $derived(locale.t)
    const metadata = createQuery(() => {
        const current = target
        return {
            queryKey: ['metadata', { ...current }],
            queryFn: ({ signal }: { signal: AbortSignal }) => {
                if (current.kind === 'node')
                    return nodeMetadataQuery(current.uuid).queryFn({ signal })
                const command = GetUserMetadataCommand
                const { userId } = command.RequestParamsSchema.parse(current)
                return callCommand({ ...command, TSQ_url: command.url(String(userId)) }, { signal })
            }
        }
    })
    onMount(() => dialog.showModal())
</script>

<dialog bind:this={dialog} onclose={close} aria-labelledby={title}>
    <div class="button-row">
        <h2 id={title}>Metadata</h2>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
    {#if metadata.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if metadata.error}
        <p role="alert">{metadata.error.message}</p>
        <button
            type="button"
            disabled={metadata.isFetching}
            onclick={() => void metadata.refetch()}
        >
            {t('common.action.try-again')}
        </button>
    {:else}
        <pre>{JSON.stringify(metadata.data.metadata, null, 4)}</pre>
    {/if}
</dialog>

<style>
    dialog {
        width: min(48rem, calc(100vw - 2rem));
    }
    pre {
        overflow: auto;
        max-height: 55dvh;
    }
</style>
