<script lang="ts">
    import { fixedVirtualWindow } from '@shared/lib/virtual-layout'
    import {
        getConnectionStatusColorUtil,
        getExpirationTextUtil,
        getTimeAgoUtil
    } from '@shared/utils/time-utils'
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import Icon from '../ui/Icon.svelte'
    import type { visibleUserColumns } from './columns'
    import type { User } from './queries'
    import { togglePageSelection, userSelection } from './selection.svelte'
    import type { TableDensity, UserTableState } from './table-state'
    import UserStatusBadge from './UserStatusBadge.svelte'
    import UserTrafficCell from './UserTrafficCell.svelte'

    type VisibleColumn = ReturnType<typeof visibleUserColumns>[number]

    let {
        rows,
        columns,
        sorting,
        busy,
        rowsKey,
        onsort,
        oncolumn,
        onresize,
        activeColumn,
        columnMenuId,
        onopen,
        headerFilter,
        density
    }: {
        rows: readonly User[]
        columns: VisibleColumn[]
        sorting: UserTableState['sorting']
        busy: boolean
        rowsKey: string
        onsort: (id: string, append: boolean) => void
        oncolumn: (id: string, anchor: HTMLElement) => void
        onresize: (id: string, width: number | undefined, persist: boolean) => void
        activeColumn: string | undefined
        columnMenuId: string
        onopen: (id: number) => void
        headerFilter?: Snippet<[VisibleColumn]> | undefined
        density: TableDensity
    } = $props()

    let viewport: HTMLElement
    let scrollTop = $state(0)
    let clientHeight = $state(0)
    let headHeight = $state(0)
    const verticalSpacing = { xxs: 2, xs: 10, md: 16, xl: 32 }
    let padding = $derived(verticalSpacing[density])
    let rowHeight = $derived(50.4 + (padding - 2) * 2)
    let visible = $derived(
        fixedVirtualWindow(rows.length, rowHeight, scrollTop, clientHeight - headHeight)
    )
    let width = $derived(columns.reduce((sum, column) => sum + column.width, 60))
    let pageSelected = $derived(rows.filter(({ id }) => userSelection.has(id)).length)
    let allSelected = $derived(rows.length > 0 && pageSelected === rows.length)
    let t = $derived(locale.t)
    let resizing = $state<{
        id: string
        origin: number
        width: number
        direction: number
        delta: number
    }>()

    $effect(() => {
        if (rowsKey) {
            viewport.scrollTop = 0
            scrollTop = 0
        }
    })

    function toggle(id: number): void {
        if (userSelection.has(id)) userSelection.delete(id)
        else userSelection.add(id)
    }

    function startResize(
        event: PointerEvent & { currentTarget: HTMLButtonElement },
        column: VisibleColumn
    ): void {
        if (event.button !== 0) return
        event.preventDefault()
        event.currentTarget.setPointerCapture(event.pointerId)
        resizing = {
            id: column.id,
            origin: event.clientX,
            width: column.width,
            delta: 0,
            direction: getComputedStyle(event.currentTarget).direction === 'rtl' ? -1 : 1
        }
    }

    function dragResize(event: PointerEvent): void {
        if (!resizing) return
        const { id, origin, width, direction } = resizing
        const delta = (event.clientX - origin) * direction
        if (delta === resizing.delta) return
        resizing.delta = delta
        onresize(id, width + delta, false)
    }

    function finishResize(): void {
        if (!resizing) return
        const { id, width, delta } = resizing
        if (delta !== 0) onresize(id, width + delta, true)
        resizing = undefined
    }
</script>

<section
    class="user-window"
    aria-label={t('user-table.widget.table-title')}
    aria-busy={busy}
    bind:this={viewport}
    bind:clientHeight
    onscroll={() => {
        scrollTop = viewport.scrollTop
    }}
>
    <table
        style:width={`${width}px`}
        style:--row-height={`${rowHeight}px`}
        style:--cell-block={`${padding}px`}
        style:--cell-inline={`${density === 'xxs' ? 6 : padding}px`}
        class:resizing={resizing !== undefined}
    >
        <colgroup>
            <col style="width:60px">
            {#each columns as column (column.id)}
                <col style:width={`${column.width}px`}>
            {/each}
        </colgroup>
        <thead bind:clientHeight={headHeight}>
            <tr>
                <th scope="col" class="selection pinned">
                    <input
                        class="user-checkbox mantine-Checkbox-input"
                        type="checkbox"
                        aria-label="Toggle select all"
                        checked={allSelected}
                        indeterminate={pageSelected > 0 && !allSelected}
                        disabled={busy || rows.length === 0}
                        onchange={() => togglePageSelection(rows)}
                    >
                </th>
                {#each columns as column (column.id)}
                    {@const order = sorting[0]}
                    {@const sort = sorting.find((item) => item.id === column.id)}
                    <th
                        scope="col"
                        class:pinned={Boolean(column.inset)}
                        class:resized={resizing?.id === column.id}
                        style={column.inset}
                        aria-sort={order?.id === column.id
                            ? order.desc
                                ? 'descending'
                                : 'ascending'
                            : undefined}
                    >
                        <div class="column-header">
                            {#if column.sortable}
                                <button
                                    class="sort-label mantine-UnstyledButton-root"
                                    type="button"
                                    class:sorted={Boolean(sort)}
                                    onclick={(event) => onsort(column.id, event.shiftKey)}
                                >
                                    <span>{column.label}</span>
                                    <Icon
                                        name={sort
                                            ? sort.desc
                                                ? 'TbSortDescending'
                                                : 'TbSortAscending'
                                            : 'TbArrowsSort'}
                                        size={18}
                                    />
                                </button>
                            {:else}
                                <span class="sort-label"><span>{column.label}</span></span>
                            {/if}
                            <button
                                class="column-menu mantine-UnstyledButton-root"
                                type="button"
                                aria-label={`Column actions for ${column.label}`}
                                aria-haspopup="menu"
                                aria-expanded={activeColumn === column.id}
                                aria-controls={activeColumn === column.id
                                    ? columnMenuId
                                    : undefined}
                                onclick={(event) => oncolumn(column.id, event.currentTarget)}
                            >
                                <Icon name="TbDotsVertical" size={20} />
                            </button>
                            <button
                                class="column-resize mantine-UnstyledButton-root"
                                type="button"
                                aria-label={`Resize ${column.label}`}
                                title={`Resize ${column.label}`}
                                onpointerdown={(event) => startResize(event, column)}
                                onpointermove={dragResize}
                                onpointerup={finishResize}
                                onlostpointercapture={finishResize}
                                ondblclick={() => onresize(column.id, undefined, true)}
                                onkeydown={(event) => {
                                    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight')
                                        return
                                    event.preventDefault()
                                    const rtl =
                                        getComputedStyle(event.currentTarget).direction === 'rtl'
                                    const grow = (event.key === 'ArrowRight') !== rtl
                                    onresize(column.id, column.width + (grow ? 16 : -16), true)
                                }}
                            ></button>
                        </div>
                        {#if headerFilter}
                            <div
                                class="column-filter"
                                class:filter-username={column.id === 'username'}
                                class:filter-id={column.id === 'id'}
                                class:filter-tag={column.id === 'tag'}
                            >
                                {#if column.filter}
                                    {@render headerFilter(column)}
                                {/if}
                            </div>
                        {/if}
                    </th>
                {/each}
            </tr>
        </thead>
        <tbody>
            {#if visible.from > 0}
                <tr
                    class="spacer"
                    aria-hidden="true"
                    style:height={`${visible.from * rowHeight}px`}
                >
                    <td colspan={columns.length + 1}></td>
                </tr>
            {/if}
            {#each rows.slice(visible.from, visible.to) as user (user.id)}
                <tr
                    class:selected={userSelection.has(user.id)}
                    class="user-row"
                    onpointerup={(event) => {
                        if (
                            event.button === 0 &&
                            event.target instanceof Element &&
                            !event.target.closest('a, button, input, label')
                        )
                            onopen(user.id)
                    }}
                >
                    <td class="selection pinned">
                        <input
                            class="user-checkbox mantine-Checkbox-input"
                            type="checkbox"
                            aria-label="Toggle select row"
                            checked={userSelection.has(user.id)}
                            disabled={busy}
                            onchange={() => toggle(user.id)}
                        >
                    </td>
                    {#each columns as column (column.id)}
                        {@const value = column.value(user)}
                        <td
                            class:pinned={Boolean(column.inset)}
                            class:traffic-cell={column.id === 'usedTrafficBytes'}
                            style={column.inset}
                            title={value === null ? undefined : String(value)}
                        >
                            {#if column.id === 'username'}
                                <a
                                    class="username"
                                    href={p('/dashboard/open/user/:id', {
                                        params: { id: String(user.id) }
                                    })}
                                    onclick={(event) => {
                                        if (
                                            event.altKey ||
                                            event.ctrlKey ||
                                            event.metaKey ||
                                            event.shiftKey
                                        )
                                            return
                                        event.preventDefault()
                                        onopen(user.id)
                                    }}
                                >
                                    <span
                                        class="connection-dot"
                                        aria-hidden="true"
                                        style:background={getConnectionStatusColorUtil(
                                            user.userTraffic.onlineAt
                                        )}
                                    ></span>
                                    <span class="username-copy">
                                        <span class="user-name">{user.username}</span>
                                        <span class="last-seen"
                                            >{getTimeAgoUtil(
                                                user.userTraffic.onlineAt,
                                                t,
                                                locale.language
                                            )}</span
                                        >
                                    </span>
                                </a>
                            {:else if column.id === 'status'}
                                <div class="center"><UserStatusBadge status={user.status} /></div>
                            {:else if column.id === 'expireAt'}
                                <span class="expiration"
                                    >{getExpirationTextUtil(user.expireAt, t, locale)}</span
                                >
                            {:else if column.id === 'usedTrafficBytes'}
                                <UserTrafficCell {user} />
                            {:else if column.id === 'tag' || column.id === 'nodeName'}
                                <span class="mono" class:center={column.id === 'tag'}
                                    >{value ?? '–'}</span
                                >
                            {:else}
                                {value ?? '—'}
                            {/if}
                        </td>
                    {/each}
                </tr>
            {/each}
            {#if visible.to < rows.length}
                <tr
                    class="spacer"
                    aria-hidden="true"
                    style:height={`${(rows.length - visible.to) * rowHeight}px`}
                >
                    <td colspan={columns.length + 1}></td>
                </tr>
            {/if}
        </tbody>
    </table>
</section>

<style>
    .user-row {
        cursor: pointer;
    }
    .resizing {
        user-select: none;
        cursor: col-resize;
    }
    .user-window {
        max-height: min(64dvh, 40rem);
        overflow: auto;
        overscroll-behavior: contain;
    }

    table {
        min-width: 100%;
        border-spacing: 0;
        table-layout: fixed;
        background: var(--mantine-color-body);
        font-size: var(--mantine-font-size-sm);
        text-align: start;
    }

    thead {
        position: sticky;
        top: 0;
        z-index: 2;
        background: var(--gw-card-bg);
    }

    .user-row {
        height: var(--row-height);
    }

    th,
    td {
        padding: 0 var(--cell-inline);
        overflow: hidden;
        border-bottom: 1px solid var(--mantine-color-default-border);
        text-align: start;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    td {
        padding-block: var(--cell-block);
        line-height: var(--mantine-line-height);
    }

    td.traffic-cell {
        padding-inline: 0;
    }

    th {
        vertical-align: top;
        padding-block: var(--mantine-spacing-xs) calc(var(--mantine-spacing-xs) + 2px);
    }

    th:last-child,
    td:last-child {
        border-inline-end: none;
    }

    .column-header {
        position: relative;
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        height: 22px;
        min-height: 22px;
        font-weight: 700;
    }

    .column-resize {
        position: absolute;
        inset-block-start: 0;
        inset-inline-end: calc(-1 * var(--cell-inline));
        width: 4px;
        height: 24px;
        border: 0;
        border-radius: var(--mantine-radius-xs);
        background: var(--mantine-color-placeholder);
        cursor: col-resize;
        touch-action: none;
    }
    .column-resize:hover,
    .column-resize:focus-visible,
    .resized .column-resize {
        background: var(--mantine-primary-color-filled);
    }

    .sort-label,
    .column-menu {
        padding: 0;
        border: 0;
        background: transparent;
        color: inherit;
    }

    .sort-label {
        display: inline-flex;
        min-width: 0;
        align-items: center;
        gap: 3px;
        font-weight: 700;
        text-align: start;
    }

    .sort-label.sorted :global(svg) {
        color: var(--mantine-color-cyan-8);
    }

    button.sort-label span {
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .column-menu {
        display: inline-grid;
        width: 22px;
        height: 22px;
        flex: 0 0 auto;
        place-items: center;
        color: var(--mantine-color-dimmed);
    }
    .column-filter {
        display: flex;
        align-items: center;
        min-height: 36px;
    }
    .column-filter :global(.filter-choice),
    .column-filter :global(.filter-choice input) {
        height: 36px;
    }
    .column-filter.filter-username :global(.filter-text input) {
        width: 180px;
        flex: 0 0 180px;
    }
    .column-filter.filter-id :global(.filter-text input) {
        width: 100px;
        flex: 0 0 100px;
    }
    .column-filter.filter-tag :global(.filter-choice) {
        width: 132px;
        height: 22.4px;
        margin-inline-start: 1px;
    }
    .column-filter.filter-tag :global(.filter-choice input) {
        height: 22.4px;
    }

    .username {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding-inline-start: 4px;
        color: var(--mantine-color-text);
        text-decoration: none;
    }

    .username-copy {
        display: grid;
        min-width: 0;
    }

    .user-name,
    .last-seen {
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .user-name {
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }

    .last-seen {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
        line-height: var(--mantine-line-height-xs);
    }

    .connection-dot {
        width: 12px;
        height: 12px;
        flex: 0 0 auto;
        border-radius: 50%;
    }

    .center,
    .expiration {
        display: block;
        text-align: center;
    }

    .expiration {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }

    .mono {
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-md);
        font-weight: 500;
    }

    .pinned {
        position: sticky;
        z-index: 1;
        background: var(--mantine-color-body);
    }

    thead .pinned {
        background: var(--gw-card-bg);
    }

    .spacer td {
        padding: 0;
        border: 0;
    }
    tr.selected td,
    tr.selected .pinned {
        background: color-mix(in srgb, var(--mantine-color-cyan-4) 12%, var(--mantine-color-body));
    }

    .user-row:last-child td {
        border-block-end: 0;
    }

    tr.user-row:hover td,
    tr.user-row:hover .pinned {
        background: var(--mantine-color-default-hover);
    }

    .selection {
        inset-inline-start: 0;
    }

    .selection .user-checkbox {
        appearance: none;
        display: grid;
        width: 24px;
        height: 24px;
        margin: 0;
        padding: 0;
        place-content: center;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-xs);
        background: var(--mantine-color-default);
        vertical-align: middle;
    }

    .user-checkbox:checked,
    .user-checkbox:indeterminate {
        border-color: var(--mantine-color-cyan-8);
        background: var(--mantine-color-cyan-8);
    }

    .user-checkbox:checked::after {
        width: 10px;
        height: 6px;
        transform: translateY(-1px) rotate(-45deg);
        border-bottom: 2px solid white;
        border-left: 2px solid white;
        content: "";
    }

    .user-checkbox:indeterminate::after {
        width: 10px;
        height: 2px;
        background: white;
        content: "";
    }
</style>
