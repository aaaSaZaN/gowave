<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import { squadsQuery } from '../internal-squads/queries'
    import { flag } from '../nodes/analytics'
    import { p } from '../router'
    import Accordion from '../ui/Accordion.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { type Profile, profileInboundsQuery } from './queries'

    let {
        profile,
        mode,
        close
    }: { profile: Profile; mode: 'nodes' | 'inbounds'; close: () => void } = $props()
    let t = $derived(locale.t)
    let error = $state('')
    let copied = $state('')
    const inbounds = createQuery(() => ({
        ...profileInboundsQuery(profile.uuid),
        enabled: mode === 'inbounds'
    }))
    const squads = createQuery(() => ({
        ...squadsQuery,
        enabled: mode === 'inbounds'
    }))
    let squadsById = $derived(
        new Map(squads.data?.internalSquads.map((squad) => [squad.uuid, squad]))
    )

    async function copy(uuid: string): Promise<void> {
        error = ''
        copied = ''
        try {
            await navigator.clipboard.writeText(uuid)
            copied = uuid
        } catch (reason) {
            error = reason instanceof Error ? reason.message : t('common.message.error')
        }
    }
</script>

<Overlay
    title={mode === 'nodes'
        ? t('config-profile-card.shared.active-on-nodes')
        : t('config-profile-inbounds.drawer.widget.inbounds-with-active-squads')}
    icon="TbServer"
    width={768}
    {close}
    subtitle={profile.name}
    fullscreenOnMobile={false}
>
    {#if error}
        <p role="alert">{error}</p>
    {/if}
    {#if mode === 'nodes'}
        {#if profile.nodes.length}
            <ul>
                {#each profile.nodes as node (node.uuid)}
                    <li class="button-row">
                        <span aria-hidden="true">{flag(node.countryCode)}</span>
                        <a href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}
                            >{node.name}</a
                        >
                        <Button onclick={() => void copy(node.uuid)}>
                            {copied === node.uuid
                                ? t('common.message.copied')
                                : t('common.action.copy-uuid')}
                        </Button>
                    </li>
                {/each}
            </ul>
        {:else}
            <p>{t('active-nodes-list.modal.shared.this-profile-is-not-active-on-any-nodes')}</p>
        {/if}
    {:else if inbounds.isPending}
        <p role="status">{t('config-profile-inbounds.drawer.widget.fetching-inbounds')}</p>
    {:else if inbounds.error}
        <p role="alert">{inbounds.error.message}</p>
        <Button onclick={() => void inbounds.refetch()}>
            {t('common.action.try-again')}
        </Button>
    {:else if !inbounds.data.inbounds.length}
        <p>{t('common.message.no-data-available')}</p>
    {:else}
        {#if squads.error}
            <p role="alert">{squads.error.message}</p>
            <Button onclick={() => void squads.refetch()}>
                {t('common.action.try-again')}
            </Button>
        {/if}
        <ul>
            {#each inbounds.data.inbounds as inbound (inbound.uuid)}
                <li>
                    <Accordion>
                        {#snippet heading()}
                            {inbound.tag}
                            · {inbound.type} · {inbound.activeSquads.length}
                        {/snippet}
                        <Button onclick={() => void copy(inbound.uuid)}>
                            {copied === inbound.uuid
                                ? t('common.message.copied')
                                : t('common.action.copy-uuid')}
                        </Button>
                        {#if inbound.activeSquads.length}
                            <ul>
                                {#each inbound.activeSquads as uuid (uuid)}
                                    {@const squad = squadsById.get(uuid)}
                                    <li>
                                        <a
                                            href={p('/dashboard/open/internal-squad/:id', {
                                                params: { id: uuid }
                                            })}
                                            >{squad?.name ?? uuid}</a
                                        >
                                        {#if squad}
                                            · {t('common.field.inbounds')} ({squad.info
                                                .inboundsCount})
                                        {/if}
                                    </li>
                                {/each}
                            </ul>
                        {/if}
                        <pre dir="ltr">{JSON.stringify(inbound.rawInbound, null, 2)}</pre>
                    </Accordion>
                </li>
            {/each}
        </ul>
    {/if}
</Overlay>

<style>
    li {
        margin-block: 0.5rem;
    }

    pre {
        max-height: 40vh;
        overflow: auto;
    }
</style>
