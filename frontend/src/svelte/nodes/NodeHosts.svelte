<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { profilesQuery } from '../config-profiles/queries'
    import HostDialog from '../hosts/HostDialog.svelte'
    import { type Host, hostsQuery } from '../hosts/queries'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { flag } from './analytics'
    import type { NodeDetail } from './queries'

    let {
        node,
        mode,
        close
    }: { node: NodeDetail; mode: 'linked' | 'inbounds'; close: () => void } = $props()
    const hosts = createQuery(() => hostsQuery)
    const profiles = createQuery(() => ({ ...profilesQuery, enabled: mode === 'inbounds' }))
    let t = $derived(locale.t)
    let editing = $state<string>()
    let activeProfile = $derived(
        profiles.data?.configProfiles.find(
            ({ uuid }) => uuid === node.configProfile.activeConfigProfileUuid
        )
    )
    let activeIds = $derived(new Set(node.configProfile.activeInbounds.map(({ uuid }) => uuid)))
    let orderedInbounds = $derived(
        activeProfile?.inbounds.toSorted(
            (a, b) => Number(activeIds.has(b.uuid)) - Number(activeIds.has(a.uuid))
        ) ?? []
    )
    let linked = $derived(hosts.data?.filter((host) => host.nodes.includes(node.uuid)) ?? [])
    let hostsByInbound = $derived(
        Map.groupBy(hosts.data ?? [], ({ inbound }) => inbound.configProfileInboundUuid)
    )
</script>

<Overlay
    drawer
    width={800}
    title={mode === 'linked'
        ? t('linked-hosts-drawer.widget.assigned-hosts')
        : t('common.field.inbounds-hosts')}
    icon={mode === 'linked' ? 'PiListChecks' : 'TbSitemap'}
    tone={mode === 'linked' ? 'teal' : 'cyan'}
    {close}
>
    {#if hosts.isPending || (mode === 'inbounds' && profiles.isPending)}
        <RouteLoading compact />
    {:else if hosts.error || (mode === 'inbounds' && profiles.error)}
        <p role="alert">{hosts.error?.message || profiles.error?.message}</p>
    {:else if mode === 'linked'}
        {#if linked.length}
            <ul class="linked">
                {#each linked as host (host.uuid)}
                    <li>{@render hostRow(host)}</li>
                {/each}
            </ul>
        {:else}
            <EmptyState
                icon="TbListCheck"
                label={t('linked-hosts-drawer.widget.no-hosts-assigned-to-this-node')}
            />
        {/if}
    {:else if activeProfile}
        <div class="topology">
            <details open>
                <summary>
                    <Icon name="TbChevronRight" size={16} />{flag(node.countryCode)}
                    {node.name}<small>{node.address}</small>
                </summary>
                <details class="branch" open>
                    <summary>
                        <Icon name="TbChevronRight" size={16} /><Icon name="XrayLogo" size={16} />
                        {activeProfile.name}<b>{orderedInbounds.length}</b>
                    </summary>
                    <div class="branch">
                        {#each orderedInbounds as inbound (inbound.uuid)}
                            {@const entries = hostsByInbound.get(inbound.uuid) ?? []}
                            <details class:inactive={!activeIds.has(inbound.uuid)}>
                                <summary>
                                    <Icon name="TbChevronRight" size={16} />
                                    <Icon name="TbTag" size={14} />{inbound.tag}<small
                                        >{inbound.type}</small
                                    ><b>{entries.length}</b>
                                </summary>
                                <ul class="branch">
                                    {#each entries as host (host.uuid)}
                                        <li>{@render hostRow(host)}</li>
                                    {/each}
                                </ul>
                            </details>
                        {/each}
                    </div>
                </details>
            </details>
        </div>
    {:else}
        <EmptyState
            icon="TbSitemap"
            label={t('node-inbounds-hosts-drawer.widget.no-active-config-profile')}
        />
    {/if}
</Overlay>

{#if editing}
    <HostDialog uuid={editing} close={() => (editing = undefined)} />
{/if}

{#snippet hostRow(host: Host)}
    <button
        class="mantine-UnstyledButton-root host"
        type="button"
        onclick={() => (editing = host.uuid)}
    >
        <Icon name="PiListChecks" size={18} />
        <span
            ><strong>{host.remark}</strong
            ><small>{host.address}{host.port ? `:${host.port}` : ''}</small></span
        >
        {#if host.nodes.includes(node.uuid)}
            <b>{t('node-inbounds-hosts-drawer.widget.linked')}</b>
        {/if}
    </button>
{/snippet}

<style>
    .branch {
        margin-inline-start: 8px;
        padding-inline-start: 16px;
        border-inline-start: 1px solid var(--gw-border);
    }
    ul {
        list-style: none;
        margin: 0;
        padding: 0;
    }
    .linked {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    summary,
    .host {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: 5px 6px;
        cursor: pointer;
        font-size: var(--mantine-font-size-sm);
    }
    summary {
        list-style: none;
    }
    summary::-webkit-details-marker {
        display: none;
    }
    details[open] > summary > :global(svg:first-child) {
        transform: rotate(90deg);
    }
    .host {
        width: 100%;
        color: inherit;
    }
    .topology,
    .linked .host {
        padding: var(--mantine-spacing-sm);
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .host:hover,
    summary:hover {
        background: var(--mantine-color-default-hover);
    }
    .host span,
    small {
        min-width: 0;
        overflow-wrap: anywhere;
    }
    small,
    .inactive {
        color: var(--mantine-color-dimmed);
    }
    .host small {
        display: block;
    }
    b {
        margin-inline-start: auto;
        padding-inline: 6px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-default-hover);
        font-size: var(--mantine-font-size-xs);
    }
</style>
