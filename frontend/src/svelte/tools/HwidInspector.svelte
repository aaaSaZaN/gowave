<script lang="ts">
    import ColorHash from '@shared/lib/color'
    import { formatInt, formatPercentage } from '@shared/utils/misc/number'
    import { createQuery } from '@tanstack/svelte-query'
    import { MediaQuery } from 'svelte/reactivity'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import ChartPlaceholder from '../ui/ChartPlaceholder.svelte'
    import Icon from '../ui/Icon.svelte'
    import Leaderboard from '../ui/Leaderboard.svelte'
    import MetricCard from '../ui/MetricCard.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import MobileTableNotice from '../users/MobileTableNotice.svelte'
    import type { Column } from './columns'
    import { devicePlatforms } from './device-platforms'
    import InspectorTable from './InspectorTable.svelte'
    import { type Device, deviceLeaders, devicesStats, loadDevices } from './queries'

    import { hwidStorage } from './table-state'

    const smallScreen = new MediaQuery('(max-width: 500px)')
    let t = $derived(locale.t)
    let topPage = $state(0)
    let topSize = $state(5)
    const stats = createQuery(() => devicesStats)
    const leaders = createQuery(() => deviceLeaders(topPage * topSize, topSize))
    const lastPage = $derived(Math.max(0, Math.ceil((leaders.data?.total ?? 0) / topSize) - 1))
    const topPageItems = $derived.by(() => {
        const pages = lastPage + 1
        if (pages <= 7) return Array.from({ length: pages }, (_, index) => index)
        const start = Math.max(1, Math.min(topPage - 1, pages - 4))
        const end = Math.min(pages - 2, start + 2)
        return [
            0,
            ...(start > 1 ? ['…'] : []),
            ...Array.from({ length: end - start + 1 }, (_, index) => start + index),
            ...(end < pages - 2 ? ['…'] : []),
            pages - 1
        ]
    })
    const appColors = new ColorHash({ lightness: [0.65, 0.65, 0.65] })
    const platforms = $derived(devicePlatforms(stats.data?.byPlatform ?? []))
    const totalDevices = $derived(platforms.reduce((total, platform) => total + platform.count, 0))
    let activePlatform = $state<(typeof platforms)[number]>()
    $effect(() => {
        if (leaders.data && topPage > lastPage) topPage = lastPage
    })
    let columns: Column<Device>[] = $derived([
        { key: 'userId', label: 'User ID', filter: 'number', width: 130 },
        {
            key: 'requestIp',
            label: t('use-srh-inspector-table-columns.request-ip'),
            filter: 'text'
        },
        { key: 'hwid', label: 'HWID', filter: 'text', width: 200 },
        { key: 'platform', label: t('common.field.platform'), filter: 'text' },
        { key: 'osVersion', label: t('common.field.os-version'), width: 100 },
        { key: 'deviceModel', label: t('use-hwid-inspector-table-columns.device-model') },
        { key: 'userAgent', label: t('common.field.user-agent'), width: 500 },
        { key: 'createdAt', label: t('common.field.created-at'), width: 250 },
        { key: 'updatedAt', label: t('common.field.updated-at'), width: 250 }
    ])

    function count(number: number): string {
        return formatInt(number, { thousandSeparator: ' ' })
    }

    function deviceKey(device: Device): string {
        return JSON.stringify([device.userId, device.hwid])
    }
</script>

<svelte:head><title>{t('constants.hwid-inspector')} | Gowave</title></svelte:head>
<main>
    {#if smallScreen.current}
        <MobileTableNotice />
    {/if}
    {#if stats.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {/if}
    {#if stats.error}
        <p role="alert">{stats.error.message}</p>
    {/if}
    <div class="metrics">
        <MetricCard
            label={t('hwid-inspector-metrics.widget.total-unique-devices')}
            value={stats.data?.stats.totalUniqueDevices ?? 0}
            icon="PiDevicesDuotone"
            color="blue"
        />
        <MetricCard
            label={t('hwid-inspector-metrics.widget.total-hwid-devices')}
            value={stats.data?.stats.totalHwidDevices ?? 0}
            icon="PiDeviceMobileDuotone"
            color="teal"
        />
        <MetricCard
            label={t('hwid-inspector-metrics.widget.avg-devices-per-user')}
            value={stats.data?.stats.averageHwidDevicesPerUser ?? 0}
            icon="PiCalculatorDuotone"
            color="orange"
        />
    </div>
    <div class="panels">
        <section class="platform-panel" aria-labelledby="hwid-platforms">
            <h2 id="hwid-platforms">
                <span class="icon-box"><Icon name="TbChartArcs3" size={20} /></span>
                {t('hwid-inspector-metrics.widget.platform-distribution')}
            </h2>
            {#if stats.data}
                {#if platforms.length}
                    <div class="platform-grid">
                        {#each platforms as platform (platform)}
                            {@const share = (platform.count / totalDevices) * 100}
                            <button
                                type="button"
                                class="platform"
                                aria-haspopup="dialog"
                                style:--platform-color={platform.color}
                                onclick={() => (activePlatform = platform)}
                            >
                                <span class="platform-heading">
                                    <span class="platform-dot" aria-hidden="true"></span>
                                    <span class="platform-name" title={platform.name}
                                        >{platform.name}</span
                                    >
                                    <span class="platform-totals">
                                        <strong>{count(platform.count)}</strong>
                                        <small>{formatPercentage(share, { precision: 1 })}</small>
                                    </span>
                                </span>
                                <span class="platform-track" aria-hidden="true">
                                    <span style:width={`${share}%`}></span>
                                </span>
                                {#if platform.preview.length}
                                    <span class="platform-apps">
                                        {#each platform.preview as app (app)}
                                            <span class="platform-app">
                                                <span class="platform-name" title={app.name}
                                                    >{app.name}</span
                                                >
                                                <strong>{count(app.count)}</strong>
                                            </span>
                                        {/each}
                                    </span>
                                {/if}
                            </button>
                        {/each}
                    </div>
                {:else}
                    <ChartPlaceholder
                        icon="PiChartPieDuotone"
                        iconSize={48}
                        height={280}
                        label={t('hwid-inspector-metrics.widget.no-platform-data')}
                    />
                {/if}
            {/if}
        </section>
        <section class="leaderboard" aria-label="Top users by devices">
            {#if leaders.isPending}
                <p role="status">{t('common.message.loading')}</p>
            {/if}
            {#if leaders.error}
                <p role="alert">{leaders.error.message}</p>
            {/if}
            {#if leaders.data}
                <div class="leaders-scroll">
                    <table aria-label="Top users by devices">
                        <thead>
                            <tr>
                                <th aria-label={t('common.action.open')}>
                                    <Icon name="TbUser" size={18} />
                                </th>
                                <th>{t('common.field.username')}</th>
                                <th>ID</th>
                                <th
                                    aria-label={t(
                                        'hwid-inspector-metrics.widget.total-hwid-devices'
                                    )}
                                >
                                    <Icon name="TbSum" size={18} />
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each leaders.data.users as user (user.id)}
                                <tr>
                                    <td>
                                        <a
                                            aria-label={t('common.action.open')}
                                            class="view-user"
                                            href={p('/dashboard/open/user/:id', {
                                                params: { id: String(user.id) }
                                            })}
                                            ><Icon name="PiUserCircle" size={24} /></a
                                        >
                                    </td>
                                    <td>{user.username}</td>
                                    <td>{user.id}</td>
                                    <td>
                                        <strong class="device-count"
                                            >{count(user.devicesCount)}</strong
                                        >
                                    </td>
                                </tr>
                            {/each}
                        </tbody>
                    </table>
                    {#if !leaders.data.users.length}
                        <div class="no-records">
                            <span><Icon name="TbDatabaseOff" size={24} /></span>
                            No records
                        </div>
                    {/if}
                </div>
                {#if leaders.data.total > 0}
                    <div class="button-row pages">
                        <span class="page-range">
                            {topPage * topSize + 1}
                            - {Math.min((topPage + 1) * topSize, leaders.data.total)}
                            / {leaders.data.total}
                        </span>
                        <ActionIcon
                            icon="TbRefresh"
                            label={t('common.action.refresh')}
                            tone="cyan"
                            size={28}
                            iconSize={20}
                            disabled={leaders.isFetching}
                            onclick={() => void leaders.refetch()}
                        />
                        <select
                            aria-label="Users per page"
                            bind:value={topSize}
                            onchange={() => (topPage = 0)}
                        >
                            {#each [5, 10, 25, 50, 100] as size (size)}
                                <option value={size}>{size}</option>
                            {/each}
                        </select>
                        <div class="page-buttons">
                            <ActionIcon
                                icon="PiCaretLeft"
                                label="Previous"
                                size={26}
                                iconSize={16}
                                subtle
                                disabled={topPage === 0}
                                onclick={() => (topPage -= 1)}
                            />
                            {#each topPageItems as pageIndex (`${pageIndex}`)}
                                {#if typeof pageIndex === 'number'}
                                    <button
                                        type="button"
                                        class="page-number"
                                        class:current={topPage === pageIndex}
                                        aria-label={`Page ${pageIndex + 1}`}
                                        aria-current={topPage === pageIndex ? 'page' : undefined}
                                        onclick={() => (topPage = pageIndex)}
                                    >
                                        {pageIndex + 1}
                                    </button>
                                {:else}
                                    <span class="page-ellipsis" aria-hidden="true"
                                        >{pageIndex}</span
                                    >
                                {/if}
                            {/each}
                            <ActionIcon
                                icon="PiCaretRight"
                                label={t('common.action.next')}
                                size={26}
                                iconSize={16}
                                subtle
                                disabled={topPage >= lastPage}
                                onclick={() => (topPage += 1)}
                            />
                        </div>
                    </div>
                {/if}
            {/if}
        </section>
    </div>
    <InspectorTable
        icon="TbDeviceAnalytics"
        title={t('hwid-inspector-table.widget.hwid-devices-list')}
        setup={hwidStorage}
        {columns}
        rowKey={deviceKey}
        ipAddress={(device) => device.requestIp}
        load={loadDevices}
    />
</main>

{#if activePlatform}
    <Overlay
        title={activePlatform.name}
        subtitle={count(activePlatform.count)}
        icon="PiAppWindowDuotone"
        close={() => (activePlatform = undefined)}
    >
        <Leaderboard
            items={activePlatform.allApps.map(({ name, count }) => ({
                name,
                total: count,
                color: appColors.hex(name)
            }))}
            format={count}
            maxHeight={420}
            emptyText={t('hwid-inspector-metrics.widget.no-app-data')}
        />
    </Overlay>
{/if}

<style>
    main,
    .metrics,
    .panels {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-md);
    }
    @media (min-width: 40em) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 80em) {
        .metrics {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }
    }
    .platform-panel {
        padding: var(--mantine-spacing-lg);
        background: rgb(255 255 255 / 2%);
        border: 1px solid rgb(255 255 255 / 8%);
        border-radius: var(--mantine-radius-md);
    }
    .platform-panel :global(.placeholder svg) {
        color: var(--mantine-color-gray-6);
    }
    h2 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        margin: 0 0 var(--mantine-spacing-lg);
        font-size: var(--mantine-font-size-lg);
        font-weight: 600;
    }
    .icon-box {
        display: grid;
        place-items: center;
        width: 34px;
        height: 34px;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-cyan-4);
        background: linear-gradient(135deg, rgb(21 170 191 / 15%), rgb(16 152 173 / 10%));
        border: 1px solid rgb(21 170 191 / 30%);
    }
    .platform-grid {
        display: grid;
        gap: var(--mantine-spacing-sm);
    }
    @media (min-width: 48em) {
        .platform-grid {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 80em) {
        .platform-grid {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }
    }
    .platform {
        display: block;
        min-width: 0;
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: rgb(0 0 0 / 10%);
        color: var(--mantine-color-text);
        text-align: start;
    }
    .platform:hover {
        background: rgba(var(--gw-fg-rgb), 0.03);
    }
    .platform-heading,
    .platform-apps,
    .platform-app {
        display: flex;
        align-items: center;
    }
    .platform-heading {
        gap: var(--mantine-spacing-xs);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        font-weight: 600;
    }
    .platform-heading strong {
        font-weight: 700;
    }
    .platform-totals {
        display: flex;
        flex: none;
        align-items: baseline;
        gap: 6px;
    }
    .platform-dot {
        width: 12px;
        height: 12px;
        border-radius: 3px;
        background: var(--platform-color);
        flex: none;
    }
    .platform-name {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .platform-track,
    .platform-track span {
        display: block;
        height: 6px;
        border-radius: 999px;
    }
    .platform-track {
        margin-block: 8px 10px;
        background: light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-4));
    }
    .platform-track span {
        background: var(--platform-color);
    }
    .platform-apps {
        flex-wrap: wrap;
        gap: var(--mantine-spacing-md);
    }
    .platform-apps,
    .platform-heading small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 400;
        line-height: var(--mantine-line-height-xs);
    }
    .platform-app {
        gap: 6px;
        min-width: 0;
    }
    .platform-app strong {
        opacity: 0.6;
    }
    .leaderboard {
        padding: 0;
        height: 350px;
        display: flex;
        flex-direction: column;
        justify-content: space-between;
        overflow: hidden;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-body);
    }
    .leaders-scroll {
        display: flex;
        flex: 1;
        flex-direction: column;
        min-height: 0;
        overflow: auto;
    }
    .no-records {
        display: flex;
        flex: 1;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .no-records span {
        display: grid;
        place-items: center;
        width: 44px;
        height: 44px;
        border-radius: 50%;
        background: var(--mantine-color-gray-light);
    }
    table {
        width: 100%;
        border-collapse: collapse;
        font-size: var(--mantine-font-size-sm);
    }
    td,
    th {
        border: 1px solid var(--gw-border);
        padding: 7px 10px;
        text-align: start;
    }
    th:first-child {
        width: 0%;
        min-width: 43px;
        color: var(--mantine-color-dimmed);
    }
    th:nth-child(2) {
        width: 54.37%;
    }
    th:nth-child(3) {
        width: 21%;
    }
    th {
        position: sticky;
        top: 0;
        height: 39.7px;
        border-block-start: 0;
        background: var(--mantine-color-body);
    }
    td:first-child {
        padding-inline: 10px;
    }
    tbody tr:nth-child(odd) {
        background: light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6));
    }
    .device-count {
        color: var(--mantine-color-bright);
        font-weight: 600;
    }
    .view-user {
        display: grid;
        place-items: center;
        width: 36px;
        height: 36px;
        border: 1px solid rgb(21 170 191 / 30%);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(135deg, rgb(21 170 191 / 15%), rgb(16 152 173 / 10%));
        color: var(--mantine-color-cyan-4);
    }
    td:first-child,
    th:first-child {
        border-inline-start: 0;
    }
    td:last-child,
    th:last-child {
        border-inline-end: 0;
    }
    .pages {
        justify-content: flex-end;
        min-height: 49px;
        padding: 6px;
        padding-inline-start: 10px;
        padding-inline-end: 10px;
        align-items: center;
        border-block-start: 1px solid var(--gw-border);
        font-size: var(--mantine-font-size-sm);
    }
    .pages > :global(button) {
        margin-inline-end: 12px;
    }
    .page-range {
        margin-inline-end: auto;
        color: var(--mantine-color-text);
        line-height: 1.45;
    }
    .pages select {
        width: 48px;
        min-width: 48px;
        min-height: 28px;
        padding-block: 0;
        padding-inline: 4px;
    }
    .page-buttons {
        display: flex;
        align-items: center;
        gap: 8px;
    }
    .page-number {
        display: grid;
        width: 26px;
        height: 26px;
        min-height: 26px;
        place-items: center;
        padding: 0;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-xs);
        background: var(--mantine-color-default);
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-xs);
        line-height: 14px;
        cursor: pointer;
    }
    .page-ellipsis {
        display: grid;
        width: 26px;
        height: 26px;
        place-items: center;
        color: var(--mantine-color-dimmed);
    }
    .page-number.current {
        border-color: transparent;
        background: var(--mantine-color-cyan-filled);
        color: var(--mantine-color-white);
    }
    @media (max-width: 500px) {
        th:nth-child(2) {
            width: 49%;
        }
        .pages {
            flex-direction: column;
            padding: var(--mantine-spacing-xs);
        }
        .page-range {
            margin-inline-end: 0;
        }
    }
</style>
