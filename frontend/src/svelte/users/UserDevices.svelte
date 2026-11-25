<script lang="ts">
    import {
        DeleteAllUserHwidDevicesCommand,
        DeleteUserHwidDeviceCommand
    } from '@remnawave/backend-contract'
    import { fixedVirtualWindow } from '@shared/lib/virtual-layout'
    import { formatTimeUtil } from '@shared/utils/time-utils'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { ipDetailsUrl } from '../torrent-reports/queries'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import TablePopup from '../ui/TablePopup.svelte'
    import { type HwidDevice, userDevicesQuery } from './queries'

    type TextField = 'platform' | 'hwid' | 'requestIp' | 'deviceModel' | 'userAgent'
    type SortField = TextField | 'createdAt' | 'updatedAt'
    let { userId, close }: { userId: number; close: () => void } = $props()
    let t = $derived(locale.t)
    const id = $props.id()
    const initialOrder: SortField[] = [
        'platform',
        'hwid',
        'requestIp',
        'deviceModel',
        'userAgent',
        'createdAt',
        'updatedAt'
    ]
    const initialWidths: Record<SortField, number> = {
        platform: 180,
        hwid: 120,
        requestIp: 200,
        deviceModel: 220,
        userAgent: 200,
        createdAt: 200,
        updatedAt: 200
    }
    let widths = $state({ ...initialWidths })
    let resize = $state<{ field: SortField; start: number; width: number }>()
    let offset = $state(0)
    let tableHeight = $state(500)
    const rowHeight = 36
    const headerHeight = 36
    let labels = $derived<Record<SortField, string>>({
        platform: 'Platform',
        hwid: 'HWID',
        requestIp: t('common.field.ip-address'),
        deviceModel: t('get-hwid-user-devices.feature.model'),
        userAgent: t('common.field.user-agent'),
        createdAt: t('get-hwid-user-devices.feature.added'),
        updatedAt: 'Updated'
    })
    let filters = $state<Partial<Record<TextField, string>>>({})
    let order = $state([...initialOrder])
    let hidden = $state<SortField[]>([])
    let visible = $derived(order.filter((field) => !hidden.includes(field)))
    let sort = $state<SortField>('createdAt')
    let ascending = $state(false)
    let dragging = $state<SortField>()
    let popup = $state<{
        anchor: HTMLElement
        mode: 'reset' | 'columns' | 'filter'
        field?: TextField
    }>()
    let confirmation = $state<{ hwid?: string }>()
    let collator = $derived(new Intl.Collator(locale.language, { numeric: true }))

    function isTextField(field: SortField): field is TextField {
        return field !== 'createdAt' && field !== 'updatedAt'
    }

    function sortBy(field: SortField): void {
        if (sort === field) ascending = !ascending
        else {
            sort = field
            ascending = true
        }
    }

    function reorder(target: SortField): void {
        if (!dragging || dragging === target) return
        const source = order.indexOf(dragging)
        const destination = order.indexOf(target)
        order.splice(source, 1)
        order.splice(destination, 0, dragging)
        dragging = undefined
    }

    function resetSort(): void {
        sort = 'createdAt'
        ascending = false
    }

    function toggle(field: SortField): void {
        if (hidden.includes(field)) hidden = hidden.filter((name) => name !== field)
        else if (visible.length > 1) hidden = [...hidden, field]
    }

    const devices = createQuery(() => userDevicesQuery(userId))
    let rows = $derived.by(() => {
        const criteria = initialOrder.filter(isTextField).map((name) => ({
            name,
            term: (filters[name] ?? '').trim().toLocaleLowerCase(locale.language)
        }))
        return (devices.data?.devices ?? [])
            .filter((device) =>
                criteria.every(
                    ({ name, term }) =>
                        !term ||
                        fieldValue(device, name).toLocaleLowerCase(locale.language).includes(term)
                )
            )
            .sort((a, b) => {
                const left = a[sort]
                const right = b[sort]
                const diff =
                    left instanceof Date && right instanceof Date
                        ? left.getTime() - right.getTime()
                        : collator.compare(String(left ?? ''), String(right ?? ''))
                return (ascending ? diff : -diff) || collator.compare(a.hwid, b.hwid)
            })
    })
    const virtual = $derived(
        fixedVirtualWindow(rows.length, rowHeight, offset, tableHeight - headerHeight)
    )
    const tableWidth = $derived(visible.reduce((total, field) => total + widths[field], 60))
    const removal = createMutation(() => ({
        mutationFn: async (hwid: string | undefined) => {
            const query = userDevicesQuery(userId)
            await queryClient.cancelQueries({ queryKey: query.queryKey })
            const result =
                hwid === undefined
                    ? await submitCommand(DeleteAllUserHwidDevicesCommand, { userId })
                    : await submitCommand(DeleteUserHwidDeviceCommand, { userId, hwid })
            queryClient.setQueryData(query.queryKey, result)
            await queryClient.invalidateQueries({ queryKey: ['hwid'] })
        },
        onSuccess: () =>
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
    }))

    function fieldValue(device: HwidDevice, name: TextField): string {
        if (name !== 'deviceModel') return device[name] ?? ''
        const model = device.deviceModel || '—'
        return device.osVersion ? `${model} (${device.osVersion})` : model
    }

    function remove(): void {
        if (removal.isPending || !confirmation) return
        removal.mutate(confirmation.hwid)
        confirmation = undefined
    }

    function copy(value: string): void {
        void navigator.clipboard.writeText(value).then(
            () =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: value,
                    color: 'teal'
                }),
            (failure: unknown) =>
                notifications.show({
                    title: t('common.message.error'),
                    message: failure instanceof Error ? failure.message : t('common.message.error'),
                    color: 'red'
                })
        )
    }
</script>

<Overlay
    class="hwid-modal"
    title={t('get-hwid-user-devices.feature.hwid-devices')}
    icon="TbDevices"
    tone="violet"
    width={1500}
    {close}
>
    <div class="layout">
        <FormCard
            title={String(devices.data?.devices.length ?? 0)}
            description={t('get-hwid-user-devices.feature.devices')}
            icon="TbDevices"
            tone="indigo"
            headingSize="xl"
        >
            {#snippet actions()}
                <div class="summary-actions">
                    <ActionIcon
                        icon="TbRefresh"
                        tone="indigo"
                        size={34}
                        iconSize={20}
                        label={t('common.action.refresh')}
                        disabled={devices.isFetching || removal.isPending}
                        onclick={() => void devices.refetch()}
                    />
                    <ActionIcon
                        icon="TbTrash"
                        tone="red"
                        size={34}
                        iconSize={20}
                        label={t('get-hwid-user-devices.feature.delete-all-devices')}
                        disabled={removal.isPending}
                        onclick={() => (confirmation = {})}
                    />
                </div>
            {/snippet}
        </FormCard>
        {#if devices.error || removal.error}
            <p role="alert">{devices.error?.message || removal.error?.message}</p>
        {/if}
        <div class="desktop">
            <div
                class="table-scroll"
                bind:clientHeight={tableHeight}
                onscroll={(event) => (offset = event.currentTarget.scrollTop)}
            >
                <table
                    aria-label={t('get-hwid-user-devices.feature.hwid-devices')}
                    style:width={`${tableWidth}px`}
                >
                    <colgroup>
                        {#each visible as field (field)}
                            <col style:width={`${widths[field]}px`}>
                        {/each}
                        <col style:width="60px">
                    </colgroup>
                    <thead>
                        <tr>
                            {#each visible as field (field)}
                                <th
                                    scope="col"
                                    aria-sort={sort === field
                                        ? ascending
                                            ? 'ascending'
                                            : 'descending'
                                        : 'none'}
                                    ondragover={(event) => {
                                        if (dragging) event.preventDefault()
                                    }}
                                    ondrop={(event) => {
                                        event.preventDefault()
                                        reorder(field)
                                    }}
                                >
                                    <div class="column">
                                        <button
                                            type="button"
                                            class="mantine-UnstyledButton-root drag"
                                            draggable="true"
                                            aria-label={`Reorder ${labels[field]}`}
                                            ondragstart={(event) => {
                                                if (!event.dataTransfer) return
                                                dragging = field
                                                event.dataTransfer.effectAllowed = 'move'
                                                event.dataTransfer.setData('text/plain', field)
                                            }}
                                            ondragend={() => (dragging = undefined)}
                                            onkeydown={(event) => {
                                                if (
                                                    event.key !== 'ArrowLeft' &&
                                                    event.key !== 'ArrowRight'
                                                )
                                                    return
                                                const target =
                                                    visible[
                                                        visible.indexOf(field) +
                                                            (event.key === 'ArrowLeft' ? -1 : 1)
                                                    ]
                                                if (!target) return
                                                event.preventDefault()
                                                dragging = field
                                                reorder(target)
                                            }}
                                        >
                                            <Icon name="TbGripVertical" size={16} />
                                        </button>
                                        <button
                                            type="button"
                                            class="mantine-UnstyledButton-root heading"
                                            onclick={() => sortBy(field)}
                                        >
                                            {labels[field]}
                                        </button>
                                        <ActionIcon
                                            icon="TbX"
                                            label={t('common.action.clear')}
                                            size={16}
                                            iconSize={14}
                                            subtle
                                            disabled={!isTextField(field) || !filters[field]}
                                            onclick={() => {
                                                if (isTextField(field)) filters[field] = ''
                                            }}
                                        />
                                        <ActionIcon
                                            icon={sort === field
                                                ? ascending
                                                    ? 'TbSortAscending'
                                                    : 'TbSortDescending'
                                                : 'TbArrowsSort'}
                                            label={t('get-hwid-user-devices.feature.sort-by')}
                                            size={18}
                                            iconSize={16}
                                            subtle
                                            onclick={() => sortBy(field)}
                                        />
                                        {#if isTextField(field) && field !== 'hwid'}
                                            <ActionIcon
                                                icon="TbFilter"
                                                label={t('common.action.search')}
                                                size={22}
                                                iconSize={16}
                                                {...filters[field] ? { tone: 'cyan' } : {}}
                                                onclick={(event) => {
                                                    if (event.currentTarget instanceof HTMLElement)
                                                        popup = {
                                                            anchor: event.currentTarget,
                                                            mode: 'filter',
                                                            field
                                                        }
                                                }}
                                            />
                                        {/if}
                                        <button
                                            class="mantine-UnstyledButton-root resize"
                                            type="button"
                                            aria-label={`Resize ${labels[field]}`}
                                            onpointerdown={(event) => {
                                                if (event.button !== 0) return
                                                resize = {
                                                    field,
                                                    start: event.clientX,
                                                    width: widths[field]
                                                }
                                                event.currentTarget.setPointerCapture(
                                                    event.pointerId
                                                )
                                            }}
                                            onpointermove={(event) => {
                                                if (!resize || resize.field !== field) return
                                                const direction =
                                                    locale.direction === 'rtl' ? -1 : 1
                                                widths[field] = Math.max(
                                                    100,
                                                    resize.width +
                                                        (event.clientX - resize.start) * direction
                                                )
                                            }}
                                            onlostpointercapture={() => (resize = undefined)}
                                            onkeydown={(event) => {
                                                if (
                                                    event.key !== 'ArrowLeft' &&
                                                    event.key !== 'ArrowRight'
                                                )
                                                    return
                                                event.preventDefault()
                                                widths[field] = Math.max(
                                                    100,
                                                    widths[field] +
                                                        (event.key === 'ArrowLeft' ? -8 : 8)
                                                )
                                            }}
                                        ></button>
                                    </div>
                                </th>
                            {/each}
                            <th aria-label={t('common.action.actions')} scope="col"></th>
                        </tr>
                    </thead>
                    <tbody>
                        {#if virtual.from > 0}
                            <tr aria-hidden="true" class="spacer">
                                <td
                                    colspan={visible.length + 1}
                                    style:height={`${virtual.from * rowHeight}px`}
                                ></td>
                            </tr>
                        {/if}
                        {#each rows.slice(virtual.from, virtual.to) as device (device.hwid)}
                            <tr>
                                {#each visible as field (field)}
                                    <td>{@render cell(device, field)}</td>
                                {/each}
                                <td>{@render deleteDevice(device)}</td>
                            </tr>
                        {/each}
                        {#if virtual.to < rows.length}
                            <tr aria-hidden="true" class="spacer">
                                <td
                                    colspan={visible.length + 1}
                                    style:height={`${(rows.length - virtual.to) * rowHeight}px`}
                                ></td>
                            </tr>
                        {/if}
                    </tbody>
                </table>
                {#if !rows.length}
                    <div class="table-empty">
                        {#if devices.isPending}
                            <RouteLoading compact />
                        {:else}
                            <span class="empty-icon"><Icon name="TbDatabaseOff" size={24} /></span>
                            No records
                        {/if}
                    </div>
                {/if}
            </div>
            <div class="footer">
                <Button
                    icon="TbRestore"
                    variant="default"
                    onclick={(event) => (popup = { anchor: event.currentTarget, mode: 'reset' })}
                >
                    {t('common.action.reset')} <Icon name="TbChevronDown" size={16} />
                </Button>
                <Button
                    icon="TbColumns"
                    variant="default"
                    onclick={(event) => (popup = { anchor: event.currentTarget, mode: 'columns' })}
                >
                    {visible.length}/{initialOrder.length}
                </Button>
            </div>
        </div>
        <div class="mobile">
            {#if devices.isPending}
                <RouteLoading compact />
            {:else if !devices.data?.devices.length}
                <EmptyState icon="TbDevices" label={t('common.message.nothing-found')} />
            {:else}
                {#each devices.data.devices as device (device.hwid)}
                    <FormCard title={device.platform ?? 'Unknown'} icon="TbDevices" tone="indigo">
                        {#snippet actions()}
                            {@render deleteDevice(device)}
                        {/snippet}
                        <dl>
                            {#each initialOrder.slice(1) as field (field)}
                                <div>
                                    <dt>{labels[field]}</dt>
                                    <dd>{@render cell(device, field)}</dd>
                                </div>
                            {/each}
                        </dl>
                    </FormCard>
                {/each}
            {/if}
        </div>
    </div>
    {#if popup}
        <TablePopup
            anchor={popup.anchor}
            id={`${id}-options`}
            label={popup.field ? labels[popup.field] : t('common.action.actions')}
            role="dialog"
            alignEnd
            width={230}
            preferAbove={popup.mode !== 'filter'}
            focusOnOpen
            close={() => (popup = undefined)}
        >
            {#if popup.mode === 'filter' && popup.field}
                {@const field = popup.field}
                <input
                    class="filter"
                    type="search"
                    data-popup-autofocus
                    aria-label={labels[field]}
                    placeholder="Filter by..."
                    value={filters[field] ?? ''}
                    oninput={(event) => (filters[field] = event.currentTarget.value)}
                >
            {:else if popup.mode === 'columns'}
                <div class="column-options">
                    {#each initialOrder as field (field)}
                        <label
                            ><input
                                type="checkbox"
                                checked={!hidden.includes(field)}
                                disabled={visible.length === 1 && visible.includes(field)}
                                onchange={() => toggle(field)}
                            >{labels[field]}</label
                        >
                    {/each}
                </div>
            {:else}
                <div class="reset-options">
                    <Button
                        variant="subtle"
                        onclick={() => {
                            resetSort()
                            popup = undefined
                        }}
                        >{t('get-hwid-user-devices.feature.sort-by')}</Button
                    >
                    <Button
                        variant="subtle"
                        onclick={() => {
                            order = [...initialOrder]
                            hidden = []
                            widths = { ...initialWidths }
                            popup = undefined
                        }}
                        ><Icon name="TbColumns" size={16} />
                        {t('action-group.feature.reset-table')}</Button
                    >
                    <Button
                        variant="subtle"
                        onclick={() => {
                            filters = {}
                            popup = undefined
                        }}
                        >{t('action-group.feature.clear-filters')}</Button
                    >
                </div>
            {/if}
        </TablePopup>
    {/if}
</Overlay>

{#if confirmation}
    <Overlay
        title={t('common.action.confirm-action')}
        icon="TbTrash"
        tone="red"
        width={440}
        close={() => (confirmation = undefined)}
    >
        <p>{t('common.message.confirm-action-description')}</p>
        <div class="footer">
            <Button variant="subtle" onclick={() => (confirmation = undefined)}
                >{t('common.action.cancel')}</Button
            >
            <Button variant="soft" color="red" onclick={remove}>{t('common.action.delete')}</Button>
        </div>
    </Overlay>
{/if}

{#snippet deleteDevice(device: HwidDevice)}
    <ActionIcon
        icon="TbTrash"
        tone="red"
        size={28}
        iconSize={16}
        label={t('get-hwid-user-devices.feature.delete-device')}
        disabled={removal.isPending}
        onclick={() => (confirmation = { hwid: device.hwid })}
    />
{/snippet}

{#snippet cell(device: HwidDevice, field: SortField)}
    {#if field === 'hwid' || field === 'userAgent'}
        {@const value = device[field]}
        {#if value}
            <button
                class="mantine-UnstyledButton-root cell-copy"
                type="button"
                title={value}
                onclick={() => copy(value)}
            >
                {value}
            </button>
        {:else}
            —
        {/if}
    {:else if field === 'requestIp'}
        {#if device.requestIp}
            <a href={ipDetailsUrl(device.requestIp)} target="_blank" rel="noreferrer"
                >{device.requestIp}</a
            >
        {:else}
            —
        {/if}
    {:else if field === 'createdAt' || field === 'updatedAt'}
        <time datetime={device[field].toISOString()}
            >{formatTimeUtil({
                time: device[field],
                template: 'TIME_FIRST_DATETIME',
                language: locale.language
            })}</time
        >
    {:else}
        {fieldValue(device, field) || 'Unknown'}
    {/if}
{/snippet}

<style>
    :global(.panel-overlay.hwid-modal.modal) {
        width: min(1500px, 90vw);
    }
    .layout,
    .desktop {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
    .summary-actions,
    .footer {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .footer {
        justify-content: end;
    }
    .table-scroll {
        position: relative;
        height: 500px;
        max-width: 100%;
        overflow: auto;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-sm);
    }
    table {
        min-width: 100%;
        border-collapse: separate;
        border-spacing: 0;
        table-layout: fixed;
        font-size: var(--mantine-font-size-sm);
    }
    :is(th, td) {
        height: 36px;
        padding-inline: var(--mantine-spacing-sm);
        border-block-end: 1px solid var(--mantine-color-default-border);
        border-inline-end: 1px solid var(--mantine-color-default-border);
        text-align: start;
    }
    :is(th, td):last-child {
        border-inline-end: 0;
    }
    th {
        position: sticky;
        top: 0;
        z-index: 1;
        background: var(--mantine-color-body);
    }
    th:first-child,
    td:first-child {
        position: sticky;
        inset-inline-start: 0;
        background: var(--mantine-color-body);
        box-shadow: 1px 0 var(--mantine-color-default-border);
    }
    th:first-child {
        z-index: 2;
    }
    .column {
        display: flex;
        align-items: center;
        gap: 4px;
        white-space: nowrap;
    }
    .drag {
        color: var(--mantine-color-cyan-4);
        cursor: grab;
    }
    .resize {
        position: absolute;
        inset: 0 0 0 auto;
        width: 6px;
        cursor: col-resize;
        touch-action: none;
    }
    .resize:hover {
        background: var(--mantine-color-cyan-light);
    }
    .spacer td:first-child {
        position: static;
        padding: 0;
        border: 0;
        box-shadow: none;
    }
    .heading {
        flex: 1;
        font-weight: 700;
    }
    td {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    tr:nth-child(even) td {
        background: var(--gw-raised-bg);
    }
    tr:hover td {
        background: var(--mantine-color-default-hover);
    }
    td a,
    time {
        font-family: var(--mantine-font-family-monospace);
    }
    .cell-copy {
        display: block;
        max-width: 100%;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .table-empty {
        position: sticky;
        inset-inline-start: 0;
        width: 100%;
        height: 460px;
        display: grid;
        place-content: center;
        justify-items: center;
        gap: 6px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        pointer-events: none;
    }
    .empty-icon {
        display: grid;
        width: 44px;
        height: 44px;
        place-items: center;
        border-radius: 50%;
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-dimmed);
    }
    .filter {
        width: 100%;
    }
    .column-options,
    .reset-options {
        display: grid;
        gap: 6px;
    }
    .column-options label {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 4px;
    }
    .mobile {
        display: none;
    }
    dl {
        display: grid;
        gap: 6px;
        margin: 0;
    }
    dt {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    dd {
        margin: 0;
        overflow-wrap: anywhere;
    }
    @media (max-width: 48rem) {
        .desktop {
            display: none;
        }
        .mobile {
            display: grid;
            gap: var(--mantine-spacing-md);
        }
    }
</style>
