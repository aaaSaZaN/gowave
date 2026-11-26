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
    import { reorderHandle } from '../ui/entity-order'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import { flag } from './analytics'
    import { type Node, trafficSummaryQuery } from './queries'

    let {
        node,
        status,
        height,
        mobile,
        plugin,
        onopen,
        onmeasure,
        reorder,
        onreordering,
        selection,
        actions
    }: {
        node: Node
        status: string
        height: number
        mobile: boolean
        plugin: string | undefined
        onopen: (event: MouseEvent) => void
        onmeasure?: (height: number) => void
        reorder?: ((source: string, target: string) => void) | undefined
        onreordering: (active: boolean) => void
        selection: Snippet
        actions: Snippet
    } = $props()

    let itemElement = $state<HTMLLIElement>()
    $effect(() => {
        const target = itemElement
        const report = onmeasure
        if (!target || !report || typeof ResizeObserver !== 'function') return
        const observer = new ResizeObserver(() => {
            const height = target.offsetHeight
            if (height > 0) report(height)
        })
        observer.observe(target, { box: 'border-box' })
        return () => observer.disconnect()
    })

    const trafficSummary = createQuery(() => trafficSummaryQuery)
    let lifetime = $derived(
        trafficSummary.data?.nodes.find((item) => item.uuid === node.uuid)?.lifetimeTrafficBytes
    )
    let menu = $state(false)
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
    let health = $derived.by(() => {
        if (node.isDisabled) return { icon: 'PiProhibitDuotone', color: 'gray-6' } as const
        if (rejected) return { icon: 'PiWarningCircle', color: 'orange-5' } as const
        if (online) return { icon: 'PiPulseDuotone', color: 'teal-6' } as const
        if (node.isConnecting) return { icon: 'PiCloudArrowUpDuotone', color: 'yellow-3' } as const
        return { icon: 'PiWarningCircle', color: 'red-3' } as const
    })
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
    let cpus = $derived(Math.max(1, node.system?.info.cpus ?? 1))
    let loadAvg = $derived(node.system?.stats.loadAvg)
    let network = $derived(node.system?.stats.interface)
    let rx = $derived(
        network ? prettySiRealtimeBytesUtil(network.rxBytesPerSec, true, true) || '0 B/s' : '—'
    )
    let tx = $derived(
        network ? prettySiRealtimeBytesUtil(network.txBytesPerSec, true, true) || '0 B/s' : '—'
    )
    let country = $derived(flag(node.countryCode))

    function loadColor(value: number): 'red' | 'yellow' | 'dimmed' {
        const ratio = value / cpus
        if (ratio > 1) return 'red'
        if (ratio > 0.7) return 'yellow'
        return 'dimmed'
    }

    function copyAddress(): void {
        void navigator.clipboard.writeText(node.address).then(
            () => {
                copied = true
                notifications.show({
                    title: locale.t('common.message.copied'),
                    message: node.address,
                    color: 'teal'
                })
            },
            () => (copied = false)
        )
    }
</script>

{#snippet meter(value: number, fill: string, label: string)}
    <div
        class="meter"
        role="progressbar"
        aria-label={label}
        aria-valuenow={value}
        style:--fill={fill}
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
    <Tooltip
        block
        disabled={lifetime === undefined}
        text={lifetime === undefined
            ? undefined
            : `${locale.t('node-stats.card.lifetime-traffic')}: ${prettifyBytesUtil(lifetime) || '0 B'}`}
    >
        <div class="traffic">
            <div class="traffic-values">
                <strong>{prettifyBytesUtil(node.trafficUsedBytes ?? 0) || '0 B'}</strong>
                <span>{limit ? prettifyBytesUtil(limit) : '∞'}</span>
            </div>
            {@render meter(
                percentage,
                node.isTrafficTrackingActive
                    ? `var(--mantine-color-${trafficTone}-6)`
                    : 'var(--mantine-color-teal-filled)',
                locale.t('use-nodes-table-widget.traffic-used')
            )}
        </div>
    </Tooltip>
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
    bind:this={itemElement}
    class="node-item"
    class:mobile
    style:min-height={`${height}px`}
    style:--row-height={`${height}px`}
    style:--node-tone={border}
    data-reorder-key={node.uuid}
>
    <article>
        <div class="node-main">
            <div class="identity">
                {#if dangling}
                    <span class="dangling-health" title="DANGLING">
                        <Icon name="TbAlertCircle" size={14} />DANGLING
                    </span>
                {:else}
                    <span
                        class="health"
                        role="img"
                        title={status}
                        aria-label={status}
                        style:--health-ink={`var(--mantine-color-${health.color})`}
                    >
                        <Icon name={health.icon} size={18} />
                    </span>
                    <span
                        class="users"
                        class:with-users={node.usersOnline > 0}
                        title={locale.t('nodes-quick-stats.widget.users-online')}
                    >
                        <Icon name="PiUsersDuotone" size={14} />{node.usersOnline}
                    </span>
                {/if}
                <a
                    href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}
                    onclick={onopen}
                >
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
            <Tooltip
                block={!mobile}
                disabled={memory === undefined}
                text={memory === undefined ? undefined : `${memory}%`}
            >
                <div class="memory">
                    <Icon name="PiMemoryDuotone" size={mobile ? 12 : 14} />
                    {@render meter(
                        Math.min(100, Math.max(0, memory ?? 0)),
                        `var(--mantine-color-${memoryTone}-6)`,
                        'RAM'
                    )}
                    <span>{memory === undefined ? '—' : `${memory}%`}</span>
                </div>
            </Tooltip>
            {#if !mobile}
                <Tooltip multiline width={250}>
                    {#snippet tip()}
                        {#if loadAvg}
                            <div class="la-tooltip">
                                <strong
                                    >{`Load Average (${cpus} ${cpus === 1 ? 'core' : 'cores'})`}</strong
                                >
                                <span
                                    >{`1 min: ${(loadAvg[0] ?? 0).toFixed(2)} (${Math.round(((loadAvg[0] ?? 0) / cpus) * 100)}%)`}</span
                                >
                                <span
                                    >{`5 min: ${(loadAvg[1] ?? 0).toFixed(2)} (${Math.round(((loadAvg[1] ?? 0) / cpus) * 100)}%)`}</span
                                >
                                <span
                                    >{`15 min: ${(loadAvg[2] ?? 0).toFixed(2)} (${Math.round(((loadAvg[2] ?? 0) / cpus) * 100)}%)`}</span
                                >
                                <span class="la-legend"
                                    >0–70% normal · 70–100% high · &gt;100% overloaded</span
                                >
                            </div>
                        {:else}
                            No data
                        {/if}
                    {/snippet}
                    <span class="la-metric">
                        <Icon name="PiCpuDuotone" size={12} />
                        {#if loadAvg}
                            <span class="la-values">
                                {#each loadAvg as value, index (index)}
                                    {#if index > 0}
                                        {' '}
                                    {/if}
                                    <span class={`tone-${loadColor(value)}`}
                                        >{value.toFixed(2)}</span
                                    >
                                {/each}
                            </span>
                        {:else}
                            <span>—</span>
                        {/if}
                    </span>
                </Tooltip>
            {/if}
            <span class="speed"
                ><Icon
                    name={mobile ? 'PiArrowUpDuotone' : 'PiArrowDownDuotone'}
                    size={mobile ? 10 : 12}
                    color={mobile ? 'var(--mantine-color-cyan-5)' : 'var(--mantine-color-teal-5)'}
                />{mobile ? tx : rx}</span
            >
            <span class="speed"
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
            use:reorderHandle={reorder
                ? {
                      drop: (target) => reorder?.(node.uuid, target),
                      onactive: (active) => {
                          if (active) menu = false
                          onreordering(active)
                      }
                  }
                : undefined}
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
        container-type: inline-size;
    }
    article {
        position: relative;
        min-height: calc(var(--row-height, 90px) - 10px);
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
        border-color: light-dark(var(--mantine-color-blue-3), var(--mantine-color-blue-5));
    }
    .node-main {
        display: grid;
        grid-template-columns: minmax(0, 5.5fr) minmax(0, 2.5fr) minmax(0, 2fr) minmax(0, 2fr);
        align-items: center;
        gap: clamp(8px, 1.2vw, 16px);
    }
    .traffic-values {
        gap: 6px;
        white-space: nowrap;
    }
    .traffic-values > * {
        white-space: nowrap;
    }
    .uptime span {
        white-space: nowrap;
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
        border: 1px solid var(--health-ink);
        border-radius: var(--mantine-radius-md);
        color: var(--health-ink);
    }
    .dangling-health {
        display: inline-flex;
        flex: none;
        align-items: center;
        gap: 4px;
        height: 26px;
        padding-inline: 10px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-red-light);
        color: var(--mantine-color-red-light-color);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
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
        border: 1px solid currentColor;
        color: light-dark(var(--mantine-color-gray-8), var(--mantine-color-gray-3));
    }
    .users.with-users {
        color: light-dark(var(--mantine-color-teal-8), var(--mantine-color-teal-4));
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
        height: 5px;
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
        flex-wrap: wrap;
        gap: 6px clamp(8px, 1.2vw, 16px);
        margin-block-start: 8px;
        white-space: nowrap;
    }
    .memory {
        min-width: 82px;
        max-width: 200px;
        flex: 1 1 82px;
        gap: 6px;
    }
    .memory > span {
        flex-shrink: 0;
        white-space: nowrap;
    }
    .memory .meter {
        min-width: 24px;
        height: 3px;
        flex: 1;
    }
    .la-metric,
    .speed {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        flex: 0 0 auto;
        white-space: nowrap;
    }
    .la-values {
        white-space: nowrap;
    }
    .tone-red {
        color: var(--mantine-color-red-text);
    }
    .tone-yellow {
        color: var(--mantine-color-yellow-text);
    }
    .tone-dimmed {
        color: var(--mantine-color-dimmed);
    }
    .la-tooltip {
        display: flex;
        flex-direction: column;
        gap: 0;
        font-size: var(--mantine-font-size-xs);
        line-height: 1.45;
    }
    .la-tooltip strong {
        font-weight: 600;
        color: #f0f6fc;
    }
    .la-legend {
        margin-top: 4px;
        color: var(--mantine-color-dimmed);
    }
    .versions {
        margin-inline-start: auto;
        flex: 0 1 auto;
        min-width: 0;
        max-width: 100%;
        gap: clamp(8px, 1vw, 16px);
    }
    .versions > span {
        flex: 0 1 auto;
        min-width: 0;
    }
    .versions .custom {
        color: var(--mantine-color-blue-4);
    }
    @container (max-width: 660px) {
        .vitals {
            gap: 6px 10px;
        }
        .memory {
            min-width: 72px;
            flex: 1 1 72px;
        }
        .versions {
            margin-inline-start: 0;
            flex-basis: 100%;
        }
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
