<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import { squadNodesQuery } from './queries'

    let { uuid, name, close }: { uuid: string; name: string; close: () => void } = $props()
    let t = $derived(locale.t)
    const title = $props.id()
    let dialog: HTMLDialogElement
    const nodes = createQuery(() => squadNodesQuery(uuid))

    onMount(() => dialog.showModal())
</script>

<dialog oncancel={cancelDialog} bind:this={dialog} onclose={close} aria-labelledby={title}>
    <div class="button-row">
        <h2 id={title}>{t('view-user-modal.widget.accessible-nodes')} · {name}</h2>
        <button type="button" onclick={() => dismissDialog(dialog)}>
            {t('common.action.close')}
        </button>
    </div>
    {#if nodes.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if nodes.error}
        <p role="alert">{nodes.error.message}</p>
        <button type="button" disabled={nodes.isFetching} onclick={() => void nodes.refetch()}>
            {t('common.action.try-again')}
        </button>
    {:else if !nodes.data.accessibleNodes.length}
        <p>{t('common.message.no-data-available')}</p>
    {:else}
        <ul>
            {#each nodes.data.accessibleNodes as node (node.uuid)}
                <li>
                    <details>
                        <summary>
                            <span aria-hidden="true">{flag(node.countryCode)}</span>
                            {node.nodeName}
                            · {node.configProfileName} ({node.activeInbounds.length})
                        </summary>
                        <ul>
                            {#each node.activeInbounds as inbound}
                                <li>{inbound}</li>
                            {/each}
                        </ul>
                    </details>
                </li>
            {/each}
        </ul>
    {/if}
</dialog>

<style>
    dialog {
        width: min(42rem, calc(100vw - 2rem));
    }
</style>
