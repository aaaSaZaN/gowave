<script lang="ts">
    import { TruncateTorrentBlockerReportsCommand } from '@remnawave/backend-contract'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import { nodesQuery } from '../nodes/queries'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { p } from '../router'
    import type { Column } from '../tools/columns'
    import InspectorTable from '../tools/InspectorTable.svelte'
    import { torrentStorage } from '../tools/table-state'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import JsonTree from '../ui/JsonTree.svelte'
    import Leaderboard from '../ui/Leaderboard.svelte'
    import MetricCard from '../ui/MetricCard.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { reportRows, reportStats, type TorrentReport } from './queries'

    let t = $derived(locale.t)
    const stats = createQuery(() => reportStats)
    const nodes = createQuery(() => nodesQuery)
    const truncate = createMutation(() => ({
        mutationFn: () => executeCommand(TruncateTorrentBlockerReportsCommand),
        onSuccess: () => {
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
            return queryClient.invalidateQueries({ queryKey: [torrentStorage.name] })
        }
    }))
    let raw = $state<TorrentReport>()

    let columns: Column<TorrentReport>[] = $derived([
        {
            key: 'user.username',
            label: 'Username',
            filter: 'text',
            align: 'center',
            value: (row) => row.user.username
        },
        {
            key: 'userId',
            label: 'User ID',
            filter: 'text',
            align: 'center'
        },
        {
            key: 'node.uuid',
            label: 'Reported Node',
            filter: 'select',
            options:
                nodes.data?.map(({ uuid, name, countryCode }) => ({
                    value: uuid,
                    label: name,
                    flag: flag(countryCode ?? '')
                })) ?? [],
            value: (row) => row.node.name,
            prefix: (row) => flag(row.node.countryCode),
            monospace: true,
            sortable: false
        },
        {
            key: 'report.actionReport.ip',
            label: 'IP',
            filter: 'text',
            align: 'center',
            sortable: false,
            value: (row) => row.report.actionReport.ip
        },
        {
            key: 'report.xrayReport.inboundTag',
            label: 'Inbound Tag',
            filter: 'text',
            align: 'center',
            sortable: false,
            value: (row) => row.report.xrayReport.inboundTag
        },
        { key: 'createdAt', label: t('common.field.created-at'), width: 250, align: 'center' },
        {
            key: 'report.xrayReport.outboundTag',
            label: 'Outbound Tag',
            width: 300,
            filter: 'text',
            align: 'center',
            sortable: false,
            value: (row) => row.report.xrayReport.outboundTag
        },
        {
            key: 'report.xrayReport.protocol',
            label: 'Protocol',
            filter: 'text',
            align: 'center',
            sortable: false,
            value: (row) => row.report.xrayReport.protocol
        }
    ])

    function removeReports(): void {
        if (!window.confirm(t('torrent-blocker-reports-table.widget.truncate-reports-description')))
            return
        truncate.mutate()
    }
</script>

<svelte:head><title>{t('constants.tb-reports')} | Gowave</title></svelte:head>
<main>
    {#if stats.isPending}
        <RouteLoading compact />
    {/if}
    {#if stats.error || nodes.error || truncate.error}
        <p role="alert">
            {stats.error?.message || nodes.error?.message || truncate.error?.message}
        </p>
    {/if}
    <div class="metrics">
        <MetricCard
            label={t('torrent-blocker-stats.widget.total-reports')}
            value={stats.data?.stats.totalReports ?? 0}
            icon="TbFileReport"
            color="cyan"
        />
        <MetricCard
            label={t('torrent-blocker-stats.widget.last-24-hours')}
            value={stats.data?.stats.reportsLast24Hours ?? 0}
            icon="TbClockHour2"
            color="cyan"
        />
        <MetricCard
            label={t('torrent-blocker-stats.widget.distinct-nodes')}
            value={stats.data?.stats.distinctNodes ?? 0}
            icon="TbServer"
            color="teal"
        />
        <MetricCard
            label={t('torrent-blocker-stats.widget.distinct-users')}
            value={stats.data?.stats.distinctUsers ?? 0}
            icon="TbUsers"
            color="red"
        />
    </div>
    {#if stats.data}
        {@const data = stats.data}
        <div class="leaders">
            <section>
                <header>
                    <span class="leader-icon" style:--tone="var(--mantine-color-red-5)"
                        ><Icon name="TbUsers" size={20} /></span
                    >
                    <div>
                        <h2>{t('torrent-blocker-stats.widget.top-users')}</h2>
                        <small>{t('torrent-blocker-stats.widget.by-report-count')}</small>
                    </div>
                </header>
                <Leaderboard
                    items={data.topUsers.map((user) => ({
                        name: user.username,
                        color: user.color,
                        total: user.total,
                        href: p('/dashboard/open/user/:id', { params: { id: String(user.userId) } })
                    }))}
                    maxHeight={230}
                    emptyText={t('common.message.no-data-available')}
                />
            </section>
            <section>
                <header>
                    <span class="leader-icon" style:--tone="var(--mantine-color-teal-5)"
                        ><Icon name="TbServer" size={20} /></span
                    >
                    <div>
                        <h2>{t('statistic-nodes.component.top-nodes')}</h2>
                        <small>{t('torrent-blocker-stats.widget.by-report-count')}</small>
                    </div>
                </header>
                <Leaderboard
                    items={data.topNodes.map((node) => ({
                        name: node.name,
                        color: node.color,
                        total: node.total,
                        href: p('/dashboard/open/node/:id', { params: { id: node.uuid } })
                    }))}
                    maxHeight={230}
                    emptyText={t('common.message.no-data-available')}
                />
            </section>
        </div>
    {/if}

    <InspectorTable
        icon="TbFlame"
        title={t('constants.tb-reports')}
        setup={torrentStorage}
        {columns}
        rowKey={(row) => String(row.id)}
        ipAddress={(row) => row.report.actionReport.ip}
        load={reportRows}
        onrefresh={() => void stats.refetch()}
    >
        {#snippet rowActions(row)}
            <ActionIcon
                icon="TbJson"
                tone="gray"
                size={36}
                label="Raw Report"
                onclick={() => (raw = row)}
            />
        {/snippet}
        {#snippet tableActions()}
            <ActionIcon
                icon="PiTrashDuotone"
                tone="red"
                label={t('torrent-blocker-reports-table.widget.truncate-reports')}
                onclick={removeReports}
                disabled={truncate.isPending}
            />
        {/snippet}
    </InspectorTable>
</main>

{#if raw}
    <Overlay
        title="Raw Report"
        icon="TbJson"
        tone="gray"
        width={780}
        class="torrent-json-dialog"
        fullscreenOnMobile={false}
        close={() => (raw = undefined)}
    >
        <JsonTree value={raw.report} />
    </Overlay>
{/if}

<style>
    main,
    .metrics,
    .leaders {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
    .metrics {
        gap: var(--mantine-spacing-xs);
    }
    @media (min-width: 30em) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 40em) {
        .leaders {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 80em) {
        .metrics {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
    .leaders > section {
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        background: var(--gw-raised-bg);
        border-radius: var(--mantine-radius-md);
    }
    header {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding-bottom: var(--mantine-spacing-md);
        margin-bottom: var(--mantine-spacing-md);
        border-bottom: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    .leader-icon {
        display: grid;
        place-items: center;
        width: 34px;
        height: 34px;
        color: var(--tone);
        background: color-mix(in srgb, var(--tone) 12%, transparent);
        border: 1px solid color-mix(in srgb, var(--tone) 30%, transparent);
        border-radius: var(--mantine-radius-md);
    }
    small {
        display: block;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    h2 {
        margin: 0;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h5-font-size);
        font-weight: var(--mantine-h5-font-weight);
        line-height: var(--mantine-h5-line-height);
    }
    :global(dialog.panel-overlay.torrent-json-dialog) {
        max-width: 90vw;
    }
    :global(dialog.torrent-json-dialog .overlay-header) {
        padding-block-start: calc(var(--mantine-spacing-sm) + 1px);
    }
</style>
