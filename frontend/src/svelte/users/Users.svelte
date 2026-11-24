<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { tick, untrack } from 'svelte'
    import { MediaQuery } from 'svelte/reactivity'
    import { externalSquadsQuery } from '../external-squads/queries'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import { nodesQuery } from '../nodes/queries'
    import { systemStatsQuery, userMetrics } from '../system-stats'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import MetricCard from '../ui/MetricCard.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Switch from '../ui/Switch.svelte'
    import TablePopup from '../ui/TablePopup.svelte'
    import TableTitle from '../ui/TableTitle.svelte'
    import BulkUserActions from './BulkUserActions.svelte'
    import { orderUserColumns, userColumns, visibleUserColumns } from './columns'
    import MobileTableNotice from './MobileTableNotice.svelte'
    import { usersQuery, userTagsQuery } from './queries'
    import { togglePageSelection, userSelection } from './selection.svelte'
    import {
        applyUserTable,
        captureUserTable,
        readUserTable,
        readUserTemplates,
        type TableDensity,
        tableDensities,
        type UserTableTemplate,
        writeUserTable,
        writeUserTemplates
    } from './table-state'
    import UserDialog from './UserDialog.svelte'
    import UserForm from './UserForm.svelte'
    import UserRows from './UserRows.svelte'
    import UserTableChoice from './UserTableChoice.svelte'
    import UserTableViews from './UserTableViews.svelte'

    let storage: Storage | undefined
    try {
        storage = window.localStorage
    } catch {
        /* Browser storage is optional. */
    }

    const narrow = new MediaQuery('(max-width: 500px)')
    const initialModes = { hwidDeviceLimit: 'equals', tag: 'equals', trafficLimitBytes: 'between' }
    const pageSizes = [5, 10, 15, 20, 25, 30, 50, 100]
    let tableState = $state.raw(readUserTable(storage))
    let modes = $state.raw<Record<string, string>>({ ...initialModes })
    let templates = $state.raw(readUserTemplates(storage))
    let storageMessage = $state('')
    const densityIcons = {
        xxs: 'TbBaselineDensitySmall',
        xl: 'TbBaselineDensityLarge',
        md: 'TbBaselineDensityMedium',
        xs: 'TbBaselineDensitySmall'
    } satisfies Record<TableDensity, IconName>
    const usernameModes = [
        ['contains', 'Contains'],
        ['equals', 'Equals'],
        ['startsWith', 'Starts with'],
        ['endsWith', 'Ends with']
    ] as const
    const popupId = $props.id()
    let density = $state<TableDensity>('xxs')
    let columnMenu = $state<{
        column: ReturnType<typeof userColumns>[number]
        anchor: HTMLElement
    }>()
    let modeAnchor = $state<HTMLElement>()
    let columnsAnchor = $state<HTMLElement>()
    let draggedColumn = $state<string>()
    let creating = $state(false)
    let editing = $state.raw<{ id: number }>()
    let bulk = $state<{ scope: 'selected' | 'all'; ids: number[]; editOnly?: boolean }>()
    let fullscreen = $state(false)
    let showColumnList = $state(false)
    let panel: HTMLDivElement
    let t = $derived(locale.t)

    const users = createQuery(() => usersQuery(tableState, modes, userSelection.size === 0))
    const stats = createQuery(() => systemStatsQuery)
    let metrics = $derived(stats.data ? userMetrics(stats.data.users, t) : [])
    const tags = createQuery(() => userTagsQuery)
    const nodes = createQuery(() => nodesQuery)
    const squads = createQuery(() => externalSquadsQuery)
    let nodeNames = $derived(new Map(nodes.data?.map((node) => [node.uuid, node.name]) ?? []))
    let squadNames = $derived(
        new Map(squads.data?.externalSquads.map((squad) => [squad.uuid, squad.name]) ?? [])
    )
    let columns = $derived(userColumns(t, locale.language, nodeNames, squadNames))
    let ordered = $derived(orderUserColumns(columns, tableState.columnOrder))
    let shown = $derived(visibleUserColumns(columns, tableState))
    let { pageIndex, pageSize } = $derived(tableState.paginationState)
    let rows = $derived(users.data?.users ?? [])
    let total = $derived(users.data?.total ?? 0)
    let pageCount = $derived(Math.max(1, Math.ceil(total / pageSize)))
    let rowsKey = $derived(
        JSON.stringify([tableState.paginationState, tableState.columnFilter, tableState.sorting])
    )

    $effect(() => {
        if (!users.isPlaceholderData && users.isSuccess && pageIndex >= pageCount) {
            goToPage(pageCount - 1)
        }
    })

    function storeTable(): void {
        if (!storage) return
        try {
            writeUserTable(storage, tableState)
        } catch {
            storageMessage = 'Unable to save table preferences in this browser.'
        }
    }

    function goToPage(index: number): void {
        tableState = {
            ...tableState,
            paginationState: { ...tableState.paginationState, pageIndex: index }
        }
        storeTable()
    }

    function changePageSize(size: number): void {
        tableState = { ...tableState, paginationState: { pageIndex: 0, pageSize: size } }
        storeTable()
    }

    function changeSort(id: string, append: boolean): void {
        const current = tableState.sorting.find((item) => item.id === id)
        const sorting = append ? tableState.sorting.filter((item) => item.id !== id) : []
        if (!current?.desc) sorting.push({ id, desc: current !== undefined })
        setSorting(sorting)
    }

    function setSorting(sorting: typeof tableState.sorting): void {
        tableState = {
            ...tableState,
            sorting,
            paginationState: { ...tableState.paginationState, pageIndex: 0 }
        }
        storeTable()
    }

    function changeFilter(id: string, value: string | string[]): void {
        const filters = tableState.columnFilter.filter((item) => item.id !== id)
        if (value.length) filters.push({ id, value })
        setFilters(filters)
    }

    function setFilters(columnFilter: typeof tableState.columnFilter): void {
        tableState = {
            ...tableState,
            columnFilter,
            paginationState: { ...tableState.paginationState, pageIndex: 0 }
        }
        storeTable()
    }

    function filterFor(id: string): string[] {
        const value = tableState.columnFilter.find((item) => item.id === id)?.value
        if (Array.isArray(value))
            return value.filter((item): item is string => typeof item === 'string')
        if (typeof value === 'string' || typeof value === 'number') return [String(value)]
        return []
    }

    function optionsFor(filter: string): { value: string; label: string; flag?: string }[] {
        if (filter === 'node') {
            return (nodes.data ?? []).map(({ uuid, name, countryCode }) => ({
                value: uuid,
                label: name,
                flag: flag(countryCode)
            }))
        }
        const values =
            filter === 'status'
                ? ['ACTIVE', 'DISABLED', 'EXPIRED', 'LIMITED']
                : (tags.data?.tags ?? [])
        return values.map((value) => ({ value, label: value }))
    }

    function moveColumn(from: number, to: number): void {
        const order = ordered.map((column) => column.id)
        const current = order[from]
        if (current === undefined || order[to] === undefined || from === to) return
        order.splice(from, 1)
        order.splice(to, 0, current)
        tableState = { ...tableState, columnOrder: order }
        storeTable()
    }

    function setColumnVisibility(id: string, visible: boolean): void {
        tableState = {
            ...tableState,
            columnVisibility: { ...tableState.columnVisibility, [id]: visible }
        }
        storeTable()
    }

    function showColumns(visible: boolean): void {
        const columnVisibility = Object.fromEntries(columns.map(({ id }) => [id, visible]))
        tableState = { ...tableState, columnVisibility }
        storeTable()
    }

    function resizeColumn(id: string, width: number | undefined, persist: boolean): void {
        const columnSize = { ...tableState.columnSize }
        if (width === undefined) {
            delete columnSize[id]
        } else {
            const min = id === 'username' || id === 'usedTrafficBytes' ? 150 : 75
            const max = id === 'username' ? 300 : id === 'usedTrafficBytes' ? 700 : 1000
            columnSize[id] = Math.round(Math.min(max, Math.max(min, width)))
        }
        tableState = { ...tableState, columnSize }
        if (persist) storeTable()
    }

    function showFilters(visible: boolean): void {
        tableState = { ...tableState, showColumnFilters: visible }
        storeTable()
    }

    async function focusFilter(id: string): Promise<void> {
        if (!tableState.showColumnFilters) showFilters(true)
        await tick()
        document.getElementById(`user-filter-${id}`)?.focus()
    }

    function pinFor(id: string): '' | 'left' | 'right' {
        if (tableState.columnPinning.left.includes(id)) return 'left'
        if (tableState.columnPinning.right.includes(id)) return 'right'
        return ''
    }

    function pinColumn(id: string, position: string): void {
        const left = tableState.columnPinning.left.filter((item) => item !== id)
        const right = tableState.columnPinning.right.filter((item) => item !== id)
        if (position === 'left') left.push(id)
        if (position === 'right') right.push(id)
        tableState = { ...tableState, columnPinning: { left, right } }
        storeTable()
    }

    function storeTemplates(): void {
        if (!storage) return
        try {
            writeUserTemplates(storage, templates)
        } catch {
            storageMessage = 'Unable to save views in this browser.'
        }
    }

    function saveTemplate(name: string, color: string): void {
        const id = globalThis.crypto.randomUUID?.() ?? String(Date.now())
        templates = [
            ...templates,
            { id, name, color, snapshot: captureUserTable(tableState, modes) }
        ]
        storeTemplates()
    }

    function loadTemplate(snapshot: UserTableTemplate['snapshot']): void {
        tableState = applyUserTable(tableState, snapshot)
        modes = { ...snapshot.columnFilterFns }
        storeTable()
    }

    function removeTemplate(id: string): void {
        templates = templates.filter((template) => template.id !== id)
        storeTemplates()
    }

    function toggleFullscreen(): void {
        void (fullscreen ? document.exitFullscreen() : panel.requestFullscreen()).catch(
            (error: unknown) => {
                storageMessage =
                    error instanceof Error ? error.message : 'Fullscreen is unavailable.'
            }
        )
    }
</script>

<svelte:head><title>{t('constants.users')} | Gowave</title></svelte:head>
<svelte:document
    onfullscreenchange={() => {
        fullscreen = document.fullscreenElement === panel
    }}
/>

{#snippet headerFilter(column: (typeof shown)[number])}
    {@const id = `user-filter-${column.id}`}
    {@const selected = filterFor(column.id)}
    {#if column.filter === 'status' || column.filter === 'tag' || column.filter === 'node'}
        <UserTableChoice
            {id}
            label={`Filter ${column.label}`}
            options={optionsFor(column.filter)}
            {selected}
            multiple={column.filter === 'tag'}
            searchable
            change={(value) => changeFilter(column.id, value)}
        />
    {:else}
        <div class="filter-text">
            <input
                class="mantine-Input-input"
                {id}
                type="search"
                aria-label={`Filter ${column.label}`}
                placeholder="Filter by..."
                value={selected[0] ?? ''}
                onchange={(event) => changeFilter(column.id, event.currentTarget.value.trim())}
            >
            {#if column.id === 'username'}
                <ActionIcon
                    icon="TbFilterCog"
                    label="Change username filter mode"
                    size={25}
                    iconSize={21}
                    subtle
                    aria-haspopup="menu"
                    aria-expanded={Boolean(modeAnchor)}
                    onclick={(event) => {
                        modeAnchor = modeAnchor ? undefined : event.currentTarget
                    }}
                />
            {/if}
        </div>
    {/if}
{/snippet}

{#snippet columnAction(
    icon: IconName,
    label: string,
    action: () => void,
    disabled = false,
    rotation = 0
)}
    <button
        type="button"
        role="menuitem"
        class="column-action m_99ac2aa1 mantine-UnstyledButton-root"
        {disabled}
        onclick={() => {
            const anchor = columnMenu?.anchor
            columnMenu = undefined
            anchor?.focus({ preventScroll: true })
            action()
        }}
    >
        <span class="menu-icon" style:rotate={`${rotation}deg`}
            ><Icon name={icon} size={24} /></span
        >
        {label}
    </button>
{/snippet}

{#each columnMenu ? [columnMenu] : [] as entry (entry.column.id)}
    {@const column = entry.column}
    {@const id = column.id}
    {@const sort = tableState.sorting.find((item) => item.id === id)}
    {@const pin = pinFor(id)}
    <TablePopup
        anchor={entry.anchor}
        label={`Column actions for ${column.label}`}
        id={`${popupId}-column`}
        close={() => {
            if (columnMenu === entry) columnMenu = undefined
        }}
    >
        {#if column.sortable}
            {@render columnAction(
                'TbEraser',
                'Clear sort',
                () => setSorting(tableState.sorting.filter((item) => item.id !== id)),
                !sort
            )}
            {@render columnAction(
                'TbSortAscending',
                `Sort by ${column.label} ascending`,
                () => setSorting([{ id, desc: false }]),
                sort?.desc === false
            )}
            {@render columnAction(
                'TbSortDescending',
                `Sort by ${column.label} descending`,
                () => setSorting([{ id, desc: true }]),
                sort?.desc === true
            )}
            <hr class="column-divider m_efdf90cb">
        {/if}
        {#if column.filter}
            {@render columnAction(
                'TbFilterOff',
                'Clear filter',
                () => changeFilter(id, ''),
                !filterFor(id).length
            )}
            {@render columnAction(
                'TbFilter',
                `Filter by ${column.label}`,
                () => void focusFilter(id)
            )}
            <hr class="column-divider m_efdf90cb">
        {/if}
        {@render columnAction(
            'TbPinned',
            'Pin to left',
            () => pinColumn(id, 'left'),
            pin === 'left',
            90
        )}
        {@render columnAction(
            'TbPinned',
            'Pin to right',
            () => pinColumn(id, 'right'),
            pin === 'right',
            -90
        )}
        {@render columnAction('TbPinnedOff', 'Unpin', () => pinColumn(id, ''), !pin)}
        <hr class="column-divider m_efdf90cb">
        {@render columnAction(
            'TbArrowAutofitContent',
            'Reset column size',
            () => resizeColumn(id, undefined, true),
            tableState.columnSize[id] === undefined
        )}
        {@render columnAction('TbEyeOff', `Hide ${column.label} column`, () =>
            setColumnVisibility(id, false)
        )}
        {@render columnAction(
            'TbColumns',
            'Show all columns',
            () => showColumns(true),
            !Object.values(tableState.columnVisibility).includes(false)
        )}
    </TablePopup>
{/each}

{#if modeAnchor}
    <TablePopup
        anchor={modeAnchor}
        label="Username filter mode"
        id={`${popupId}-mode`}
        close={() => (modeAnchor = undefined)}
    >
        {#each usernameModes as [value, label] (value)}
            {@const selected = (modes['username'] ?? 'contains') === value}
            <button
                type="button"
                role="menuitemradio"
                aria-checked={selected}
                class="column-action m_99ac2aa1 mantine-UnstyledButton-root"
                onclick={() => {
                    modes = { ...modes, username: value }
                    modeAnchor?.focus({ preventScroll: true })
                    modeAnchor = undefined
                }}
            >
                <span class="menu-icon"
                    >{#if selected}
                        <Icon name="TbCheck" size={16} />
                    {/if}</span
                >
                {label}
            </button>
        {/each}
    </TablePopup>
{/if}

<main class="users-page">
    {#if narrow.current}
        <MobileTableNotice />
    {/if}
    {#if metrics.length}
        <div class="metrics">
            {#each metrics as metric (metric.label)}
                <MetricCard {...metric} />
            {/each}
        </div>
    {/if}
    <div class="users-panel" bind:this={panel}>
        <TableTitle title={t('constants.users')} icon="PiUsersDuotone">
            <div class="action-group">
                <UserTableViews
                    {templates}
                    onsave={saveTemplate}
                    onapply={loadTemplate}
                    onremove={removeTemplate}
                />
                <ActionIcon
                    icon="TbFilterOff"
                    tone="gray"
                    label={t('action-group.feature.clear-filters')}
                    disabled={users.isFetching}
                    onclick={() => setFilters([])}
                />
                <ActionIcon
                    icon="TbRestore"
                    tone="gray"
                    label={t('action-group.feature.reset-table')}
                    disabled={users.isFetching}
                    onclick={() => {
                        tableState = readUserTable()
                        modes = { ...initialModes }
                        storeTable()
                    }}
                />
            </div>
            <div class="action-group">
                <ActionIcon
                    icon="TbRefresh"
                    tone="cyan"
                    label={t('common.action.update')}
                    onclick={() => void users.refetch()}
                    disabled={users.isFetching}
                />
                <ActionIcon
                    icon="TbPlus"
                    tone="teal"
                    label={t('common.action.create')}
                    onclick={() => (creating = true)}
                    disabled={fullscreen}
                />
            </div>
        </TableTitle>
        <div class="table-controls">
            {#if userSelection.size}
                <div class="selection-summary">
                    <div class="selection-tools">
                        <span class="selected-count m_347db0ec mantine-Badge-root" role="status">
                            {t('common.message.selected', { count: userSelection.size })}
                        </span>
                        <div class="selection-shortcuts">
                            <ActionIcon
                                icon="TbSelectAll"
                                label={t('common.action.select-all')}
                                size={34}
                                iconSize={20}
                                subtle
                                disabled={users.isPlaceholderData || rows.length === 0}
                                onclick={() => togglePageSelection(rows)}
                            />
                            <ActionIcon
                                icon="TbX"
                                label={t('common.action.clear-selection')}
                                size={28}
                                iconSize={23}
                                subtle
                                onclick={() => userSelection.clear()}
                            />
                        </div>
                    </div>
                    <div class="selection-actions">
                        <Button
                            variant="soft"
                            color="green"
                            icon="TbBolt"
                            disabled={fullscreen}
                            onclick={() => (bulk = { scope: 'selected', ids: [...userSelection] })}
                            >{t('common.action.actions')}</Button
                        >
                        <Button
                            variant="soft"
                            color="red"
                            icon="TbEdit"
                            disabled={fullscreen}
                            onclick={() =>
                                (bulk = {
                                    scope: 'selected',
                                    ids: [...userSelection],
                                    editOnly: true
                                })}
                            >{t('common.action.update')}</Button
                        >
                    </div>
                </div>
            {/if}
            <div class="table-toolbar">
                <div class="action-group">
                    <ActionIcon
                        icon="TbBolt"
                        size={34}
                        iconSize={20}
                        tone="green"
                        label="Bulk actions for all users"
                        onclick={() => (bulk = { scope: 'all', ids: [] })}
                        disabled={fullscreen}
                    />
                    <ActionIcon
                        icon="TbEdit"
                        size={34}
                        iconSize={20}
                        tone="red"
                        label={t('common.action.bulk-edit')}
                        onclick={() => (bulk = { scope: 'all', ids: [], editOnly: true })}
                        disabled={fullscreen}
                    />
                </div>
                <ActionIcon
                    icon={densityIcons[density]}
                    size={34}
                    iconSize={24}
                    subtle
                    label="Adjust row spacing density"
                    onclick={() => {
                        density =
                            tableDensities[
                                (tableDensities.indexOf(density) + 1) % tableDensities.length
                            ] ?? 'xxs'
                    }}
                />
                {#if typeof document.documentElement.requestFullscreen === 'function'}
                    <ActionIcon
                        icon={fullscreen ? 'PiArrowsIn' : 'TbMaximize'}
                        size={34}
                        iconSize={24}
                        subtle
                        label={fullscreen ? 'Exit fullscreen' : 'Fullscreen'}
                        onclick={toggleFullscreen}
                    />
                {/if}
                <ActionIcon
                    icon="TbColumns"
                    size={34}
                    iconSize={24}
                    subtle
                    label="Show/Hide columns"
                    aria-haspopup="menu"
                    aria-expanded={showColumnList}
                    aria-controls={showColumnList ? `${popupId}-columns` : undefined}
                    onclick={(event) => {
                        columnsAnchor = event.currentTarget
                        showColumnList = !showColumnList
                    }}
                />
            </div>
        </div>
        {#if showColumnList && columnsAnchor}
            <TablePopup
                anchor={columnsAnchor}
                label="Show/Hide columns"
                focusOnOpen
                keyboardNavigation={false}
                id={`${popupId}-columns`}
                wide
                close={() => (showColumnList = false)}
            >
                <div class="column-controls">
                    <Button
                        variant="subtle"
                        disabled={!shown.length}
                        onclick={() => showColumns(false)}
                        >Hide all</Button
                    >
                    <Button
                        variant="subtle"
                        onclick={() => {
                            tableState = { ...tableState, columnOrder: [] }
                            storeTable()
                        }}
                        >Reset order</Button
                    >
                    <Button
                        variant="subtle"
                        disabled={!tableState.columnPinning.left.length &&
                            !tableState.columnPinning.right.length}
                        onclick={() => {
                            tableState = { ...tableState, columnPinning: { left: [], right: [] } }
                            storeTable()
                        }}
                        >Unpin all</Button
                    >
                    <Button
                        variant="subtle"
                        disabled={shown.length === columns.length}
                        onclick={() => showColumns(true)}
                        >Show all</Button
                    >
                </div>
                <hr class="column-divider m_efdf90cb">
                <ul class="column-options">
                    <li class="selection-option">
                        <div class="column-pins"></div>
                        <Switch label="Select" switchSize="sm" showLabel checked disabled />
                    </li>
                    {#each ordered as column, index (column.id)}
                        {@const pin = pinFor(column.id)}
                        <li
                            ondragover={(event) => {
                                if (draggedColumn) event.preventDefault()
                            }}
                            ondrop={(event) => {
                                event.preventDefault()
                                if (draggedColumn)
                                    moveColumn(
                                        ordered.findIndex(({ id }) => id === draggedColumn),
                                        index
                                    )
                                draggedColumn = undefined
                            }}
                        >
                            <ActionIcon
                                icon="TbGripHorizontal"
                                label={`Reorder ${column.label}`}
                                size={28}
                                iconSize={22}
                                subtle
                                draggable="true"
                                ondragstart={(event) => {
                                    draggedColumn = column.id
                                    event.dataTransfer?.setData('text/plain', column.id)
                                }}
                                ondragend={() => (draggedColumn = undefined)}
                                onkeydown={(event) => {
                                    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
                                    event.preventDefault()
                                    moveColumn(index, index + (event.key === 'ArrowUp' ? -1 : 1))
                                }}
                            />
                            <div class="column-pins">
                                {#if pin}
                                    <ActionIcon
                                        icon="TbPinnedOff"
                                        label={`Unpin ${column.label}`}
                                        size={28}
                                        iconSize={24}
                                        subtle
                                        onclick={() => pinColumn(column.id, '')}
                                    />
                                {:else}
                                    <ActionIcon
                                        icon="TbPinned"
                                        label={`Pin ${column.label} to left`}
                                        size={28}
                                        iconSize={24}
                                        subtle
                                        style="rotate: 90deg"
                                        onclick={() => pinColumn(column.id, 'left')}
                                    />
                                    <ActionIcon
                                        icon="TbPinned"
                                        label={`Pin ${column.label} to right`}
                                        size={28}
                                        iconSize={24}
                                        subtle
                                        style="rotate: -90deg"
                                        onclick={() => pinColumn(column.id, 'right')}
                                    />
                                {/if}
                            </div>
                            <Switch
                                label={column.label}
                                switchSize="sm"
                                tone="cyan"
                                showLabel
                                checked={tableState.columnVisibility[column.id] !== false}
                                onchange={(event) =>
                                    setColumnVisibility(column.id, event.currentTarget.checked)}
                            />
                        </li>
                    {/each}
                </ul>
                <div class="column-options-footer">
                    <Switch
                        label="Show column filters"
                        switchSize="sm"
                        tone="cyan"
                        showLabel
                        checked={tableState.showColumnFilters}
                        onchange={(event) => showFilters(event.currentTarget.checked)}
                    />
                </div>
            </TablePopup>
        {/if}

        {#if storageMessage}
            <p role="alert">{storageMessage}</p>
        {/if}
        {#if users.isPending}
            <p class="table-message" role="status">{t('common.message.loading')}</p>
        {:else if users.isError}
            <p class="table-message" role="alert">{users.error.message}</p>
            <button type="button" onclick={() => void users.refetch()}>
                {t('common.action.try-again')}
            </button>
        {:else}
            <UserRows
                {rows}
                columns={shown}
                sorting={tableState.sorting}
                busy={users.isPlaceholderData}
                {rowsKey}
                {density}
                onsort={changeSort}
                onopen={(id) => (editing = { id })}
                oncolumn={(id, anchor) => {
                    const column = columns.find((item) => item.id === id)
                    columnMenu =
                        column && columnMenu?.column.id !== id ? { column, anchor } : undefined
                }}
                onresize={resizeColumn}
                activeColumn={columnMenu?.column.id}
                columnMenuId={`${popupId}-column`}
                headerFilter={tableState.showColumnFilters ? headerFilter : undefined}
            />
            {#if !rows.length}
                <p class="table-message">{t('common.message.nothing-found')}</p>
            {/if}
            <footer class="pagination">
                <div class="page-size">
                    <span>Rows per page</span>
                    <SelectField
                        label="Rows per page"
                        placeholder=""
                        options={(pageSizes.includes(pageSize)
                            ? pageSizes
                            : [pageSize, ...pageSizes]
                        ).map((size) => ({
                            value: String(size),
                            label: String(size)
                        }))}
                        value={String(pageSize)}
                        searchable={false}
                        clearable={false}
                        hideLabel
                        onchoose={(value) => changePageSize(Number(value))}
                    />
                </div>
                <span role="status"
                    >{total ? pageIndex * pageSize + 1 : 0}-{Math.min(
                        (pageIndex + 1) * pageSize,
                        total
                    )}
                    of {total}</span
                >
                <div class="pagination-actions">
                    <ActionIcon
                        icon="PiCaretLeft"
                        size={28}
                        iconSize={18}
                        label="Previous"
                        disabled={pageIndex === 0 || users.isPlaceholderData}
                        onclick={() => goToPage(pageIndex - 1)}
                    />
                    <ActionIcon
                        icon="PiCaretRight"
                        size={28}
                        iconSize={18}
                        label={t('common.action.next')}
                        disabled={pageIndex + 1 >= pageCount || users.isPlaceholderData}
                        onclick={() => goToPage(pageIndex + 1)}
                    />
                </div>
            </footer>
        {/if}
    </div>
</main>

{#key editing}
    <!-- A native close event can arrive after another editor opens. -->
    {@const opened = untrack(() => editing)}
    {#if opened}
        <UserDialog
            id={opened.id}
            close={() => {
                if (editing === opened) editing = undefined
            }}
        />
    {/if}
{/key}
{#if creating}
    <UserForm
        user={null}
        onclose={() => {
            creating = false
        }}
    />
{/if}
{#if bulk}
    <BulkUserActions
        target={bulk.scope}
        ids={bulk.ids}
        editOnly={bulk.editOnly ?? false}
        onclose={() => {
            bulk = undefined
        }}
    />
{/if}

<style>
    .users-page {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-md);
    }

    .metrics {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-xs);
    }

    @media (min-width: 30em) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }

    @media (min-width: 80em) {
        .metrics {
            grid-template-columns: repeat(5, minmax(0, 1fr));
        }
    }

    .users-panel {
        position: relative;
        min-width: 0;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
    }

    .users-panel:fullscreen {
        overflow: auto;
    }

    .table-controls {
        position: relative;
        background: var(--gw-card-bg);
    }

    .selection-summary,
    .selection-tools,
    .selection-shortcuts,
    .selection-actions {
        display: flex;
        align-items: center;
    }

    .selection-summary {
        position: absolute;
        inset: 0 0 2px;
        gap: var(--mantine-spacing-md);
        padding: 8px 20px;
        background: var(--mantine-color-blue-light);
    }

    .selection-tools {
        gap: var(--mantine-spacing-md);
    }

    .selection-actions {
        gap: var(--mantine-spacing-xs);
    }

    .selected-count {
        --badge-bg: var(--mantine-color-gray-light);
        --badge-color: var(--mantine-color-gray-light-color);
        --badge-height: var(--badge-height-lg);
        --badge-fz: var(--badge-fz-lg);
        --badge-padding-x: var(--badge-padding-x-lg);
        --badge-radius: var(--mantine-radius-md);
    }

    @media (max-width: 720px) {
        .selection-summary {
            position: relative;
            inset: auto;
            justify-content: space-between;
            flex-wrap: wrap;
        }
    }

    .table-toolbar {
        display: flex;
        align-items: flex-start;
        justify-content: end;
        min-height: 56px;
        padding: 8px;
        pointer-events: none;
    }

    .table-toolbar :global(.action-icon) {
        position: relative;
        pointer-events: auto;
    }

    .action-group {
        display: flex;
    }

    .action-group :global(.action-icon:not(:first-child)) {
        margin-inline-start: -1px;
        border-start-start-radius: 0;
        border-end-start-radius: 0;
    }

    .action-group :global(.action-icon:not(:last-child)) {
        border-start-end-radius: 0;
        border-end-end-radius: 0;
    }

    .column-action {
        gap: var(--mantine-spacing-xs);
        white-space: nowrap;
    }

    .column-divider {
        border-inline: 0;
        border-block-end: 0;
    }

    .menu-icon {
        display: inline-grid;
        min-width: 24px;
        place-items: center;
    }

    .column-controls {
        display: flex;
        justify-content: space-between;
        gap: 6px;
        padding: 4px 0;
    }

    .column-options {
        width: min(438px, calc(100vw - 20px));
        margin: 0;
        padding: 0;
        list-style: none;
    }

    .column-options li,
    .column-pins,
    .column-options-footer {
        display: flex;
        align-items: center;
    }

    .column-options li {
        min-height: 40px;
        gap: 8px;
        padding-inline: 12px;
    }

    .column-options li:hover {
        background: var(--mantine-color-default-hover);
    }

    .column-pins {
        width: 70px;
        flex: none;
        justify-content: center;
    }

    .selection-option .column-pins {
        margin-inline-start: 36px;
    }

    @media (max-width: 500px) {
        .column-controls :global(.mantine-Button-root) {
            padding-inline: 6px;
        }
    }

    .column-options-footer {
        padding: 8px;
    }

    .filter-text {
        display: flex;
        height: 36px;
        align-items: center;
        gap: 4px;
        font-weight: 400;
    }

    .filter-text input {
        width: 100%;
        min-width: 0;
        height: 36px;
        min-height: 35px;
        padding: 0 0 2px;
        border: 0;
        border-bottom: 2px solid var(--mantine-color-default-border);
        border-radius: 0;
        outline-offset: -2px;
        background: transparent;
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
    }

    .filter-text input::placeholder {
        color: var(--mantine-color-placeholder);
    }

    .pagination,
    .page-size,
    .pagination-actions {
        display: flex;
        align-items: center;
    }

    .pagination {
        justify-content: end;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        border-top: 1px solid var(--mantine-color-default-border);
    }

    .page-size {
        gap: var(--mantine-spacing-xs);
    }

    .page-size :global(.select-field) {
        width: 80px;
        flex: none;
    }

    .pagination-actions {
        gap: 6px;
    }

    .pagination-actions :global(.action-icon) {
        border-color: transparent;
    }

    .table-message {
        margin: 0;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-md);
    }

    @media (max-width: 40em) {
        .pagination {
            flex-wrap: wrap;
        }
    }
</style>
