<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import RouteLoading from '../RouteLoading.svelte'
    import Accordion from '../ui/Accordion.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { squadNodesQuery } from './queries'

    let { uuid, name, close }: { uuid: string; name: string; close: () => void } = $props()
    let t = $derived(locale.t)
    const nodes = createQuery(() => squadNodesQuery(uuid))
</script>

<Overlay
    title={t('view-user-modal.widget.accessible-nodes')}
    icon="TbServer"
    width={672}
    {close}
    subtitle={name}
    fullscreenOnMobile={false}
>
    {#if nodes.isPending}
        <RouteLoading compact />
    {:else if nodes.error}
        <p role="alert">{nodes.error.message}</p>
        <Button disabled={nodes.isFetching} onclick={() => void nodes.refetch()}>
            {t('common.action.try-again')}
        </Button>
    {:else if !nodes.data.accessibleNodes.length}
        <p>{t('common.message.no-data-available')}</p>
    {:else}
        <ul>
            {#each nodes.data.accessibleNodes as node (node.uuid)}
                <li>
                    <Accordion>
                        {#snippet heading()}
                            <span aria-hidden="true">{flag(node.countryCode)}</span>
                            {node.nodeName}
                            · {node.configProfileName} ({node.activeInbounds.length})
                        {/snippet}
                        <ul>
                            {#each node.activeInbounds as inbound}
                                <li>{inbound}</li>
                            {/each}
                        </ul>
                    </Accordion>
                </li>
            {/each}
        </ul>
    {/if}
</Overlay>

<style>
</style>
