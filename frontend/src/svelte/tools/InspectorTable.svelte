<script lang="ts" generics="Row extends InspectorRow">
    import { formatTimeUtil } from '@shared/utils/time-utils/format-time.util'
    import { createQuery } from '@tanstack/svelte-query'
    import { type Snippet, untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import { notifications } from '../ui/notification-state.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import TableTitle from '../ui/TableTitle.svelte'
    import type { Column, InspectorRow } from './columns'
    import type { InspectorParams } from './queries'
    import {
        type FilterMode,
        type InspectorStorage,
        type InspectorTableState,
        inspectorRequest,
        readInspectorState
    } from './table-state'

    let {
        title,
        icon,
        setup,
        columns,
        rowKey,
        ipAddress,
        load,
        rowActions,
        tableActions,
        onrefresh
    }: {
        title: string
        icon: IconName
        setup: InspectorStorage
        columns: Column<Row>[]
        rowKey: (row: Row) => string
        ipAddress: (row: Row) => string | null
        load: (
            filters: InspectorParams,
            signal: AbortSignal
        ) => Promise<{ rows: Row[]; total: number }>
        rowActions?: Snippet<[Row]>
        tableActions?: Snippet
        onrefresh?: () => void
    } = $props()

    let columnsOpen = $state(false)
    let t = $derived(locale.t)
    let prefs = $state<InspectorTableState>(untrack(() => readInspectorState(setup)))
    let error = $state('')
    let live = $state(true)
    let selectedUser = $state<InspectorRow['userId']>()
    let panel: HTMLElement
    let fullscreen = $state(false)
    let byKey = $derived(
        new Map<string, Column<Row>>(columns.map((column) => [column.key, column]))
    )
    let filterable = $derived(columns.filter((column) => column.filter))
    let sortable = $derived(
        new Set(columns.filter((column) => column.sortable !== false).map(({ key }) => key))
    )
    let allOrder = $derived([
        ...new Set([
            ...prefs.columnOrder.filter((id) => byKey.has(id)),
            ...columns.map(({ key }) => key)
        ])
    ])
    let visible = $derived(allOrder.filter((id) => prefs.columnVisibility[id] !== false))
    let left = $derived(prefs.columnPinning.left.filter((id) => visible.includes(id)))
    let right = $derived(
        prefs.columnPinning.right.filter((id) => visible.includes(id) && !left.includes(id))
    )
    let ordered = $derived([
        ...left,
        ...visible.filter((id) => !left.includes(id) && !right.includes(id)),
        ...right
    ])
    let params = $derived(
        inspectorRequest(
            prefs,
            new Set(filterable.map(({ key }) => key)),
            sortable,
            new Map<string, FilterMode>(
                filterable.map((column) => [
                    column.key,
                    column.filterMode === 'choose'
                        ? (prefs.filterModes?.[column.key] ?? 'contains')
                        : (column.filterMode ?? 'contains')
                ])
            )
        )
    )
    const query = createQuery(() => ({
        queryKey: [setup.name, 'rows', params],
        queryFn: ({ signal }) => load(params, signal),
        refetchInterval: live ? 25_000 : false,
        staleTime: 10_000
    }))
    let pages = $derived(
        Math.max(1, Math.ceil((query.data?.total ?? 0) / prefs.paginationState.pageSize))
    )
    let pageSizeOptions = $derived(
        [...new Set([10, 25, 50, 100, prefs.paginationState.pageSize])]
            .sort((a, b) => a - b)
            .map((size) => ({ value: String(size), label: String(size) }))
    )
    const actionsWidth = $derived(rowActions ? 130 : 110)
    const total = $derived(query.data?.total ?? 0)
    const firstRow = $derived(
        total ? prefs.paginationState.pageSize * prefs.paginationState.pageIndex + 1 : 0
    )
    const lastRow = $derived(
        Math.min(total, prefs.paginationState.pageSize * (prefs.paginationState.pageIndex + 1))
    )

    $effect(() => {
        if (query.data && prefs.paginationState.pageIndex >= pages) page(pages - 1)
    })

    function store(): void {
        try {
            window.localStorage.setItem(
                setup.name,
                JSON.stringify({ version: setup.version, state: prefs })
            )
        } catch {
            // The current table state remains usable.
        }
    }

    function setFilter(id: string, value: string): void {
        prefs.columnFilter = prefs.columnFilter.filter((filter) => filter.id !== id)
        if (value.trim()) prefs.columnFilter.push({ id, value })
        page(0)
    }

    function reset(): void {
        prefs = readInspectorState(setup, { getItem: () => null })
        store()
    }

    function page(index: number): void {
        prefs.paginationState.pageIndex = index
        store()
    }

    function sort(id: string, append: boolean): void {
        if (!sortable.has(id)) return
        const current = append ? prefs.sorting.find((entry) => entry.id === id) : prefs.sorting[0]
        const next =
            current?.id === id
                ? current.desc
                    ? undefined
                    : { id, desc: true }
                : { id, desc: false }
        prefs.sorting = append
            ? prefs.sorting.flatMap((entry) => (entry.id !== id ? [entry] : next ? [next] : []))
            : next
              ? [next]
              : []
        if (append && !current && next) prefs.sorting.push(next)
        page(0)
    }

    function width(id: string): number {
        return prefs.columnSize[id] ?? byKey.get(id)?.width ?? 180
    }

    function offset(id: string, side: 'left' | 'right'): string | undefined {
        const pins = side === 'left' ? left : right
        const index = pins.indexOf(id)
        if (index === -1) return undefined
        const ahead = side === 'left' ? pins.slice(0, index) : pins.slice(index + 1)
        return `${ahead.reduce((sum, item) => sum + width(item), 0)}px`
    }

    function move(index: number, step: -1 | 1): void {
        const next = allOrder.slice()
        const first = next[index]
        const second = next[index + step]
        if (first === undefined || second === undefined) return
        next[index] = second
        next[index + step] = first
        prefs.columnOrder = next
        store()
    }

    function pin(key: string, side: string): void {
        prefs.columnPinning = {
            left: [
                ...prefs.columnPinning.left.filter((id) => id !== key),
                ...(side === 'left' ? [key] : [])
            ],
            right: [
                ...prefs.columnPinning.right.filter((id) => id !== key),
                ...(side === 'right' ? [key] : [])
            ]
        }
        store()
    }

    function text(value: unknown): string {
        if (value instanceof Date) {
            return formatTimeUtil({
                time: value,
                template: 'TIME_FIRST_DATETIME',
                language: locale.language
            })
        }
        return value == null || value === '' ? '–' : String(value)
    }

    function copy(value: string): void {
        error = ''
        void navigator.clipboard.writeText(value).then(
            () => {
                notifications.show({
                    title: t('common.message.copied'),
                    message: value,
                    color: 'teal'
                })
            },
            (reason: unknown) => {
                error = reason instanceof Error ? reason.message : String(reason)
            }
        )
    }

    function refresh(): void {
        void query.refetch()
        onrefresh?.()
    }

    function toggleFullscreen(): void {
        void (fullscreen ? document.exitFullscreen() : panel.requestFullscreen()).catch(
            (reason: unknown) => {
                error = reason instanceof Error ? reason.message : String(reason)
            }
        )
    }
</script>

<svelte:window onfullscreenchange={() => (fullscreen = document.fullscreenElement === panel)} />
<section
    class="inspector-panel"
    aria-label={title}
    bind:this={panel}
    class:compact={prefs.compact ?? true}
>
    <TableTitle {title} {icon} tone={rowActions ? 'red' : 'cyan'}>
        <div class="heading-controls">
            {@render tableActions?.()}
            <ActionIcon
                icon="TbRefresh"
                tone="cyan"
                label={t('common.action.update')}
                disabled={query.isFetching}
                onclick={refresh}
            />
            <ActionIcon
                icon="TbFilterOff"
                tone="gray"
                label={t('action-group.feature.clear-filters')}
                disabled={query.isFetching}
                onclick={() => {
                    prefs.columnFilter = []
                    page(0)
                }}
            />
            <ActionIcon
                icon="TbRestore"
                tone="gray"
                label={t('action-group.feature.reset-table')}
                onclick={reset}
            />
        </div>
    </TableTitle>
    <div class="table-panel">
        <div class="toolbar">
            <ActionIcon
                icon="TbBaselineDensitySmall"
                label="Compact rows"
                size={34}
                iconSize={24}
                subtle
                aria-pressed={prefs.compact ?? true}
                onclick={() => {
                    prefs.compact = !(prefs.compact ?? true)
                    store()
                }}
            />
            {#if document.fullscreenEnabled}
                <ActionIcon
                    icon={fullscreen ? 'TbMinimize' : 'TbMaximize'}
                    size={34}
                    iconSize={24}
                    subtle
                    label={fullscreen ? 'Exit fullscreen' : 'Fullscreen'}
                    onclick={toggleFullscreen}
                />
            {/if}
            <ActionIcon
                icon="TbColumns"
                label="Columns"
                size={34}
                iconSize={24}
                subtle
                aria-expanded={columnsOpen}
                onclick={() => (columnsOpen = !columnsOpen)}
            />
        </div>
        {#if columnsOpen}
            <div class="column-menu">
                <div class="button-row">
                    <label class="check">
                        <input
                            type="checkbox"
                            bind:checked={prefs.showColumnFilters}
                            onchange={store}
                        >
                        Filters
                    </label>
                    <label class="check">
                        <input type="checkbox" bind:checked={live}>
                        Auto refresh
                    </label>
                </div>
                <ul>
                    {#each allOrder as id, index (id)}
                        {@const heading = byKey.get(id)?.label ?? id}
                        <li class="button-row">
                            <label class="check"
                                ><input
                                    type="checkbox"
                                    checked={visible.includes(id)}
                                    disabled={visible.length === 1 && visible.includes(id)}
                                    onchange={(event) => {
                                        prefs.columnVisibility[id] = event.currentTarget.checked
                                        store()
                                    }}
                                > {heading}</label
                            >
                            <button
                                type="button"
                                disabled={index === 0}
                                onclick={() => move(index, -1)}
                                aria-label={`Move ${heading} earlier`}
                            >
                                ↑
                            </button>
                            <button
                                type="button"
                                disabled={index === allOrder.length - 1}
                                onclick={() => move(index, 1)}
                                aria-label={`Move ${heading} later`}
                            >
                                ↓
                            </button>
                            <select
                                aria-label={`Pin ${heading}`}
                                value={left.includes(id)
                                    ? 'left'
                                    : right.includes(id)
                                      ? 'right'
                                      : ''}
                                onchange={(event) => pin(id, event.currentTarget.value)}
                            >
                                <option value="">Unpinned</option>
                                <option value="left">Start</option>
                                <option value="right">End</option>
                            </select>
                            <input
                                type="number"
                                aria-label={`Width of ${heading}`}
                                min="48"
                                max="800"
                                value={width(id)}
                                onchange={(event) => {
                                    if (
                                        event.currentTarget.validity.valid &&
                                        event.currentTarget.value
                                    ) {
                                        prefs.columnSize[id] = event.currentTarget.valueAsNumber
                                        store()
                                    }
                                }}
                            >
                        </li>
                    {/each}
                </ul>
            </div>
        {/if}

        {#if query.isPending}
            <RouteLoading compact />
        {/if}
        {#if query.error || error}
            <p role="alert">{query.error?.message || error}</p>
        {/if}
        {#if query.data}
            <div class="scroll">
                <table
                    aria-label={title}
                    style:width={`${ordered.reduce((sum, id) => sum + width(id), actionsWidth)}px`}
                >
                    <thead>
                        <tr>
                            <th scope="col" style:width={`${actionsWidth}px`}>
                                {t('common.action.actions')}
                            </th>
                            {#each ordered as id (id)}
                                {@const column = byKey.get(id)}
                                <th
                                    scope="col"
                                    style:width={`${width(id)}px`}
                                    style:inset-inline-start={offset(id, 'left')}
                                    style:inset-inline-end={offset(id, 'right')}
                                    aria-sort={prefs.sorting[0]?.id === id
                                        ? prefs.sorting[0]?.desc
                                            ? 'descending'
                                            : 'ascending'
                                        : 'none'}
                                >
                                    <div class="column-heading">
                                        {#if sortable.has(id)}
                                            <button
                                                class:sorted={prefs.sorting[0]?.id === id}
                                                type="button"
                                                onclick={(event) => sort(id, event.shiftKey)}
                                            >
                                                <span class="column-label"
                                                    >{column?.label ?? id}</span
                                                >
                                                <Icon
                                                    size={16}
                                                    name={prefs.sorting[0]?.id === id
                                                        ? prefs.sorting[0].desc
                                                            ? 'TbSortDescending'
                                                            : 'TbSortAscending'
                                                        : 'TbArrowsSort'}
                                                />
                                            </button>
                                        {:else}
                                            {byKey.get(id)?.label ?? id}
                                        {/if}
                                        <button
                                            type="button"
                                            class="column-options"
                                            aria-label={`Column actions for ${column?.label ?? id}`}
                                            onclick={() => (columnsOpen = !columnsOpen)}
                                        >
                                            <Icon name="TbDotsVertical" size={18} />
                                        </button>
                                    </div>
                                    {#if prefs.showColumnFilters && column?.filter}
                                        <div class="column-filter">
                                            {#if column.filter === 'select'}
                                                {@const value = String(
                                                    prefs.columnFilter.find(
                                                        (filter) => filter.id === id
                                                    )?.value ?? ''
                                                )}
                                                <div class="column-filter-select">
                                                    <SelectField
                                                        label={column.label}
                                                        hideLabel
                                                        placeholder="Filter by..."
                                                        options={column.options ?? []}
                                                        alwaysShowClear
                                                        {value}
                                                        onchoose={(next) => setFilter(id, next)}
                                                    />
                                                </div>
                                            {:else}
                                                <input
                                                    aria-label={`Filter by ${column.label}`}
                                                    type={column.filter === 'number'
                                                        ? 'number'
                                                        : 'text'}
                                                    placeholder="Filter by..."
                                                    value={String(
                                                        prefs.columnFilter.find(
                                                            (filter) => filter.id === id
                                                        )?.value ?? ''
                                                    )}
                                                    onchange={(event) =>
                                                        setFilter(id, event.currentTarget.value)}
                                                    onkeydown={(event) => {
                                                        if (event.key === 'Enter')
                                                            event.currentTarget.blur()
                                                    }}
                                                >
                                            {/if}
                                            {#if column.filterMode === 'choose'}
                                                {@const exact =
                                                    prefs.filterModes?.[id] === 'equals'}
                                                <ActionIcon
                                                    icon="TbFilterCog"
                                                    label={exact
                                                        ? 'Match part of the value'
                                                        : 'Match the entire value'}
                                                    size={25}
                                                    iconSize={22}
                                                    subtle
                                                    aria-pressed={exact}
                                                    onclick={() => {
                                                        prefs.filterModes = {
                                                            ...prefs.filterModes,
                                                            [id]: exact ? 'contains' : 'equals'
                                                        }
                                                        page(0)
                                                    }}
                                                />
                                            {/if}
                                        </div>
                                    {/if}
                                </th>
                            {/each}
                        </tr>
                    </thead>
                    <tbody>
                        {#each query.data.rows as row (rowKey(row))}
                            {@const ip = ipAddress(row)}
                            <tr>
                                <td>
                                    <div class="row-controls">
                                        <ActionIcon
                                            icon="PiUserCircle"
                                            tone="cyan"
                                            size={36}
                                            label={t('node-active-session.item.widget.view-user')}
                                            onclick={() => (selectedUser = row.userId)}
                                        />
                                        <ActionIcon
                                            icon="TbExternalLink"
                                            tone="grape"
                                            size={36}
                                            label="IP info"
                                            class="ip-info-action"
                                            href={ip
                                                ? `https://ipinfo.io/${encodeURIComponent(ip)}`
                                                : undefined}
                                            disabled={!ip}
                                            target="_blank"
                                            rel="noreferrer"
                                        />
                                        {@render rowActions?.(row)}
                                    </div>
                                </td>
                                {#each ordered as id (id)}
                                    {@const column = byKey.get(id)}
                                    {@const raw =
                                        column && 'value' in column
                                            ? column.value(row)
                                            : row[id as keyof Row]}
                                    {@const value = text(raw)}
                                    {@const prefix = column?.prefix?.(row)}
                                    <td
                                        class:pinned={left.includes(id) || right.includes(id)}
                                        class:timestamp={raw instanceof Date}
                                        class:centered={column?.align === 'center'}
                                        class:monospace={column?.monospace}
                                        style:inset-inline-start={offset(id, 'left')}
                                        style:inset-inline-end={offset(id, 'right')}
                                    >
                                        {#if value === '–'}
                                            {value}
                                        {:else}
                                            <button
                                                type="button"
                                                class="cell"
                                                class:prefixed={prefix}
                                                title={value}
                                                style:max-width={`${width(id) - 8}px`}
                                                onclick={() => copy(value)}
                                            >
                                                {#if prefix}
                                                    <span class="cell-prefix" aria-hidden="true"
                                                        >{prefix}</span
                                                    >
                                                {/if}
                                                {#if prefix}
                                                    <span class="cell-value">{value}</span>
                                                {:else}
                                                    {value}
                                                {/if}
                                            </button>
                                        {/if}
                                    </td>
                                {/each}
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
            {#if !query.data.rows.length}
                <p class="empty">{t('common.message.no-records-to-display')}</p>
            {/if}
            <div class="button-row pagination">
                <div class="rows-per-page">
                    <span class="rows-label">{t('common.field.rows-per-page')}</span>
                    <div class="page-size-select">
                        <SelectField
                            label={t('common.field.rows-per-page')}
                            hideLabel
                            searchable={false}
                            clearable={false}
                            placeholder=""
                            options={pageSizeOptions}
                            value={String(prefs.paginationState.pageSize)}
                            onchoose={(value) => {
                                prefs.paginationState.pageSize = Number(value)
                                page(0)
                            }}
                        />
                    </div>
                </div>
                <span>
                    {t('common.message.table-range', {
                        first: firstRow.toLocaleString(locale.language),
                        last: lastRow.toLocaleString(locale.language),
                        total: total.toLocaleString(locale.language)
                    })}
                </span>
                <div class="page-controls">
                    <ActionIcon
                        icon="PiCaretLeft"
                        label={t('create-node-modal.widget.back')}
                        size={28}
                        iconSize={16}
                        subtle
                        disabled={prefs.paginationState.pageIndex === 0}
                        onclick={() => page(prefs.paginationState.pageIndex - 1)}
                    />
                    <ActionIcon
                        icon="PiCaretRight"
                        label={t('common.action.next')}
                        size={28}
                        iconSize={16}
                        subtle
                        disabled={prefs.paginationState.pageIndex + 1 >= pages}
                        onclick={() => page(prefs.paginationState.pageIndex + 1)}
                    />
                </div>
            </div>
        {/if}
    </div>
</section>

{#if selectedUser !== undefined}
    {#await import('../users/UserDialog.svelte') then { default: UserDialog }}
        <UserDialog id={selectedUser} close={() => (selectedUser = undefined)} />
    {/await}
{/if}

<style>
    .inspector-panel {
        min-width: 0;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
    }
    .heading-controls,
    .row-controls {
        display: flex;
    }
    .heading-controls :global(.action-icon:not(:first-child)),
    .row-controls :global(.action-icon:not(:first-child)) {
        margin-inline-start: -1px;
        border-start-start-radius: 0;
        border-end-start-radius: 0;
    }
    .heading-controls :global(.action-icon:not(:last-child)),
    .row-controls :global(.action-icon:not(:last-child)) {
        border-start-end-radius: 0;
        border-end-end-radius: 0;
    }
    .table-panel {
        position: relative;
    }
    .toolbar {
        display: flex;
        justify-content: flex-end;
        align-items: flex-start;
        gap: 2px;
        min-height: 56px;
        padding: 8px;
    }
    .column-menu {
        position: absolute;
        inset-inline-end: 8px;
        top: 44px;
        z-index: 5;
        max-height: 60vh;
        max-width: 95%;
        overflow: auto;
        padding: var(--mantine-spacing-sm);
        background: var(--gw-surface);
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-sm);
    }
    .empty {
        color: var(--mantine-color-dimmed);
        text-align: center;
        margin: 0;
        padding: 32px;
        min-height: 102px;
        font-style: italic;
        background: var(--mantine-color-body);
    }
    .column-filter {
        display: flex;
        align-items: center;
        gap: 4px;
    }
    .column-filter input {
        min-width: 0;
        width: 100%;
        font: inherit;
        font-weight: 400;
        color: inherit;
        border: 0;
        min-height: 36px;
        border-bottom: 2px solid var(--mantine-color-default-border);
        border-radius: 0;
        padding: 0 1px;
        background: transparent;
    }
    .column-filter input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    .column-filter-select {
        display: flex;
        flex: 1;
        min-width: 0;
    }
    .column-filter-select :global(.select-field) {
        flex: 1;
    }
    .column-filter-select :global(.select-input) {
        --input-height: 36px;
        border: 0;
        border-radius: 0;
        background: transparent;
    }
    .column-filter-select :global(.mantine-Input-input) {
        height: 36px;
        min-height: 36px;
        padding-inline: 1px 60px;
        border: 0;
        border-bottom: 2px solid var(--mantine-color-default-border);
        border-radius: 0;
        background: transparent;
        color: inherit;
        font: inherit;
        font-weight: 400;
    }
    .column-filter-select :global(.mantine-Input-input::placeholder) {
        color: var(--mantine-color-placeholder);
    }
    .column-filter-select :global(.clear[data-position="right"]) {
        inset-inline-end: 21px;
    }
    .column-filter-select :global(.arrow[data-position="right"]) {
        inset-inline-end: 1px;
    }
    .column-heading {
        display: flex;
        position: relative;
        justify-content: space-between;
        align-items: center;
        width: 100%;
        min-height: 24px;
        min-width: 0;
        gap: 4px;
        top: 1px;
    }
    .sorted :global(svg) {
        color: var(--mantine-color-cyan-7);
    }
    .column-heading > button:not(.column-options) {
        flex: 1 1 0;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .column-label {
        min-width: 0;
        overflow: hidden;
        text-overflow: clip;
        white-space: nowrap;
    }
    .column-heading > button:not(.column-options) :global(svg) {
        flex: 0 0 16px;
    }
    .column-heading::after {
        position: absolute;
        inset-inline-end: -6px;
        width: 4px;
        height: 24px;
        border-radius: var(--mantine-radius-xs);
        background: light-dark(var(--mantine-color-gray-5), var(--mantine-color-dark-3));
        content: "";
    }
    .column-options {
        flex: 0 0 22px;
        width: 22px;
        height: 22px;
        color: var(--mantine-color-dimmed);
        margin-inline-end: 0;
    }
    :global(.action-icon.ip-info-action.tinted) {
        border-color: transparent;
        background: var(--mantine-color-grape-light);
    }
    th button {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        min-height: 0;
        padding: 0;
        border: 0;
        font: inherit;
    }
    .pagination {
        position: relative;
        justify-content: flex-end;
        padding: var(--mantine-spacing-xs) 12px;
        border-top: 0;
        gap: 20px;
    }
    .pagination::before {
        position: absolute;
        inset-block-start: 0;
        inset-inline: 0;
        height: 1px;
        background: var(--gw-border);
        content: "";
    }
    .rows-label {
        display: flex;
        align-items: center;
    }
    .rows-per-page,
    .page-controls {
        display: flex;
        align-items: center;
    }
    .rows-per-page {
        gap: 10px;
    }
    .page-controls {
        gap: 6px;
    }
    .page-size-select {
        flex: none;
        width: 80px;
    }
    .page-size-select :global(.select-field) {
        flex: none;
    }

    th button,
    .cell {
        background: transparent;
        color: inherit;
    }
    .pagination {
        align-items: center;
    }
    .check {
        display: flex;
        align-items: center;
    }
    ul {
        list-style: none;
        padding: 0;
    }
    li {
        margin-block: 0.25rem;
        align-items: center;
    }
    .scroll {
        overflow: auto;
        max-height: 62vh;
    }
    section:fullscreen {
        padding: 1rem;
        overflow: auto;
        background: var(--gw-surface);
    }
    table {
        min-width: 100%;
        font-size: var(--mantine-font-size-sm);
        border-collapse: separate;
        border-spacing: 0;
        table-layout: fixed;
        white-space: nowrap;
    }
    th,
    td {
        text-align: start;
        padding: 0 6px;
        border-block-end: 1px solid var(--gw-border);
    }
    th {
        position: sticky;
        vertical-align: top;
        padding-block: 8px 11px;
        top: 0;
        z-index: 2;
        background: var(--gw-card-bg);
    }
    td.pinned {
        position: sticky;
        z-index: 1;
        background: var(--gw-surface);
    }
    td {
        padding-block: 6px;
    }
    .cell {
        border: 0;
        padding: 0.25rem;
        min-height: 1.75rem;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .timestamp .cell {
        font-family: var(--mantine-font-family-monospace);
    }
    td.centered {
        text-align: center;
    }
    .monospace .cell {
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-md);
        font-weight: 500;
    }
    .cell-prefix {
        flex: none;
        font-family: var(--mantine-font-family);
        font-size: calc(var(--mantine-font-size-sm) * 1.1);
        line-height: 1;
    }
    .cell.prefixed {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
    }
    .cell-value {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .compact td {
        padding-block: 1.5px;
    }
    .compact tbody tr:first-child td {
        padding-block: 2px;
    }
    .compact tbody:not(:has(tr:nth-child(2))) tr:first-child td {
        padding-block: 1.5px;
    }
</style>
