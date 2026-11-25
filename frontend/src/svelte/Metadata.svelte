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
    import { locale } from './i18n.svelte'
    import RouteLoading from './RouteLoading.svelte'
    import Button from './ui/Button.svelte'
    import JsonTree from './ui/JsonTree.svelte'
    import Overlay from './ui/Overlay.svelte'

    type Target = { kind: 'node'; uuid: string } | { kind: 'user'; userId: number }
    let { target, close }: { target: Target; close: () => void } = $props()
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
</script>

<Overlay
    title="Metadata"
    icon="TbJson"
    tone="teal"
    class="metadata-modal"
    fullscreenOnMobile={false}
    {close}
>
    {#if metadata.isPending}
        <RouteLoading compact />
    {:else if metadata.error}
        <p role="alert">{metadata.error.message}</p>
        <Button disabled={metadata.isFetching} onclick={() => void metadata.refetch()}>
            {t('common.action.try-again')}
        </Button>
    {:else}
        <div class="metadata-content">
            <JsonTree value={metadata.data.metadata} indent={4} expandEmptyRoot />
        </div>
    {/if}
</Overlay>

<style>
    :global(.panel-overlay.metadata-modal.modal) {
        width: fit-content;
        min-width: min(276px, calc(100% - 32px));
        max-width: min(640px, calc(100% - 32px));
    }
    .metadata-content {
        overflow: auto;
        max-height: 65dvh;
    }
</style>
