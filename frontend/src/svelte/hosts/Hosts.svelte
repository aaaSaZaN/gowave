<script lang="ts">
    import {
        BulkDeleteHostsCommand,
        BulkDisableHostsCommand,
        BulkEnableHostsCommand,
        CloneHostCommand,
        INTERNAL_SQUADS_MODE,
        ReorderHostsCommand,
        SUBSCRIPTION_TEMPLATE_TYPE
    } from '@remnawave/backend-contract'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import ColorHash from '@shared/lib/color'
    import { type Move, moveSelection } from '@shared/lib/move-selection'
    import { fixedVirtualWindow } from '@shared/lib/virtual-layout'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { SvelteSet } from 'svelte/reactivity'
    import { submitCommand, submitVoidCommand } from '../commands'
    import { profilesQuery } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import DataTable from '../ui/DataTable.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import JsonTree from '../ui/JsonTree.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import TableFilter from '../ui/TableFilter.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import { type EntityViewMode, readHostView, writeHostView } from '../view-preferences'
    import HostConfirmation from './HostConfirmation.svelte'
    import HostDialog from './HostDialog.svelte'
    import HostEditor from './HostEditor.svelte'
    import { type Host, hostsKey, hostsQuery } from './queries'

    type Action = 'enable' | 'disable' | 'clone' | 'delete'
    const columns = {
        remark: 'base-host-form.remark',
        status: 'common.field.status',
        address: 'common.field.address',
        port: 'common.field.port',
        sni: 'base-host-form.sni',
        profile: 'use-nodes-table-widget.config-profile',
        inbound: 'common.field.inbounds',
        tags: 'common.field.tags'
    } as const
    type Column = keyof typeof columns
    const tableColumnCount = Object.keys(columns).length + 2
    const actionLabels = {
        enable: 'common.action.enable',
        disable: 'common.action.disable',
        clone: 'common.action.clone',
        delete: 'common.action.delete'
    } as const
    const actionStyles = {
        enable: { icon: 'PiPulseDuotone', color: 'green' },
        disable: { icon: 'PiProhibitDuotone', color: 'gray' },
        clone: { icon: 'TbCopy', color: 'indigo' },
        delete: { icon: 'TbTrash', color: 'red' }
    } as const
    const moveActions = [
        { mode: 'top', icon: 'TbArrowBarToUp', label: 'Move to top' },
        { mode: 'up', icon: 'TbArrowBigUp', label: 'Move up' },
        { mode: 'down', icon: 'TbArrowBigDown', label: 'Move down' },
        { mode: 'bottom', icon: 'TbArrowBarToDown', label: 'Move to bottom' }
    ] as const
    const bulkCommands = {
        enable: BulkEnableHostsCommand,
        disable: BulkDisableHostsCommand,
        delete: BulkDeleteHostsCommand
    }
    const colorHash = new ColorHash({ lightness: [0.65, 0.65, 0.65] })
    const initial = readHostView()
    const hostParameters = [
        { field: 'muxParams', label: 'Mux', title: 'Mux Params', icon: 'TbCloudNetwork' },
        { field: 'finalMask', label: 'Final Mask', title: 'Final Mask', icon: 'TbMask' },
        { field: 'sockoptParams', label: 'SockOpt', title: 'SockOpt Params', icon: 'PiNetwork' }
    ] as const satisfies ReadonlyArray<{
        field: keyof Host
        label: string
        title: string
        icon: IconName
    }>
    let hostParameter = $state.raw<{ title: string; icon: IconName; value: unknown }>()
    let confirmation = $state.raw<{ action: 'clone' | 'delete'; items: Host[]; skipped: number }>()
    let helpOpen = $state(false)
    let searchOpen = $state(true)
    let editing = $state<string | null>(null)
    let viewport = $state(1024)
    let viewportHeight = $state(0)
    let pageScroll = $state(0)
    const compact = $derived(viewport < 768)
    const hosts = createQuery(() => hostsQuery)
    const profiles = createQuery(() => profilesQuery)
    const selection = new SvelteSet<string>()
    let t = $derived(locale.t)
    let mode = $state<EntityViewMode>(initial.mode)
    let tag = $state(initial.tag)
    let filters = $state<Partial<Record<Column, string>>>({})
    const hasFilters = $derived(Object.values(filters).some((value) => value.trim()))
    let sort = $state<{ column: Column; descending: boolean }>()
    let bulkIds = $state.raw<string[] | undefined>()
    let scrollArea = $state<HTMLDivElement>()
    let height = $state(0)
    let scrollTop = $state(0)
    let cardList = $state<HTMLUListElement>()
    let drag = $state<{ uuid: string; from: number; to: number; offset: number }>()
    let grab:
        | {
              handle: HTMLButtonElement
              pointerId: number
              uuid: string
              index: number
              startY: number
              anchor: number
              scrollTopEdge: number
              clientY: number
          }
        | undefined
    let scrollFrame: number | undefined
    let ignoreGripClick = false
    let pendingOrder = $state.raw<string[]>()
    let ordered = $derived.by(() => {
        const items = hosts.data ?? []
        if (!pendingOrder) return items
        const positions = new Map(pendingOrder.map((uuid, index) => [uuid, index]))
        const end = positions.size
        return [...items].sort(
            (a, b) => (positions.get(a.uuid) ?? end) - (positions.get(b.uuid) ?? end)
        )
    })
    let selected = $derived(ordered.filter(({ uuid }) => selection.has(uuid)))
    let profilesById = $derived(
        new Map(profiles.data?.configProfiles.map((profile) => [profile.uuid, profile]))
    )
    let inbounds = $derived(
        new Map(
            profiles.data?.configProfiles.flatMap((profile) =>
                profile.inbounds.map(
                    (inbound) => [inbound.uuid, `${profile.name} / ${inbound.tag}`] as const
                )
            )
        )
    )
    let filtered = $derived.by(() => {
        const active = Object.entries(filters).flatMap(([key, value]) => {
            const text = value.trim().toLocaleLowerCase(locale.language)
            return text ? [{ column: key as Column, text }] : []
        })
        const list = ordered.filter(
            (host) =>
                (!tag || (tag === NO_TAG ? host.tags.length === 0 : host.tags.includes(tag))) &&
                active.every(({ column, text }) =>
                    column === 'status'
                        ? stateOf(host) === text
                        : value(host, column).toLocaleLowerCase(locale.language).includes(text)
                )
        )
        if (!sort) return list
        const { column, descending } = sort
        return list.sort((a, b) => {
            const order =
                column === 'port'
                    ? a.port - b.port
                    : value(a, column).localeCompare(value(b, column), locale.language, {
                          numeric: true
                      })
            return descending ? -order : order
        })
    })
    let reorderable = $derived(!sort && !tag && !hasFilters)
    let rowHeight = $derived(mode === 'table' ? 50 : compact ? 202 : 88)
    let range = $derived.by(() => {
        if (mode === 'table')
            return fixedVirtualWindow(filtered.length, rowHeight, scrollTop, height)
        const listTop = scrollArea?.offsetTop ?? 0
        return fixedVirtualWindow(
            filtered.length,
            rowHeight,
            pageScroll - listTop,
            viewportHeight - Math.max(0, listTop - pageScroll),
            7
        )
    })
    let rows = $derived(filtered.slice(range.from, range.to))
    let cardRows = $derived.by(() => {
        const cards = rows.map((host, offset) => ({ host, index: range.from + offset }))
        // The browser needs the drag source to stay mounted during autoscroll.
        if (drag && (drag.from < range.from || drag.from >= range.to)) {
            const host = filtered[drag.from]
            if (host?.uuid === drag.uuid) {
                cards.push({ host, index: drag.from })
                cards.sort((a, b) => a.index - b.index)
            }
        }
        return cards
    })

    const update = createMutation(() => ({
        mutationFn: async (change: { ordered: string[] } | { action: Action; items: Host[] }) => {
            if ('ordered' in change) {
                const result = await submitCommand(ReorderHostsCommand, {
                    hosts: change.ordered.map((uuid, index) => ({ uuid, viewPosition: index + 1 }))
                })
                if (!result.isUpdated) throw new Error('Could not save host order')
                return
            }
            if (change.action === 'clone') {
                let created = 0
                try {
                    for (const { uuid } of change.items) {
                        await submitCommand(CloneHostCommand, { cloneFromUuid: uuid })
                        created += 1
                    }
                } catch (error) {
                    if (created === 0) throw error
                    throw new Error(
                        `Cloned ${created} of ${change.items.length} hosts. ${error instanceof Error ? error.message : String(error)}`,
                        { cause: error }
                    )
                }
                return
            }
            await submitVoidCommand(bulkCommands[change.action], {
                uuids: change.items.map(({ uuid }) => uuid)
            })
        },
        onSuccess: (_data, change) => {
            if ('action' in change) {
                for (const { uuid } of change.items) selection.delete(uuid)
                notifications.show({
                    title: t('common.message.success'),
                    message: t('common.message.operation-completed'),
                    color: 'teal'
                })
            }
        },
        onError: (_error, change) => {
            if ('ordered' in change) pendingOrder = undefined
        },
        onSettled: async (_data, _error, change) => {
            try {
                await queryClient.invalidateQueries({ queryKey: hostsKey })
            } finally {
                if ('ordered' in change) pendingOrder = undefined
            }
        }
    }))

    function stateOf(host: Host): string {
        if (host.isDisabled) return 'disabled'
        if (host.isHidden) return 'hidden'
        return 'enabled'
    }

    function tagsChanged(value: string): void {
        tag = value
        writeHostView(mode, tag)
        scrollToStart()
    }

    function stateLabel(host: Host): string {
        switch (stateOf(host)) {
            case 'disabled':
                return t('use-hosts-table-widget.disabled')
            case 'hidden':
                return t('use-hosts-table-widget.hidden')
            default:
                return t('use-hosts-table-widget.enabled')
        }
    }

    function profileName(host: Host): string {
        return profilesById.get(host.inbound.configProfileUuid ?? '')?.name ?? ''
    }

    function inboundTag(host: Host): string {
        const profile = profilesById.get(host.inbound.configProfileUuid ?? '')
        return (
            profile?.inbounds.find(({ uuid }) => uuid === host.inbound.configProfileInboundUuid)
                ?.tag ?? ''
        )
    }

    function inboundOf(host: Host): string {
        const uuid = host.inbound.configProfileInboundUuid
        return uuid ? (inbounds.get(uuid) ?? uuid) : t('common.message.no-inbound-selected')
    }

    function value(host: Host, column: Column): string {
        switch (column) {
            case 'inbound':
                return inboundTag(host)
            case 'profile':
                return profileName(host)
            case 'status':
                return stateLabel(host)
            case 'tags':
                return host.tags.join(', ')
            default:
                return String(host[column] ?? '')
        }
    }

    function scrollToStart(): void {
        if (scrollArea) scrollArea.scrollTop = 0
        scrollTop = 0
        if (mode === 'cards') window.scrollTo({ top: 0, behavior: 'instant' })
    }

    function view(next: EntityViewMode): void {
        if (mode === next) return
        mode = next
        sort = undefined
        if (next === 'cards') {
            filters = {}
        }
        writeHostView(mode, tag)
        scrollToStart()
    }

    function resetTable(): void {
        filters = {}
        sort = undefined
        tagsChanged('')
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

    function select(uuid: string): void {
        if (selection.has(uuid)) selection.delete(uuid)
        else selection.add(uuid)
    }

    function selectAll(): void {
        for (const { uuid } of ordered) selection.add(uuid)
    }

    function apply(action: Action, items: Host[]): void {
        update.reset()
        const targets =
            action === 'clone'
                ? items.filter(
                      ({ inbound }) => inbound.configProfileInboundUuid && inbound.configProfileUuid
                  )
                : items
        if (targets.length === 0) {
            notifications.show({
                title: t('common.message.error'),
                message: t('edit-host-modal.widget.dangling-host-cannot-be-cloned'),
                color: 'red'
            })
            return
        }
        if (action === 'clone' || action === 'delete') {
            confirmation = { action, items: targets, skipped: items.length - targets.length }
            return
        }
        update.mutate({ action, items: targets })
    }

    function confirmAction(): void {
        if (!confirmation || update.isPending) return
        const { action, items, skipped } = confirmation
        confirmation = undefined
        if (skipped) {
            notifications.show({
                title: t('common.message.warning'),
                message: t('multi-select-hosts.feature.dangling-hosts-skipped', { count: skipped }),
                color: 'teal'
            })
        }
        update.mutate({ action, items })
    }

    function saveOrder(next: string[]): void {
        if (update.isPending || next.every((id, index) => id === ordered[index]?.uuid)) return
        pendingOrder = next
        update.mutate({ ordered: next })
    }

    function move(mode: Move): void {
        if (!reorderable) return
        saveOrder(
            moveSelection(
                ordered.map(({ uuid }) => uuid),
                selection,
                mode
            )
        )
    }

    function grabHost(
        event: PointerEvent & { currentTarget: HTMLButtonElement },
        uuid: string
    ): void {
        if (
            !reorderable ||
            update.isPending ||
            grab ||
            !cardList ||
            event.button !== 0 ||
            !event.isPrimary
        )
            return
        const index = ordered.findIndex((host) => host.uuid === uuid)
        if (index === -1) return

        const handle = event.currentTarget
        handle.setPointerCapture(event.pointerId)
        ignoreGripClick = false
        const header = document.querySelector('.dashboard-header')
        grab = {
            handle,
            pointerId: event.pointerId,
            uuid,
            index,
            startY: event.clientY,
            anchor: event.clientY - cardList.getBoundingClientRect().top - index * rowHeight,
            scrollTopEdge: (header?.getBoundingClientRect().bottom ?? 0) + 64,
            clientY: event.clientY
        }
    }

    function updateDragOffset(): void {
        if (!grab || !drag || !cardList) return
        const top = cardList.getBoundingClientRect().top
        const offset = grab.clientY - top - grab.anchor
        drag.offset = Math.max(0, Math.min((ordered.length - 1) * rowHeight, offset))
        drag.to = Math.round(drag.offset / rowHeight)
    }

    function scrollDuringDrag(): void {
        scrollFrame = undefined
        if (!grab || !drag) return
        const top = grab.scrollTopEdge
        const bottom = viewportHeight - 64
        const distance =
            grab.clientY < top ? grab.clientY - top : Math.max(0, grab.clientY - bottom)
        if (distance === 0) return
        const speed = Math.max(-18, Math.min(18, distance / 4))
        window.scrollBy({ top: speed, behavior: 'instant' })
        updateDragOffset()
        scrollFrame = requestAnimationFrame(scrollDuringDrag)
    }

    function moveGrab(event: PointerEvent): void {
        if (!grab || event.pointerId !== grab.pointerId) return
        grab.clientY = event.clientY
        if (!drag) {
            if (Math.abs(event.clientY - grab.startY) < 5) return
            const { index, uuid } = grab
            drag = { uuid, from: index, to: index, offset: index * rowHeight }
            ignoreGripClick = true
        }
        updateDragOffset()
        scrollFrame ??= requestAnimationFrame(scrollDuringDrag)
    }

    function cardOffset(index: number): number {
        if (!drag) return index * rowHeight
        const { from, to, offset } = drag
        if (index === from) return offset
        if (from < index && index <= to) return (index - 1) * rowHeight
        if (to <= index && index < from) return (index + 1) * rowHeight
        return index * rowHeight
    }

    function cancelGrab(): void {
        const active = grab
        grab = undefined
        drag = undefined
        if (scrollFrame !== undefined) {
            cancelAnimationFrame(scrollFrame)
            scrollFrame = undefined
        }
        if (active?.handle.hasPointerCapture(active.pointerId)) {
            active.handle.releasePointerCapture(active.pointerId)
        }
    }

    function finishGrab(event: PointerEvent): void {
        if (!grab || event.pointerId !== grab.pointerId) return
        if (drag) {
            grab.clientY = event.clientY
            updateDragOffset()
        }
        const active = drag
        const bounds = cardList?.getBoundingClientRect()
        cancelGrab()
        if (
            !active ||
            !bounds ||
            event.clientX < bounds.left ||
            event.clientX > bounds.right ||
            event.clientY < bounds.top ||
            event.clientY > bounds.bottom ||
            !reorderable
        )
            return
        const { uuid, from, to } = active
        if (from === to || ordered[from]?.uuid !== uuid) return
        const next = ordered.map((host) => host.uuid)
        next.splice(from, 1)
        next.splice(to, 0, uuid)
        saveOrder(next)
    }

    function lostGrab(event: PointerEvent): void {
        if (event.pointerId === grab?.pointerId) cancelGrab()
    }

    $effect(() => () => {
        if (scrollFrame !== undefined) cancelAnimationFrame(scrollFrame)
    })

    function openBulkEdit(): void {
        bulkIds = selected.map(({ uuid }) => uuid)
    }

    function closeBulkEdit(): void {
        bulkIds = undefined
        selection.clear()
    }
</script>

<svelte:head><title>{t('constants.hosts')} | Gowave</title></svelte:head>

<svelte:window
    bind:innerWidth={viewport}
    bind:innerHeight={viewportHeight}
    bind:scrollY={pageScroll}
    onkeydown={(event) => {
        if (event.key === 'Escape' && grab) cancelGrab()
    }}
/>

<main class="hosts-page">
    <PageHeader title={t('constants.hosts')} icon="TbListCheck">
        <ActionIcon
            icon="TbQuestionMark"
            tone="lime"
            label={t('help-action-icon.shared.help-article')}
            onclick={() => (helpOpen = true)}
        />
        <EntitySpotlight
            icon="TbListCheck"
            items={ordered.map((host) => ({
                id: host.uuid,
                name: host.remark,
                description: `${host.address} · ${host.uuid}`,
                detail: inboundOf(host),
                icon: host.isDisabled ? 'PiProhibit' : host.isHidden ? 'TbEyeOff' : 'PiPulse',
                tone: host.isDisabled ? 'gray' : host.isHidden ? 'violet' : 'teal',
                keywords: [String(host.port), ...host.tags],
                select: () => (editing = host.uuid)
            }))}
        />
        {#if mode === 'table'}
            <ActionIcon
                icon="TbAdjustmentsHorizontal"
                tone="gray"
                label="Column filters"
                aria-expanded={searchOpen}
                onclick={() => (searchOpen = !searchOpen)}
            />
        {/if}
        <ActionIcon
            icon={mode === 'cards' ? 'TbTable' : 'TbCards'}
            tone="gray"
            label="Toggle view mode"
            onclick={() => view(mode === 'cards' ? 'table' : 'cards')}
        />
        <ActionIcon
            icon="TbRefresh"
            tone="cyan"
            label={t('common.action.update')}
            pending={hosts.isFetching}
            onclick={() => void hosts.refetch()}
        />
        <ActionIcon
            icon="TbPlus"
            tone="teal"
            label={t('common.action.create')}
            onclick={() => (editing = '')}
        />
    </PageHeader>

    {#if hosts.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else if hosts.isError}
        <p role="alert">{hosts.error.message}</p>
    {:else}
        <EntityTagBar items={ordered} active={tag} select={tagsChanged} />
        {#if update.error}
            <p role="alert">{update.error.message}</p>
        {/if}
        {#if !filtered.length && mode === 'cards'}
            <EmptyState icon="TbListCheck" label={t('common.message.nothing-found')} />
        {:else}
            <div
                class="scroll"
                class:cards={mode === 'cards'}
                bind:this={scrollArea}
                bind:clientHeight={height}
                onscroll={(event) => (scrollTop = event.currentTarget.scrollTop)}
            >
                {#if mode === 'cards'}
                    <ul
                        class="card-list"
                        aria-label={t('constants.hosts')}
                        style:height={`${filtered.length * rowHeight}px`}
                        bind:this={cardList}
                    >
                        {#each cardRows as { host, index } (host.uuid)}
                            {@const profile = profiles.data?.configProfiles.find(
                                ({ uuid }) => uuid === host.inbound.configProfileUuid
                            )}
                            {@const inbound = profile?.inbounds.find(
                                ({ uuid }) => uuid === host.inbound.configProfileInboundUuid
                            )}
                            <li
                                class="host-card"
                                class:selected={selection.has(host.uuid)}
                                class:dangling={!profile}
                                class:dragging={drag?.uuid === host.uuid}
                                style:--host-offset={`${cardOffset(index)}px`}
                                style:height={`${rowHeight - (compact ? 12 : 10)}px`}
                            >
                                <div class="selection">
                                    {@render check(host)}
                                    {#if reorderable}
                                        {@render grip(host)}
                                    {/if}
                                </div>
                                <div class="summary">
                                    <div class="host-heading">
                                        {@render statusIcon(host)}
                                        <button
                                            class="host-link mantine-UnstyledButton-root"
                                            type="button"
                                            onclick={() => (editing = host.uuid)}
                                        >
                                            <span class="host-name">{host.remark}</span>
                                            <span class="host-address"
                                                >{host.address}{host.port
                                                    ? `:${host.port}`
                                                    : ''}</span
                                            >
                                        </button>
                                        <div class="host-parameters">
                                            {#if host.internalSquads.squads.length}
                                                {@const allowOnly =
                                                    host.internalSquads.mode ===
                                                    INTERNAL_SQUADS_MODE.ALLOW_ONLY}
                                                <span
                                                    class="squad-badge"
                                                    title={t(
                                                        allowOnly
                                                            ? 'base-host-form.allowed-internal-squads'
                                                            : 'base-host-form.excluded-internal-squads'
                                                    )}
                                                    class:allow-only={allowOnly}
                                                    role="img"
                                                    aria-label={t(
                                                        allowOnly
                                                            ? 'base-host-form.allowed-internal-squads'
                                                            : 'base-host-form.excluded-internal-squads'
                                                    )}
                                                    ><Icon
                                                        name="TbCirclesRelation"
                                                        size={16}
                                                    /></span
                                                >
                                            {/if}
                                            <ActionIcon
                                                icon="XrayLogo"
                                                label={t('base-host-form.xray-json-template')}
                                                tone={host.xrayJsonTemplateUuid ? 'teal' : 'gray'}
                                                size={28}
                                                iconSize={16}
                                                disabled={!host.xrayJsonTemplateUuid}
                                                href={host.xrayJsonTemplateUuid
                                                    ? p('/dashboard/templates/:type/:uuid', {
                                                          params: {
                                                              type: SUBSCRIPTION_TEMPLATE_TYPE.XRAY_JSON,
                                                              uuid: host.xrayJsonTemplateUuid
                                                          }
                                                      })
                                                    : undefined}
                                                target="_blank"
                                                rel="noopener"
                                            />
                                            {#each hostParameters as param (param.field)}
                                                <ActionIcon
                                                    icon={param.icon}
                                                    label={param.label}
                                                    tone={host[param.field] ? 'teal' : 'gray'}
                                                    size={28}
                                                    iconSize={16}
                                                    disabled={host[param.field] == null}
                                                    onclick={() =>
                                                        (hostParameter = {
                                                            title: param.title,
                                                            icon: param.icon,
                                                            value: host[param.field]
                                                        })}
                                                />
                                            {/each}
                                        </div>
                                    </div>
                                    <div class="host-badges">
                                        <span
                                            class="host-profile"
                                            style:color={profile
                                                ? colorHash.hex(profile.uuid)
                                                : 'var(--mantine-color-red-4)'}
                                        >
                                            <Icon
                                                name={profile ? 'XrayLogo' : 'TbAlertCircle'}
                                                size={12}
                                            />
                                            {profile?.name ?? 'DANGLING'}
                                            {#if host.inbound.configProfileInboundUuid}
                                                <span class="separator">›</span
                                                ><span class="inbound"
                                                    >{inbound?.tag ?? 'UNKNOWN'}</span
                                                >
                                            {/if}
                                        </span>
                                        {#each [...host.tags].sort().slice(0, 2) as tag (tag)}
                                            <span class="host-tag" style:color={colorHash.hex(tag)}
                                                ><Icon name="TbStar" size={12} />{tag}</span
                                            >
                                        {/each}
                                        {#if host.tags.length > 2}
                                            <span
                                                class="host-tag"
                                                title={host.tags.slice(2).join(', ')}
                                                >+{host.tags.length - 2}</span
                                            >
                                        {/if}
                                    </div>
                                </div>
                            </li>
                        {/each}
                    </ul>
                {:else}
                    <DataTable class="hosts-table" aria-label={t('constants.hosts')}>
                        <thead>
                            <tr>
                                <th scope="col">
                                    <input
                                        class="mantine-Checkbox-input"
                                        type="checkbox"
                                        aria-label={t('common.action.select-all')}
                                        checked={filtered.every(({ uuid }) => selection.has(uuid))}
                                        onchange={(event) => {
                                            for (const { uuid } of filtered) {
                                                if (event.currentTarget.checked) selection.add(uuid)
                                                else selection.delete(uuid)
                                            }
                                        }}
                                    >
                                </th>
                                {#each Object.keys(columns) as key (key)}
                                    {@const column = key as Column}
                                    {@const label = t(columns[column])}
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
                                            <span>{label}</span>
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
                                        {#if searchOpen}
                                            <TableFilter
                                                {label}
                                                value={filters[column] ?? ''}
                                                live
                                                options={column === 'status'
                                                    ? [
                                                          {
                                                              value: 'enabled',
                                                              label: t(
                                                                  'use-hosts-table-widget.enabled'
                                                              )
                                                          },
                                                          {
                                                              value: 'disabled',
                                                              label: t(
                                                                  'use-hosts-table-widget.disabled'
                                                              )
                                                          },
                                                          {
                                                              value: 'hidden',
                                                              label: t(
                                                                  'use-hosts-table-widget.hidden'
                                                              )
                                                          }
                                                      ]
                                                    : undefined}
                                                onfilter={(value) => {
                                                    filters[column] = value
                                                    scrollToStart()
                                                }}
                                            />
                                        {/if}
                                    </th>
                                {/each}
                                <th scope="col" aria-label={t('common.action.edit')}>
                                    <Icon name="TbEdit" size={18} />
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {#if !filtered.length}
                                <tr>
                                    <td colspan={tableColumnCount}>
                                        {t('common.message.nothing-found')}
                                    </td>
                                </tr>
                            {/if}
                            <tr aria-hidden="true" style:height={`${range.from * rowHeight}px`}>
                                <td colspan={tableColumnCount}></td>
                            </tr>
                            {#each rows as host (host.uuid)}
                                <tr
                                    class:selected={selection.has(host.uuid)}
                                    style:height={`${rowHeight}px`}
                                >
                                    <td>{@render check(host)}</td>
                                    <th scope="row">
                                        <button
                                            class="mantine-UnstyledButton-root table-remark"
                                            type="button"
                                            onclick={() => (editing = host.uuid)}
                                        >
                                            {host.remark}
                                        </button>
                                    </th>
                                    <td>{@render statusIcon(host)}</td>
                                    <td>{host.address}</td>
                                    <td>{host.port}</td>
                                    <td>{host.sni || '–'}</td>
                                    <td>{profileName(host) || '–'}</td>
                                    <td>{inboundTag(host) || '–'}</td>
                                    <td>{host.tags.join(', ')}</td>
                                    <td>
                                        <ActionIcon
                                            icon="TbEdit"
                                            tone="teal"
                                            size={28}
                                            iconSize={18}
                                            label={t('common.action.edit')}
                                            onclick={() => (editing = host.uuid)}
                                        />
                                    </td>
                                </tr>
                            {/each}
                            <tr
                                aria-hidden="true"
                                style:height={`${(filtered.length - range.to) * rowHeight}px`}
                            >
                                <td colspan={tableColumnCount}></td>
                            </tr>
                        </tbody>
                    </DataTable>
                {/if}
            </div>
            {#if mode === 'table'}
                <footer class="table-summary">
                    <Button
                        size="compact-sm"
                        variant="subtle"
                        disabled={!hasFilters && !sort && !tag}
                        onclick={resetTable}
                    >
                        {t('common.action.reset')}
                    </Button>
                    <span role="status">{filtered.length} / {ordered.length}</span>
                </footer>
            {/if}
        {/if}
    {/if}

    {#if helpOpen}
        <HelpDrawer screen="PAGE_HOSTS" close={() => (helpOpen = false)} />
    {/if}
    {#if hostParameter}
        <Overlay
            title={hostParameter.title}
            icon={hostParameter.icon}
            tone="teal"
            width={620}
            close={() => (hostParameter = undefined)}
        >
            <JsonTree value={hostParameter.value} />
        </Overlay>
    {/if}
    {#if bulkIds}
        <Overlay
            drawer
            largePadding
            width={700}
            title={`${t('common.action.bulk-edit')} · ${bulkIds.length}`}
            icon="PiListChecks"
            tone="teal"
            close={() => (bulkIds = undefined)}
        >
            <HostEditor host={undefined} {bulkIds} onsaved={closeBulkEdit} />
        </Overlay>
    {/if}
    {#if confirmation}
        {@const { action, items } = confirmation}
        {@const label = t(actionLabels[action])}
        <HostConfirmation
            title={`${label} · ${items.length}`}
            {label}
            icon={action === 'delete' ? 'PiTrashDuotone' : 'TbCopy'}
            tone={action === 'delete' ? 'red' : 'cyan'}
            description={t('common.message.confirm-action-description')}
            close={() => (confirmation = undefined)}
            confirm={confirmAction}
        />
    {/if}
    {#if editing !== null}
        <HostDialog uuid={editing || undefined} close={() => (editing = null)} />
    {/if}

    {#if selected.length}
        <section class="selection-panel" aria-label={t('common.action.bulk-edit')}>
            <div class="selection-actions">
                <div class="selection-heading">
                    <span class="selection-count">
                        {t('common.message.selected', { count: selected.length })}
                    </span>
                    <div class="selection-controls">
                        <ActionIcon
                            icon="TbSelectAll"
                            label={t('common.action.select-all')}
                            size={34}
                            iconSize={20}
                            subtle
                            onclick={selectAll}
                        />
                        <ActionIcon
                            icon="TbX"
                            label={t('common.action.clear-selection')}
                            size={28}
                            iconSize={16}
                            subtle
                            onclick={() => selection.clear()}
                        />
                    </div>
                </div>
                {#if reorderable}
                    <div class="selection-order">
                        {#each moveActions as { mode, icon, label } (mode)}
                            <Tooltip text={label}>
                                <ActionIcon
                                    {icon}
                                    {label}
                                    tone="gray"
                                    size={34}
                                    iconSize={20}
                                    disabled={update.isPending}
                                    onclick={() => move(mode)}
                                />
                            </Tooltip>
                        {/each}
                    </div>
                {/if}
                <div class="selection-status">
                    {@render bulkAction('enable')}
                    {@render bulkAction('disable')}
                </div>
                <Button
                    variant="soft"
                    color="cyan"
                    icon="TbCategoryPlus"
                    disabled={update.isPending}
                    onclick={openBulkEdit}
                >
                    {t('common.action.update')}
                </Button>
                {@render bulkAction('clone')}
                {@render bulkAction('delete')}
            </div>
        </section>
    {/if}
</main>

{#snippet bulkAction(action: Action)}
    {@const { color, icon } = actionStyles[action]}
    <Button
        {icon}
        {color}
        variant="soft"
        block
        disabled={update.isPending}
        onclick={() => apply(action, selected)}
    >
        {t(actionLabels[action])}
    </Button>
{/snippet}

{#snippet check(host: Host)}
    <input
        class="mantine-Checkbox-input"
        type="checkbox"
        aria-label={`${t('common.action.select')} ${host.remark}`}
        checked={selection.has(host.uuid)}
        onchange={() => select(host.uuid)}
    >
{/snippet}

{#snippet grip(host: Host)}
    <button
        class="mantine-UnstyledButton-root grip"
        type="button"
        disabled={!reorderable || update.isPending}
        aria-label={`Reorder ${host.remark}`}
        title="Drag to reorder"
        onpointerdown={(event) => grabHost(event, host.uuid)}
        onpointermove={moveGrab}
        onpointerup={finishGrab}
        onpointercancel={lostGrab}
        onlostpointercapture={lostGrab}
        onclick={(event) => {
            if (!ignoreGripClick || event.detail === 0) select(host.uuid)
        }}
    >
        <Icon name="RiDraggable" size={24} />
    </button>
{/snippet}

{#snippet statusIcon(host: Host)}
    {@const status = stateOf(host)}
    <span
        class="host-status"
        class:disabled={status === 'disabled'}
        class:hidden={status === 'hidden'}
        title={stateLabel(host)}
    >
        <Icon
            name={status === 'disabled'
                ? 'PiProhibit'
                : status === 'hidden'
                  ? 'TbEyeOff'
                  : 'PiPulse'}
            size={16}
        />
    </span>
{/snippet}

<style>
    .hosts-page {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .selection-panel {
        position: fixed;
        z-index: 100;
        inset-inline-end: 20px;
        inset-block-end: max(20px, env(safe-area-inset-bottom));
        width: min(300px, calc(100vw - 40px));
        max-height: calc(100dvh - var(--page-top) - 20px);
        overflow: auto;
        padding: 4px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-default);
        background: var(--gw-raised-bg);
        box-shadow: var(--mantine-shadow-md);
    }
    .selection-actions {
        display: grid;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-md);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: calc(var(--mantine-radius-default) - 4px);
    }
    .selection-heading,
    .selection-controls {
        display: flex;
        align-items: center;
        justify-content: space-between;
    }
    .selection-count {
        border-radius: 999px;
        padding: 3px 10px;
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        text-transform: uppercase;
    }
    .selection-order,
    .selection-status {
        display: grid;
        grid-template-columns: repeat(4, minmax(0, 1fr));
    }
    .selection-order :global(button) {
        width: 100%;
        border-radius: 0;
    }
    .selection-order > :global(span:first-child) :global(button) {
        border-start-start-radius: var(--mantine-radius-md);
        border-end-start-radius: var(--mantine-radius-md);
    }
    .selection-order > :global(span:last-child) :global(button) {
        border-start-end-radius: var(--mantine-radius-md);
        border-end-end-radius: var(--mantine-radius-md);
    }
    .selection-status {
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-xs);
    }
    @media (prefers-reduced-motion: no-preference) {
        .selection-panel {
            animation: selection-entry 200ms ease-out both;
        }
    }
    @keyframes selection-entry {
        from {
            opacity: 0;
            translate: 0 12px;
        }
    }
    .scroll {
        max-height: min(65vh, 44rem);
        overflow: auto;
        overflow-anchor: none;
        border: 1px solid var(--gw-border);
        border-radius: 0.5rem;
    }
    .card-list {
        position: relative;
        margin: 0;
        padding: 0;
        list-style: none;
    }
    .scroll.cards {
        padding: 0 var(--mantine-spacing-md);
        border: 0;
        max-height: none;
        overflow: visible;
    }

    .host-card {
        position: absolute;
        inset: 0 0 auto;
        transform: translateY(var(--host-offset));
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-md);
        margin-block-end: var(--mantine-spacing-xs);
        border: 1px solid var(--mantine-color-dark-5);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
    }

    .host-card:hover {
        border-color: var(--mantine-color-blue-4);
    }
    .host-card.selected {
        border-color: light-dark(var(--mantine-color-gray-2), var(--mantine-color-blue-5));
    }
    .host-card.dangling {
        border-color: light-dark(var(--mantine-color-gray-2), var(--mantine-color-red-3));
    }
    .host-card.dragging {
        z-index: 2;
        border-color: var(--mantine-color-blue-4);
        box-shadow: var(--mantine-shadow-xl);
    }
    @media (prefers-reduced-motion: no-preference) {
        .host-card:not(.dragging) {
            transition: transform 160ms ease-out;
        }
    }

    .selection,
    .host-heading,
    .host-link,
    .host-badges,
    .host-profile,
    .host-tag,
    .host-parameters {
        display: flex;
        align-items: center;
    }

    .selection {
        gap: var(--mantine-spacing-xs);
    }
    input[type="checkbox"] {
        --gw-checkbox-size: 24px;
    }
    .grip {
        display: grid;
        border: 0;
        background: transparent;
        padding: 0;
        place-items: center;
        color: var(--mantine-color-bright);
        cursor: grab;
    }
    .grip:hover {
        background: var(--mantine-color-default-hover);
    }
    .summary {
        min-width: 0;
        flex: 1;
        display: grid;
        gap: 6px;
    }
    .host-heading,
    .host-link {
        gap: var(--mantine-spacing-sm);
        min-width: 0;
    }
    .host-link {
        flex: 1;
        color: var(--mantine-color-text);
        text-decoration: none;
    }
    .host-link:hover {
        color: var(--mantine-color-text);
    }
    .host-name,
    .host-address,
    .host-badges {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .host-name {
        font-weight: 600;
    }
    .host-address {
        color: var(--mantine-color-dimmed);
        font:
            var(--mantine-font-size-sm) / var(--mantine-line-height-sm)
            var(--mantine-font-family-monospace);
    }
    .squad-badge {
        display: grid;
        place-items: center;
        width: 28px;
        height: 28px;
        border: 1px solid var(--mantine-color-yellow-outline);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-yellow-light-color);
        background: var(--mantine-color-yellow-light);
    }
    .squad-badge.allow-only {
        border-color: var(--mantine-color-teal-outline);
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
    }
    .host-status {
        display: grid;
        place-items: center;
        width: 28px;
        height: 28px;
        flex: 0 0 auto;
        border: 1px solid rgb(18 184 134 / 30%);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-teal-4);
        background: linear-gradient(135deg, rgb(18 184 134 / 15%), rgb(12 166 120 / 10%));
    }
    .host-status.disabled {
        color: var(--mantine-color-gray-4);
        border-color: rgb(134 142 150 / 30%);
        background: rgb(134 142 150 / 14%);
    }
    .host-status.hidden {
        color: var(--mantine-color-violet-4);
        border-color: rgb(121 80 242 / 30%);
        background: rgb(121 80 242 / 14%);
    }
    .host-parameters {
        gap: 6px;
    }
    .host-profile,
    .host-tag {
        height: 20px;
        gap: 5px;
        padding-inline-end: 10px;
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.25px;
        text-transform: uppercase;
    }
    .host-tag {
        padding-inline-start: 10px;
    }
    .inbound {
        opacity: 0.75;
    }
    .separator {
        margin-inline: 1px;
        opacity: 0.5;
    }

    .table-summary {
        display: flex;
        justify-content: flex-end;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        border-block-start: 1px solid var(--mantine-color-default-border);
        font-family: var(--mantine-font-family-monospace);
    }

    :global(.hosts-table) {
        width: 100%;
        border-collapse: collapse;
        text-align: start;
        white-space: nowrap;
    }
    thead {
        position: sticky;
        inset-block-start: 0;
        background: var(--gw-surface);
        z-index: 1;
    }
    tbody tr:not([aria-hidden]) {
        border-block-end: 1px solid var(--gw-border);
        background: var(--gw-surface);
    }
    tbody tr:nth-child(even of :not([aria-hidden])) {
        background: var(--gw-card-bg);
    }
    tbody tr.selected {
        background: var(--mantine-color-cyan-light);
    }
    :global(.hosts-table) tr:not([aria-hidden]) > :not(:last-child) {
        border-inline-end: 1px solid var(--gw-border);
    }
    :global(.hosts-table) tr:not([aria-hidden]) > :last-child {
        position: sticky;
        inset-inline-end: 0;
        background: var(--gw-surface);
        text-align: end;
    }
    .table-remark {
        font-weight: 600;
    }
    .table-remark:hover {
        color: var(--mantine-color-teal-4);
    }
    th,
    td {
        padding-inline: 0.5rem;
        text-align: start;
    }
    thead th {
        height: 40px;
        font-size: var(--mantine-font-size-sm);
        box-shadow: inset 0 -1px var(--gw-border);
    }
    .grip {
        touch-action: none;
        user-select: none;
    }
    .host-card.dragging .grip {
        cursor: grabbing;
    }
    @media (max-width: 47.99em) {
        .scroll.cards {
            padding-inline: 0;
        }

        .host-card {
            display: block;
            padding: var(--mantine-spacing-md);
            margin-block-end: var(--mantine-spacing-sm);
            border-radius: var(--mantine-radius-lg);
        }
        .selection {
            min-height: 34px;
            gap: var(--mantine-spacing-sm);
        }
        .grip {
            width: 32px;
            height: 32px;
            color: var(--mantine-color-dimmed);
        }
        .summary {
            margin-block-start: var(--mantine-spacing-sm);
            gap: var(--mantine-spacing-xs);
        }
        .host-link {
            align-items: flex-start;
            flex-direction: column;
            gap: var(--mantine-spacing-xs);
        }
        .host-name {
            max-width: 100%;
            margin-block-end: 2px;
            color: var(--mantine-color-bright);
            font-size: var(--mantine-font-size-lg);
            line-height: 1.3;
        }
        .host-address {
            max-width: 100%;
            line-height: 1.4;
            opacity: 0.8;
        }
        .host-status {
            position: absolute;
            top: 16px;
            inset-inline-end: 16px;
            width: 34px;
            height: 34px;
        }
        .host-parameters {
            display: none;
        }
        .host-badges {
            flex-wrap: wrap;
            row-gap: var(--mantine-spacing-xs);
        }
        .host-profile {
            flex-basis: 100%;
        }
        .host-tag {
            padding-inline-start: 0;
        }
    }
</style>
