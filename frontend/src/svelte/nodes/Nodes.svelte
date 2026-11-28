<script lang="ts">
    import {
        BulkNodesActionsCommand,
        NODES_BULK_ACTIONS,
        ReorderNodesCommand,
        RestartAllNodesCommand
    } from '@remnawave/backend-contract'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { type Move, moveSelection } from '@shared/lib/move-selection'
    import { fixedVirtualWindow, virtualLayout } from '@shared/lib/virtual-layout'
    import {
        prettifyBytesUtil,
        prettySiRealtimeBytesUtil
    } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { MediaQuery, SvelteMap, SvelteSet } from 'svelte/reactivity'
    import { submitCommand, submitVoidCommand } from '../commands'
    import { profilesQuery } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import { nodePluginsQuery } from '../plugins/queries'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { p } from '../router'
    import type { SummaryMetric } from '../system-stats'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import DataTable from '../ui/DataTable.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import EmptyState from '../ui/EmptyState.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import { applyEntityOrder } from '../ui/entity-order'
    import Icon from '../ui/Icon.svelte'
    import MetricCard from '../ui/MetricCard.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import ThemeIcon from '../ui/ThemeIcon.svelte'
    import { type EntityViewMode, readNodeView, writeNodeView } from '../view-preferences'
    import { type NodeAction, nodeActionLabels, runNodeAction } from './actions'
    import { flag } from './analytics'
    import NodeCard from './NodeCard.svelte'
    import NodeCreation from './NodeCreation.svelte'
    import NodeDetails from './NodeDetails.svelte'
    import NodeEditor from './NodeEditor.svelte'
    import NodeUsersUsage from './NodeUsersUsage.svelte'
    import { type Node, nodesKey, nodesQuery } from './queries'

    const confirmAction = useConfirmation()

    type BulkAction = Exclude<NodeAction, 'delete'>
    type EditMode = 'fields' | 'profile'
    type Column = 'name' | 'address' | 'status' | 'profile' | 'users' | 'traffic' | 'tags'
    const columns = ['name', 'address', 'status', 'profile', 'users', 'traffic', 'tags'] as const
    const bulkActions = {
        enable: NODES_BULK_ACTIONS.ENABLE,
        disable: NODES_BULK_ACTIONS.DISABLE,
        restart: NODES_BULK_ACTIONS.RESTART,
        reset: NODES_BULK_ACTIONS.RESET_TRAFFIC
    }
    const initial = readNodeView()
    const nodes = createQuery(() => ({ ...nodesQuery, refetchInterval: 5_000 }))
    const profiles = createQuery(() => profilesQuery)
    const selection = new SvelteSet<string>()
    let t = $derived(locale.t)
    let mode = $state<EntityViewMode>(initial.mode)
    let tag = $state(initial.tag)
    let creating = $state(false)
    let detailsNode = $state<string>()
    let confirmRestart = $state(false)
    let usageTargets = $state<string[]>()
    let sort = $state<{ column: Column; descending: boolean }>()
    let pendingOrder = $state.raw<string[]>()
    let ordered = $derived(applyEntityOrder(nodes.data ?? [], pendingOrder))
    const plugins = createQuery(() => ({
        ...nodePluginsQuery,
        enabled: ordered.some(({ activePluginUuid }) => activePluginUuid !== null)
    }))
    let pluginNames = $derived(
        new Map(plugins.data?.nodePlugins.map(({ uuid, name }) => [uuid, name]))
    )
    let metrics = $derived<SummaryMetric[]>([
        {
            label: t('nodes-realtime-metrics.widget.download-speed'),
            value:
                prettySiRealtimeBytesUtil(
                    ordered.reduce(
                        (sum, node) => sum + (node.system?.stats.interface?.rxBytesPerSec ?? 0),
                        0
                    ),
                    true,
                    true
                ) || '0 B/s',
            icon: 'PiArrowDownDuotone',
            color: 'teal'
        },
        {
            label: t('nodes-realtime-metrics.widget.upload-speed'),
            value:
                prettySiRealtimeBytesUtil(
                    ordered.reduce(
                        (sum, node) => sum + (node.system?.stats.interface?.txBytesPerSec ?? 0),
                        0
                    ),
                    true,
                    true
                ) || '0 B/s',
            icon: 'PiArrowUpDuotone',
            color: 'indigo'
        },
        {
            label: t('nodes-quick-stats.widget.users-online'),
            value: ordered.reduce((sum, node) => sum + node.usersOnline, 0),
            icon: 'TbUsers',
            color: 'cyan'
        },
        {
            label: t('nodes-quick-stats.widget.online-nodes'),
            value: ordered.filter((node) => node.isConnected).length,
            icon: 'PiPulse',
            color: 'teal'
        }
    ])
    let selected = $derived(
        ordered.filter(({ uuid }) => selection.has(uuid)).map(({ uuid }) => uuid)
    )
    let profileNames = $derived(
        new Map(profiles.data?.configProfiles.map(({ uuid, name }) => [uuid, name]))
    )
    let visible = $derived.by(() => {
        const filtered = ordered.filter(
            (node) => !tag || (tag === NO_TAG ? node.tags.length === 0 : node.tags.includes(tag))
        )
        if (!sort) return filtered
        const { column, descending } = sort
        return filtered.sort((a, b) => {
            const cmp =
                column === 'users'
                    ? a.usersOnline - b.usersOnline
                    : column === 'traffic'
                      ? (a.trafficUsedBytes ?? 0) - (b.trafficUsedBytes ?? 0)
                      : value(a, column).localeCompare(value(b, column), locale.language, {
                            numeric: true
                        })
            return descending ? -cmp : cmp
        })
    })
    let canReorder = $derived(!sort && !tag && pendingOrder === undefined)
    let area = $state<HTMLDivElement>()
    let height = $state(0)
    let offset = $state(0)
    const mobile = new MediaQuery('(max-width: 48em)')
    const cardSizes = new SvelteMap<string, number>()
    let draggedNode = $state<string>()
    let rowHeight = $derived(mode === 'table' ? 50 : mobile.current ? 190 : 90)
    let range = $derived(fixedVirtualWindow(visible.length, rowHeight, offset, height))
    let cardLayout = $derived(
        mode === 'cards'
            ? virtualLayout(
                  {
                      count: visible.length,
                      estimateSize: () => rowHeight,
                      getItemKey: (index) => visible[index]?.uuid ?? index,
                      overscan: 4,
                      ...(draggedNode ? { pinnedKey: draggedNode } : {})
                  },
                  // The first cards give the scroll container its measurable height.
                  { offset, height: height || rowHeight },
                  cardSizes
              )
            : undefined
    )
    let cardItems = $derived(cardLayout?.virtualItems ?? [])
    let cardBottom = $derived(
        cardItems.length
            ? Math.max(
                  0,
                  (cardLayout?.totalSize ?? 0) - (cardItems[cardItems.length - 1]?.end ?? 0)
              )
            : 0
    )
    let rows = $derived(visible.slice(range.from, range.to))
    let edit = $state<{ mode: EditMode; ids: string[] }>()

    const update = createMutation(() => ({
        mutationFn: async (
            change:
                | { kind: 'bulk'; ids: string[]; action: BulkAction }
                | { kind: 'one'; uuid: string; action: NodeAction }
                | { kind: 'order'; ids: string[] }
                | { kind: 'restartAll'; forceRestart: boolean }
        ) => {
            switch (change.kind) {
                case 'one':
                    return executeOne(change.uuid, change.action)
                case 'bulk':
                    return submitVoidCommand(BulkNodesActionsCommand, {
                        uuids: change.ids,
                        action: bulkActions[change.action]
                    })
                case 'order': {
                    const ordered = await submitCommand(ReorderNodesCommand, {
                        nodes: change.ids.map((uuid, index) => ({ uuid, viewPosition: index + 1 }))
                    })
                    queryClient.setQueryData(nodesQuery.queryKey, ordered)
                    return
                }
                case 'restartAll':
                    return submitVoidCommand(RestartAllNodesCommand, {
                        forceRestart: change.forceRestart
                    })
            }
        },
        onSuccess: (_data, change) => {
            if (change.kind === 'order') return
            notifications.show({
                title: t('common.message.success'),
                message:
                    change.kind === 'restartAll'
                        ? t('nodes-header-action-buttons.feature.restart-requested')
                        : t('common.message.operation-completed'),
                color: 'teal'
            })
        },
        onSettled: async (_, __, change) => {
            try {
                await queryClient.invalidateQueries({ queryKey: nodesKey })
            } finally {
                if (change.kind === 'order') pendingOrder = undefined
            }
        }
    }))

    async function executeOne(uuid: string, action: NodeAction): Promise<void> {
        await runNodeAction(uuid, action)
        if (action === 'delete') selection.delete(uuid)
    }

    function scrollToStart(): void {
        if (area) area.scrollTop = 0
        offset = 0
    }

    function view(next: EntityViewMode): void {
        mode = next
        sort = undefined
        writeNodeView(mode, tag)
        scrollToStart()
    }

    function sortBy(column: Column): void {
        sort =
            sort?.column === column
                ? sort.descending
                    ? undefined
                    : { column, descending: true }
                : { column, descending: false }
        scrollToStart()
    }

    function nodeState(node: Node): string {
        if (node.isDisabled) return 'Disabled'
        if (node.isConnecting) return 'Connecting'
        return node.isConnected ? 'Online' : 'Offline'
    }

    function profileName(node: Node): string {
        const uuid = node.configProfile.activeConfigProfileUuid
        return uuid ? (profileNames.get(uuid) ?? uuid) : '—'
    }

    function value(node: Node, column: Column): string {
        switch (column) {
            case 'address':
                return node.port ? `${node.address}:${node.port}` : node.address
            case 'status':
                return nodeState(node)
            case 'profile':
                return profileName(node)
            case 'users':
                return node.usersOnline.toLocaleString(locale.language)
            case 'traffic':
                return prettifyBytesUtil(node.trafficUsedBytes ?? 0) ?? '—'
            case 'tags':
                return node.tags.join(', ')
            default:
                return node.name
        }
    }

    function label(column: Column): string {
        const labels = {
            name: 'common.field.name',
            address: 'common.field.address',
            status: 'common.field.status',
            profile: 'use-nodes-table-widget.config-profile',
            users: 'constants.users',
            traffic: 'use-nodes-table-widget.traffic-used',
            tags: 'common.field.tags'
        } as const
        return t(labels[column])
    }

    function move(direction: Move): void {
        if (!canReorder || update.isPending) return
        const current = ordered.map(({ uuid }) => uuid)
        const next = moveSelection(current, selection, direction)
        if (next.every((uuid, index) => uuid === current[index])) return
        saveOrder(next)
    }

    function reorder(source: string, target: string): void {
        if (!canReorder || update.isPending || source === target) return
        const ids = ordered.map(({ uuid }) => uuid)
        const from = ids.indexOf(source)
        const to = ids.indexOf(target)
        if (from < 0 || to < 0) return
        ids.splice(from, 1)
        ids.splice(to, 0, source)
        saveOrder(ids)
    }

    function saveOrder(ids: string[]): void {
        pendingOrder = ids
        update.mutate({ kind: 'order', ids })
    }

    async function apply(action: NodeAction, uuid?: string): Promise<void> {
        if (update.isPending) return
        const ids = selected
        if (
            (action === 'delete' || action === 'restart' || action === 'reset') &&
            !(await confirmAction(t('common.message.confirm-action-description')))
        )
            return
        if (uuid) update.mutate({ kind: 'one', uuid, action })
        else if (action !== 'delete' && ids.length) update.mutate({ kind: 'bulk', ids, action })
    }

    function open(mode: EditMode): void {
        edit = { mode, ids: selected }
    }
    function openNode(uuid: string, event?: MouseEvent): void {
        if (event) {
            if (
                event.button !== 0 ||
                event.metaKey ||
                event.ctrlKey ||
                event.shiftKey ||
                event.altKey
            )
                return
            event.preventDefault()
        }
        detailsNode = uuid
    }
</script>

<svelte:head><title>{t('constants.nodes')} | Gowave</title></svelte:head>
<main class="nodes-page">
    <div class="node-metrics">
        {#each metrics as metric (metric.label)}
            <MetricCard {...metric} />
        {/each}
    </div>
    <PageHeader title={t('constants.nodes')} icon="HiServer">
        <EntitySpotlight
            icon="TbServer"
            searchIcon="TbServer2"
            items={ordered.map((node) => ({
                id: node.uuid,
                name: node.name,
                flag: node.countryCode !== 'XX' ? flag(node.countryCode) : '',
                badges: [
                    {
                        icon: 'PiUsersDuotone',
                        count: node.usersOnline,
                        tone: node.usersOnline ? 'teal' : 'gray'
                    }
                ],
                description: [node.address, node.provider?.name, node.uuid]
                    .filter(Boolean)
                    .join(' | '),
                select: () => openNode(node.uuid)
            }))}
        />
        <ActionIcon
            icon={mode === 'cards' ? 'TbTable' : 'TbCards'}
            tone="gray"
            label={mode === 'cards' ? 'Table' : 'Cards'}
            onclick={() => view(mode === 'cards' ? 'table' : 'cards')}
        />
        <ActionIcon
            icon="PiSpiral"
            tone="grape"
            label={t('nodes-header-action-buttons.feature.restart-all-nodes')}
            pending={update.isPending && update.variables?.kind === 'restartAll'}
            disabled={update.isPending}
            onclick={() => (confirmRestart = true)}
        />
        <ActionIcon
            icon="TbRefresh"
            tone="cyan"
            label={t('common.action.refresh')}
            pending={nodes.isFetching && !nodes.isPending}
            disabled={nodes.isPending}
            onclick={() => void nodes.refetch()}
        />
        <ActionIcon
            icon="TbPlus"
            tone="teal"
            label={t('common.action.create')}
            onclick={() => (creating = true)}
        />
    </PageHeader>
    <EntityTagBar
        items={ordered}
        active={tag}
        select={(value) => {
            tag = value
            writeNodeView(mode, tag)
            scrollToStart()
        }}
    />

    {#if nodes.isPending}
        <RouteLoading height="60vh" label={t('common.message.loading')} />
    {:else if nodes.isError}
        <p role="alert">{nodes.error.message}</p>
    {:else}
        {#if selected.length}
            <div class="button-row toolbar">
                <Button
                    disabled={!ordered.length || selected.length === ordered.length}
                    onclick={() => {
                        for (const { uuid } of ordered) selection.add(uuid)
                    }}
                >
                    {t('common.action.select-all')}
                </Button>
                {#if selected.length}
                    <Button onclick={() => selection.clear()}>
                        {t('common.action.clear-selection')}
                    </Button>
                    <span>{t('common.message.selected', { count: selected.length })}</span>
                    <Button disabled={update.isPending} onclick={() => open('fields')}>
                        {t('common.action.bulk-edit')}
                    </Button>
                    <Button disabled={update.isPending} onclick={() => open('profile')}>
                        {t('multi-select-nodes.feature.profile-and-inbounds')}
                    </Button>
                    <Button onclick={() => (usageTargets = selected)}>
                        {t('common.field.usage-stats')}
                    </Button>
                    {#each Object.keys(bulkActions) as name (name)}
                        {@const action = name as BulkAction}
                        <Button disabled={update.isPending} onclick={() => apply(action)}>
                            {t(nodeActionLabels[action])}
                        </Button>
                    {/each}
                    {#if canReorder}
                        {#each [
                            ['top', 'Move to top'],
                            ['up', 'Move up'],
                            ['down', 'Move down'],
                            ['bottom', 'Move to bottom']
                        ] as [direction, title] (direction)}
                            <Button
                                disabled={update.isPending}
                                onclick={() => move(direction as Move)}
                            >
                                {title}
                            </Button>
                        {/each}
                    {/if}
                {/if}
            </div>
        {/if}
        {#if !visible.length}
            <EmptyState icon="PiCpu" label={t('common.message.nothing-found')} />
        {:else}
            <div
                class="scroll"
                class:cards={mode === 'cards'}
                bind:this={area}
                bind:clientHeight={height}
                onscroll={(event) => (offset = event.currentTarget.scrollTop)}
            >
                {#if mode === 'cards'}
                    <ul aria-label={t('constants.nodes')}>
                        {#each cardItems as item, index (item.key)}
                            {@const node = visible[item.index]}
                            {@const gap = item.start - (cardItems[index - 1]?.end ?? 0)}
                            {#if gap > 0}
                                <li aria-hidden="true" style:height={`${gap}px`}></li>
                            {/if}
                            {#if node}
                                <NodeCard
                                    {node}
                                    status={nodeState(node)}
                                    height={rowHeight}
                                    mobile={mobile.current}
                                    plugin={node.activePluginUuid
                                        ? pluginNames.get(node.activePluginUuid)
                                        : undefined}
                                    onmeasure={(measured) => {
                                        if (cardSizes.get(node.uuid) !== measured)
                                            cardSizes.set(node.uuid, measured)
                                    }}
                                    onopen={(event) => openNode(node.uuid, event)}
                                    reorder={canReorder && !update.isPending ? reorder : undefined}
                                    onreordering={(active) =>
                                        (draggedNode = active ? node.uuid : undefined)}
                                >
                                    {#snippet selection()}
                                        {@render select(node)}
                                    {/snippet}
                                    {#snippet actions()}
                                        {@render nodeActions(node)}
                                    {/snippet}
                                </NodeCard>
                            {/if}
                        {/each}
                        <li aria-hidden="true" style:height={`${cardBottom}px`}></li>
                    </ul>
                {:else}
                    <DataTable class="nodes-table" aria-label={t('constants.nodes')}>
                        <thead>
                            <tr>
                                <th scope="col">{t('common.action.select')}</th>
                                {#each columns as column (column)}
                                    {@const direction =
                                        sort?.column === column
                                            ? sort.descending
                                                ? 'descending'
                                                : 'ascending'
                                            : 'none'}
                                    <th scope="col" aria-sort={direction}>
                                        <button
                                            class="mantine-UnstyledButton-root table-sort-button"
                                            type="button"
                                            onclick={() => sortBy(column)}
                                        >
                                            <span>{label(column)}</span>
                                            <span class="table-sort-indicator" aria-hidden="true">
                                                <Icon
                                                    name={direction === 'ascending'
                                                        ? 'TbArrowUp'
                                                        : direction === 'descending'
                                                          ? 'TbArrowDown'
                                                          : 'TbArrowsSort'}
                                                    size={14}
                                                />
                                            </span>
                                        </button>
                                    </th>
                                {/each}
                                <th scope="col">{t('common.action.actions')}</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr aria-hidden="true" style:height={`${range.from * rowHeight}px`}>
                                <td colspan="9"></td>
                            </tr>
                            {#each rows as node (node.uuid)}
                                <tr style:height={`${rowHeight}px`}>
                                    <td>{@render select(node)}</td>
                                    <th scope="row">{@render nodeLink(node)}</th>
                                    {#each columns.slice(1) as column (column)}
                                        <td>{value(node, column)}</td>
                                    {/each}
                                    <td>{@render nodeActions(node)}</td>
                                </tr>
                            {/each}
                            <tr
                                aria-hidden="true"
                                style:height={`${(visible.length - range.to) * rowHeight}px`}
                            >
                                <td colspan="9"></td>
                            </tr>
                        </tbody>
                    </DataTable>
                {/if}
            </div>
        {/if}
    {/if}
    {#if update.error}
        <p role="alert">{update.error.message}</p>
    {/if}
    {#if usageTargets}
        <NodeUsersUsage
            target={usageTargets}
            name={t('common.message.selected', { count: usageTargets.length })}
            close={() => (usageTargets = undefined)}
        />
    {/if}
    {#if edit}
        <Overlay
            title={edit.mode === 'profile'
                ? t('multi-select-nodes.feature.profile-and-inbounds')
                : t('common.action.bulk-edit')}
            subtitle={t('common.message.selected', { count: edit.ids.length })}
            icon="TbServer"
            width={928}
            close={() => (edit = undefined)}
        >
            <NodeEditor
                ids={edit.ids}
                profileOnly={edit.mode === 'profile'}
                onsaved={() => {
                    notifications.show({
                        title: t('common.message.success'),
                        message: t('common.message.operation-completed'),
                        color: 'teal'
                    })
                    edit = undefined
                }}
            />
        </Overlay>
    {/if}
    {#if creating}
        <NodeCreation
            close={() => (creating = false)}
            openNode={(uuid) => {
                creating = false
                openNode(uuid)
            }}
        />
    {/if}
</main>

{#if confirmRestart}
    <Overlay
        title={t('nodes-header-action-buttons.feature.restart-all-nodes')}
        icon="TbRocket"
        tone="teal"
        width={440}
        fullscreenOnMobile={false}
        close={() => (confirmRestart = false)}
    >
        <div class="restart-options">
            {#each [true, false] as forceRestart (forceRestart)}
                <button
                    type="button"
                    class="restart-option"
                    disabled={!ordered.length || update.isPending}
                    onclick={(event) => {
                        update.mutate({ kind: 'restartAll', forceRestart })
                        dismissContainingDialog(event)
                    }}
                >
                    <ThemeIcon
                        icon={forceRestart ? 'TbAlertCircle' : 'TbRocket'}
                        tone={forceRestart ? 'red' : 'teal'}
                    />
                    <span>
                        <strong>
                            {t(
                                forceRestart
                                    ? 'nodes-header-action-buttons.feature.force'
                                    : 'nodes-header-action-buttons.feature.graceful'
                            )}
                        </strong>
                        <small>
                            {t(
                                forceRestart
                                    ? 'nodes-header-action-buttons.feature.force-restart-description'
                                    : 'nodes-header-action-buttons.feature.graceful-restart-description-1'
                            )}
                        </small>
                    </span>
                </button>
            {/each}
        </div>
    </Overlay>
{/if}

{#snippet select(node: Node)}
    <input
        type="checkbox"
        aria-label={`${t('common.action.select')} ${node.name}`}
        checked={selection.has(node.uuid)}
        onchange={(event) => {
            if (event.currentTarget.checked) selection.add(node.uuid)
            else selection.delete(node.uuid)
        }}
    >
{/snippet}

{#snippet nodeLink(node: Node)}
    <a
        href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}
        onclick={(event) => openNode(node.uuid, event)}
        >{node.name}</a
    >
{/snippet}

{#snippet nodeActions(node: Node)}
    <div class="button-row node-actions">
        {#each [node.isDisabled ? 'enable' : 'disable', 'restart', 'delete'] as action (action)}
            <ActionIcon
                icon={action === 'restart'
                    ? 'TbRefresh'
                    : action === 'delete'
                      ? 'PiTrashDuotone'
                      : action === 'enable'
                        ? 'TbCheck'
                        : 'PiProhibit'}
                label={t(nodeActionLabels[action as NodeAction])}
                size={28}
                iconSize={16}
                subtle
                pending={update.isPending &&
                    update.variables?.kind === 'one' &&
                    update.variables.uuid === node.uuid &&
                    update.variables.action === action}
                disabled={update.isPending}
                onclick={() => apply(action as NodeAction, node.uuid)}
            />
        {/each}
    </div>
{/snippet}

{#if detailsNode}
    <NodeDetails nodeId={detailsNode} onclose={() => (detailsNode = undefined)} />
{/if}

<style>
    .nodes-page {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-md);
    }

    .node-metrics {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-xs);
    }

    @media (min-width: 30em) {
        .node-metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }

    @media (min-width: 80em) {
        .node-metrics {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }

    .toolbar {
        align-items: center;
        margin-block-end: 0.75rem;
    }
    .restart-options {
        display: grid;
        gap: var(--mantine-spacing-sm);
    }
    .restart-option {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm);
        text-align: start;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-text);
    }
    .restart-option:hover:not(:disabled) {
        background: var(--mantine-color-default-hover);
    }
    .restart-option:disabled {
        opacity: 0.5;
    }
    .restart-option :is(strong, small) {
        display: block;
    }
    .restart-option small {
        color: var(--mantine-color-dimmed);
    }
    .scroll {
        max-height: min(65vh, 44rem);
        overflow: auto;
        overflow-anchor: none;
        border: 1px solid var(--gw-border);
        border-radius: 0.5rem;
    }
    ul {
        padding: 0;
        margin: 0;
        list-style: none;
    }
    .scroll.cards {
        border: 0;
        border-radius: 0;
    }
    .node-actions {
        flex-wrap: nowrap;
    }
    :global(.nodes-table) {
        border-collapse: collapse;
        width: 100%;
        white-space: nowrap;
    }
    th,
    td {
        text-align: start;
        padding-inline: 0.5rem;
    }
    thead {
        position: sticky;
        inset-block-start: 0;
        background: var(--gw-surface);
    }
    thead th {
        height: 40px;
        font-size: var(--mantine-font-size-sm);
        box-shadow: inset 0 -1px var(--gw-border);
    }
    tbody tr:not([aria-hidden]) {
        border-block-end: 1px solid var(--gw-border);
    }
</style>
