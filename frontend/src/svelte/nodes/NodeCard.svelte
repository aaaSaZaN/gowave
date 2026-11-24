<script lang="ts">
    import {
        prettifyBytesUtil,
        prettySiRealtimeBytesUtil
    } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { isCustomXrayVersion } from '@shared/utils/misc/node-versions'
    import { getNodeResetDaysUtil, getXrayUptimeUtil } from '@shared/utils/time-utils'
    import { createQuery } from '@tanstack/svelte-query'
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import BrandIcon from '../ui/BrandIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import { flag } from './analytics'
    import { type Node, trafficSummaryQuery } from './queries'

    let {
        node,
        status,
        height,
        mobile,
        plugin,
        reorder,
        selection,
        actions
    }: {
        node: Node
        status: string
        height: number
        mobile: boolean
        plugin: string | undefined
        reorder?: ((source: string, target: string) => void) | undefined
        selection: Snippet
        actions: Snippet
    } = $props()

    const trafficSummary = createQuery(() => trafficSummaryQuery)
    let lifetime = $derived(
        trafficSummary.data?.nodes.find((item) => item.uuid === node.uuid)?.lifetimeTrafficBytes
    )
    const reorderType = 'application/x-gowave-node'
    let menu = $state(false)
    let dragging = $state(false)
    let copied = $state(false)
    const menuId = $props.id()
    let hasError = $derived(Boolean(node.lastStatusMessage && node.lastStatusMessage !== 'OK'))
    let online = $derived(
        node.isConnected && !hasError && (node.xrayUptime ?? 1) > 0 && !node.isDisabled
    )
    let rejected = $derived(node.isConnected && !node.isDisabled && hasError)
    let border = $derived(
        node.isDisabled
            ? '107, 114, 128'
            : rejected
              ? '249, 115, 22'
              : online
                ? '45, 212, 191'
                : node.isConnecting
                  ? '245, 158, 11'
                  : '239, 68, 68'
    )
    let dangling = $derived(
        !node.configProfile.activeConfigProfileUuid || !node.configProfile.activeInbounds.length
    )
    let limit = $derived(node.isTrafficTrackingActive ? node.trafficLimitBytes : 0)
    let percentage = $derived(
        limit ? Math.min(100, Math.floor(((node.trafficUsedBytes ?? 0) * 100) / limit)) : 100
    )
    let trafficTone = $derived(
        !limit ? 'teal' : percentage > 95 ? 'red' : percentage > 80 ? 'yellow' : 'teal'
    )
    let memory = $derived(
        node.system?.info.memoryTotal
            ? Math.round((node.system.stats.memoryUsed * 100) / node.system.info.memoryTotal)
            : undefined
    )
    let memoryTone = $derived((memory ?? 0) > 90 ? 'red' : (memory ?? 0) > 70 ? 'yellow' : 'teal')
    let load = $derived(
        node.system?.stats.loadAvg?.map((value) => value.toFixed(2)).join(' ') ?? '—'
    )
    let network = $derived(node.system?.stats.interface)
    let rx = $derived(
        network ? prettySiRealtimeBytesUtil(network.rxBytesPerSec, true, true) || '0 B/s' : '—'
    )
    let tx = $derived(
        network ? prettySiRealtimeBytesUtil(network.txBytesPerSec, true, true) || '0 B/s' : '—'
    )
    let country = $derived(flag(node.countryCode))

    function copyAddress(): void {
        void navigator.clipboard.writeText(node.address).then(
            () => (copied = true),
            () => (copied = false)
        )
    }

    function dragstart(event: DragEvent): void {
        if (!event.dataTransfer || !reorder) return
        event.dataTransfer.effectAllowed = 'move'
        event.dataTransfer.setData(reorderType, node.uuid)
        dragging = true
        menu = false
    }

    function dragover(event: DragEvent): void {
        if (!reorder || !event.dataTransfer?.types.includes(reorderType)) return
        event.preventDefault()
        event.dataTransfer.dropEffect = 'move'
    }

    function drop(event: DragEvent): void {
        const source = event.dataTransfer?.getData(reorderType)
        if (!source || !reorder) return
        event.preventDefault()
        reorder(source, node.uuid)
    }
</script>

{#snippet meter(value: number, tone: string, label: string)}
    <div
        class="meter"
        role="progressbar"
        aria-label={label}
        aria-valuenow={value}
        style:--fill={`var(--mantine-color-${tone}-6)`}
    >
        <span style:width={`${value}%`}></span>
    </div>
{/snippet}

{#snippet provider()}
    {#if node.provider}
        <span class="provider" title={node.provider.name}>
            <span>{node.provider.name.slice(0, 2).toUpperCase()}</span>
            {node.provider.name}
        </span>
    {/if}
{/snippet}

{#snippet traffic()}
    <div
        class="traffic"
        title={lifetime === undefined
            ? undefined
            : `${locale.t('node-stats.card.lifetime-traffic')}: ${prettifyBytesUtil(lifetime) || '0 B'}`}
    >
        <div class="traffic-values">
            <strong>{prettifyBytesUtil(node.trafficUsedBytes ?? 0) || '0 B'}</strong>
            <span>{limit ? prettifyBytesUtil(limit) : '∞'}</span>
        </div>
        {@render meter(percentage, trafficTone, locale.t('use-nodes-table-widget.traffic-used'))}
    </div>
{/snippet}

{#snippet uptime()}
    <div class="uptime">
        <span>
            {#if node.isTrafficTrackingActive}
                <Icon name="PiArrowsCounterClockwise" size={14} />
                {getNodeResetDaysUtil(node.trafficResetDay ?? 1)}
            {/if}
        </span>
        {#if online || mobile}
            <span class:online-uptime={online}>
                <Icon name="XrayLogo" size={mobile ? 12 : 14} />
                {online ? getXrayUptimeUtil(node.xrayUptime) : 'offline'}
            </span>
        {/if}
    </div>
{/snippet}

<li
    class:mobile
    class:dragging
    style:height={`${height}px`}
    style:--node-tone={border}
    ondragover={dragover}
    ondrop={drop}
>
    <article>
        <div class="node-main">
            <div class="identity">
                {#if dangling}
                    <span class="health dangling-health" title="DANGLING">
                        <Icon name="TbAlertCircle" size={18} />
                    </span>
                {:else}
                    <span
                        class="health"
                        class:online
                        class:disabled={node.isDisabled}
                        role="img"
                        title={status}
                        aria-label={status}
                        ><Icon
                            name={node.isDisabled
                                ? 'PiProhibitDuotone'
                                : online
                                  ? 'PiPulseDuotone'
                                  : 'TbAlertCircle'}
                            size={18}
                        /></span
                    >
                    <span
                        class="users"
                        class:with-users={node.usersOnline > 0}
                        title={locale.t('nodes-quick-stats.widget.users-online')}
                    >
                        <Icon name="PiUsersDuotone" size={14} />{node.usersOnline}
                    </span>
                {/if}
                <a href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}>
                    {#if country}
                        <span class="flag">{country}</span>
                    {/if}
                    <span class="name">{node.name}</span>
                </a>
                {#if !mobile}
                    {#if hasError}
                        <span
                            class="error-badge"
                            class:rejected
                            title={node.lastStatusMessage ?? undefined}
                        >
                            {locale.t(
                                rejected
                                    ? 'node-status-badge.widget.config-not-applied'
                                    : 'node-status-badge.widget.error'
                            )}
                        </span>
                    {/if}
                    {@render provider()}
                {/if}
            </div>
            {#if mobile}
                <div class="mobile-provider">{@render provider()}</div>
            {:else}
                <button
                    type="button"
                    class="mantine-UnstyledButton-root address"
                    title={copied
                        ? locale.t('common.message.copied')
                        : locale.t('common.action.copy')}
                    onclick={copyAddress}
                >
                    <Icon name="PiGlobeSimple" size={14} />
                    <span>{node.address}</span>
                </button>
            {/if}
            {@render traffic()}
            {@render uptime()}
        </div>
        <div class="vitals">
            <div class="memory" title={memory === undefined ? undefined : `${memory}%`}>
                <Icon name="PiMemoryDuotone" size={mobile ? 12 : 14} />
                {@render meter(Math.min(100, Math.max(0, memory ?? 0)), memoryTone, 'RAM')}
                <span>{memory === undefined ? '—' : `${memory}%`}</span>
            </div>
            {#if !mobile}
                <span><Icon name="PiCpuDuotone" size={12} />{load}</span>
            {/if}
            <span
                ><Icon
                    name={mobile ? 'PiArrowUpDuotone' : 'PiArrowDownDuotone'}
                    size={mobile ? 10 : 12}
                    color={mobile ? 'var(--mantine-color-cyan-5)' : 'var(--mantine-color-teal-5)'}
                />{mobile ? tx : rx}</span
            >
            <span
                ><Icon
                    name={mobile ? 'PiArrowDownDuotone' : 'PiArrowUpDuotone'}
                    size={mobile ? 10 : 12}
                    color={mobile ? 'var(--mantine-color-teal-5)' : 'var(--mantine-color-cyan-5)'}
                />{mobile ? rx : tx}</span
            >
            {#if !mobile}
                <div class="versions">
                    {#if plugin}
                        <span title={plugin}><Icon name="TbPackage" size={12} />{plugin}</span>
                    {/if}
                    <span class:custom={isCustomXrayVersion(node.versions)}
                        ><Icon name="XrayLogo" size={12} />{node.versions?.xray ?? '—'}</span
                    >
                    <span><BrandIcon size={12} />{node.versions?.node ?? '—'}</span>
                </div>
            {/if}
        </div>
        <button
            type="button"
            class="mantine-UnstyledButton-root handle"
            draggable={!!reorder}
            ondragstart={dragstart}
            ondragend={() => (dragging = false)}
            aria-label={locale.t('common.action.actions')}
            aria-expanded={menu}
            aria-controls={menu ? menuId : undefined}
            onclick={() => (menu = !menu)}
        >
            <Icon name="PiDotsSixVertical" size={24} />
        </button>
        {#if menu}
            <div id={menuId} class="row-actions">
                <div class="selection">{@render selection()}{locale.t('common.action.select')}</div>
                {@render actions()}
            </div>
        {/if}
    </article>
</li>

<style>
    li {
        padding: 0 16px 10px;
    }
    article {
        position: relative;
        height: 100%;
        min-width: 0;
        padding: 12px 48px 12px 16px;
        border: 1px solid rgba(var(--node-tone), 0.3);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(
            135deg,
            rgba(var(--node-tone), 0.15),
            var(--mantine-color-dark-7)
        );
    }
    article:hover {
        border-color: color-mix(in srgb, rgb(var(--node-tone)) 65%, var(--gw-border));
    }
    .dragging article {
        opacity: 0.45;
    }
    .node-main {
        display: grid;
        grid-template-columns: minmax(0, 5.5fr) minmax(0, 2.5fr) minmax(0, 2fr) minmax(0, 2fr);
        align-items: center;
        gap: var(--mantine-spacing-md);
    }
    .identity,
    .identity a,
    .address,
    .uptime,
    .uptime span,
    .vitals,
    .vitals > span,
    .versions,
    .versions span,
    .memory {
        display: flex;
        align-items: center;
        min-width: 0;
        gap: 4px;
    }
    .identity {
        gap: var(--mantine-spacing-sm);
    }
    .identity a {
        color: inherit;
        text-decoration: none;
        font-weight: 600;
        gap: var(--mantine-spacing-xs);
    }
    .identity a::after {
        position: absolute;
        inset: 0;
        content: "";
    }
    .flag {
        font-size: 1.6em;
        line-height: 1;
    }
    .name,
    .address span,
    .versions span,
    .provider {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .health {
        display: grid;
        flex: 0 0 26px;
        width: 26px;
        height: 26px;
        place-items: center;
        border: 1px solid color-mix(in srgb, rgb(var(--node-tone)) 52%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, rgb(var(--node-tone)) 12%, transparent);
        color: rgb(var(--node-tone));
    }
    .health.online {
        color: var(--mantine-color-teal-4);
        border-color: color-mix(in srgb, var(--mantine-color-teal-4) 48%, transparent);
        background: color-mix(in srgb, var(--mantine-color-teal-6) 14%, transparent);
    }
    .health.disabled {
        color: var(--mantine-color-gray-5);
    }
    .health.dangling-health {
        color: var(--mantine-color-red-4);
        border-color: color-mix(in srgb, var(--mantine-color-red-5) 50%, transparent);
        background: var(--mantine-color-red-light);
    }
    .users,
    .provider,
    .error-badge {
        display: inline-flex;
        flex: none;
        align-items: center;
        justify-content: center;
        gap: 4px;
        height: 26px;
        padding-inline: 12px;
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
        font-weight: 700;
    }
    .error-badge {
        color: var(--mantine-color-red-light-color);
        background: var(--mantine-color-red-light);
    }
    .error-badge {
        height: 18px;
        padding-inline: 7px;
        font-size: 10px;
        text-transform: uppercase;
    }
    .error-badge.rejected {
        color: var(--mantine-color-orange-light-color);
        background: var(--mantine-color-orange-light);
    }
    .users {
        min-width: 7ch;
        padding-inline: 8px;
        border: 1px solid var(--mantine-color-gray-6);
        color: var(--mantine-color-gray-light-color);
    }
    .users.with-users {
        border-color: var(--mantine-color-teal-6);
        color: var(--mantine-color-teal-light-color);
    }
    .provider {
        max-width: 20ch;
        color: var(--mantine-color-gray-light-color);
        background: var(--mantine-color-gray-light);
        text-transform: uppercase;
    }
    .provider > span {
        border-radius: 4px;
        background: var(--mantine-color-dark-4);
        font-size: 10px;
    }
    .address,
    .vitals {
        color: var(--mantine-color-dimmed);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
    }
    .address {
        position: relative;
        gap: var(--mantine-spacing-xs);
        font-size: var(--mantine-font-size-sm);
        cursor: copy;
    }
    .address:hover {
        color: var(--mantine-color-blue-6);
    }
    .traffic {
        display: grid;
        gap: 4px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .traffic-values,
    .uptime {
        display: flex;
        align-items: center;
        justify-content: space-between;
    }
    .traffic strong {
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }
    .meter {
        min-width: 0;
        position: relative;
        height: 6px;
        border-radius: var(--mantine-radius-sm);
        overflow: hidden;
        background: var(--mantine-color-dark-4);
    }
    .meter > span {
        display: block;
        height: 100%;
        background: var(--fill);
    }
    .uptime {
        min-width: 0;
        gap: var(--mantine-spacing-xs);
        font-size: var(--mantine-font-size-sm);
        color: var(--mantine-color-dimmed);
    }
    .online-uptime {
        color: var(--mantine-color-teal-4);
        font-weight: 600;
    }
    .vitals {
        gap: var(--mantine-spacing-md);
        margin-block-start: var(--mantine-spacing-xs);
    }
    .memory {
        max-width: 200px;
        flex: 1 1 0;
        gap: 6px;
    }
    .memory .meter {
        height: 3px;
        flex: 1;
    }
    .versions {
        margin-inline-start: auto;
        gap: var(--mantine-spacing-md);
    }
    .versions .custom {
        color: var(--mantine-color-blue-4);
    }
    .handle {
        position: absolute;
        z-index: 1;
        inset-inline-end: 6px;
        inset-block-start: 50%;
        transform: translateY(-50%);
        display: grid;
        place-items: center;
        padding: 8px;
        border-radius: 4px;
        color: var(--mantine-color-bright);
    }
    .handle[draggable="true"] {
        cursor: grab;
    }
    .handle:hover {
        background: var(--mantine-color-default-hover);
    }
    .row-actions {
        position: absolute;
        inset-inline-end: 4px;
        inset-block-start: calc(50% + 20px);
        z-index: 2;
        padding: var(--mantine-spacing-xs);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-body);
        box-shadow: var(--mantine-shadow-md);
    }
    .selection {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: 4px;
        font-size: var(--mantine-font-size-sm);
    }
    .mobile {
        padding: 0 0 6px;
    }
    .mobile article {
        padding: 10px 36px 10px 10px;
    }
    .mobile .node-main {
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-xs);
    }
    .mobile .identity {
        font-size: var(--mantine-font-size-sm);
    }
    .mobile .flag {
        font-size: 1.5em;
    }
    .mobile-provider {
        min-height: 26px;
    }
    .mobile .traffic {
        gap: var(--mantine-spacing-xs);
    }
    .mobile .meter {
        height: 3px;
    }
    .mobile .uptime {
        font-size: var(--mantine-font-size-xs);
    }
    .mobile .memory {
        max-width: 160px;
    }
    .mobile .handle {
        padding: 4px;
    }
</style>
