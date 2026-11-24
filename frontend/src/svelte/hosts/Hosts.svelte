<script lang="ts">
    import {
        BulkDeleteHostsCommand,
        BulkDisableHostsCommand,
        BulkEnableHostsCommand,
        CloneHostCommand,
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
    import EmptyState from '../ui/EmptyState.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import PageHeader from '../ui/PageHeader.svelte'
    import { type EntityViewMode, readHostView, writeHostView } from '../view-preferences'
    import HostDialog from './HostDialog.svelte'
    import HostEditor from './HostEditor.svelte'
    import { type Host, hostsKey, hostsQuery } from './queries'

    type Action = 'enable' | 'disable' | 'clone' | 'delete'
    type Column = 'remark' | 'address' | 'port' | 'status' | 'inbound' | 'tags'
    const columns = {
        remark: 'base-host-form.remark',
        address: 'common.field.address',
        port: 'common.field.port',
        inbound: 'common.field.inbounds',
        status: 'common.field.status',
        tags: 'common.field.tags'
    } as const
    const actionLabels = {
        enable: 'common.action.enable',
        disable: 'common.action.disable',
        clone: 'common.action.clone',
        delete: 'common.action.delete'
    } as const
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
    let hostParameter = $state<{ title: string; value: unknown }>()
    let paramsDialog: HTMLDialogElement
    let helpOpen = $state(false)
    let searchOpen = $state(false)
    let editing = $state<string | null>(null)
    let viewport = $state(1024)
    const compact = $derived(viewport < 768)
    const hosts = createQuery(() => hostsQuery)
    const profiles = createQuery(() => profilesQuery)
    const selection = new SvelteSet<string>()
    let t = $derived(locale.t)
    let mode = $state<EntityViewMode>(initial.mode)
    let tag = $state(initial.tag)
    let search = $state('')
    let status = $state('')
    let sort = $state<{ column: Column; descending: boolean }>()
    let notice = $state('')
    let bulkIds = $state.raw<string[] | undefined>()
    let bulkDialog: HTMLDialogElement
    let scrollArea = $state<HTMLDivElement>()
    let height = $state(0)
    let scrollTop = $state(0)
    let dragging = $state('')
    let ordered = $derived(hosts.data ?? [])
    let selected = $derived(ordered.filter(({ uuid }) => selection.has(uuid)))
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
        const normalized = search.trim().toLocaleLowerCase(locale.language)
        const list = ordered.filter(
            (host) =>
                (!tag || (tag === NO_TAG ? host.tags.length === 0 : host.tags.includes(tag))) &&
                (!status || stateOf(host) === status) &&
                [host.remark, host.address, String(host.port), ...host.tags, inboundOf(host)].some(
                    (value) => value.toLocaleLowerCase(locale.language).includes(normalized)
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
    let reorderable = $derived(!sort && !tag && !status && !search.trim())
    let rowHeight = $derived(mode === 'table' ? 50 : compact ? 202 : 86)
    let range = $derived(fixedVirtualWindow(filtered.length, rowHeight, scrollTop, height))
    let rows = $derived(filtered.slice(range.from, range.to))

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
                if (!notice) notice = t('common.message.operation-completed')
            }
        },
        onSettled: () => queryClient.invalidateQueries({ queryKey: hostsKey })
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

    function inspect(title: string, value: unknown): void {
        if (!value) return
        hostParameter = { title, value }
        paramsDialog.showModal()
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

    function inboundOf(host: Host): string {
        const uuid = host.inbound.configProfileInboundUuid
        return uuid ? (inbounds.get(uuid) ?? uuid) : t('common.message.no-inbound-selected')
    }

    function value(host: Host, column: Column): string {
        switch (column) {
            case 'inbound':
                return inboundOf(host)
            case 'status':
                return stateLabel(host)
            case 'tags':
                return host.tags.join(', ')
            default:
                return String(host[column])
        }
    }

    function scrollToStart(): void {
        if (scrollArea) scrollArea.scrollTop = 0
        scrollTop = 0
    }

    function view(next: EntityViewMode): void {
        if (mode === next) return
        mode = next
        sort = undefined
        if (next === 'cards') {
            searchOpen = false
            search = ''
            status = ''
        }
        writeHostView(mode, tag)
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

    function select(uuid: string): void {
        if (selection.has(uuid)) selection.delete(uuid)
        else selection.add(uuid)
    }

    function selectAll(): void {
        for (const { uuid } of ordered) selection.add(uuid)
    }

    function apply(action: Action, items: Host[]): void {
        notice = ''
        update.reset()
        const targets =
            action === 'clone'
                ? items.filter(
                      ({ inbound }) => inbound.configProfileInboundUuid && inbound.configProfileUuid
                  )
                : items
        if (targets.length === 0) {
            notice = t('edit-host-modal.widget.dangling-host-cannot-be-cloned')
            return
        }
        if (action === 'clone' || action === 'delete') {
            const label = t(actionLabels[action])
            if (
                !window.confirm(
                    `${label} (${targets.length})? ${t('common.message.confirm-action-description')}`
                )
            )
                return
        }
        if (targets.length < items.length)
            notice = t('multi-select-hosts.feature.dangling-hosts-skipped', {
                count: items.length - targets.length
            })
        update.mutate({ action, items: targets })
    }

    function saveOrder(next: string[]): void {
        if (next.every((id, index) => id === ordered[index]?.uuid)) return
        notice = ''
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

    function startDrag(event: DragEvent, uuid: string): void {
        if (!reorderable || update.isPending || !event.dataTransfer) return
        dragging = uuid
        event.dataTransfer.effectAllowed = 'move'
        event.dataTransfer.setData('text/plain', uuid)
    }

    function dragOver(event: DragEvent): void {
        if (dragging && reorderable && !update.isPending) event.preventDefault()
    }

    function drop(event: DragEvent, target: string): void {
        if (!dragging || !reorderable || update.isPending) return
        event.preventDefault()
        const source = dragging
        dragging = ''
        if (source === target) return
        const next = ordered.map(({ uuid }) => uuid)
        const from = next.indexOf(source)
        const to = next.indexOf(target)
        if (from === -1 || to === -1) return
        next.splice(from, 1)
        next.splice(to, 0, source)
        saveOrder(next)
    }

    function openBulkEdit(): void {
        bulkIds = selected.map(({ uuid }) => uuid)
        bulkDialog.showModal()
    }

    function closeBulkEdit(): void {
        bulkDialog.close()
        selection.clear()
    }
</script>

<svelte:head><title>{t('constants.hosts')} | Gowave</title></svelte:head>

<svelte:window bind:innerWidth={viewport} />

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
                label={t('common.field.status')}
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
            disabled={hosts.isFetching}
            onclick={() => void hosts.refetch()}
        />
        <ActionIcon
            icon="TbPlus"
            tone="teal"
            label={t('common.action.create')}
            onclick={() => (editing = '')}
        />
    </PageHeader>

    {#if searchOpen && mode === 'table'}
        <div class="button-row toolbar">
            <input
                type="search"
                aria-label={t('common.action.search')}
                placeholder={t('common.action.search')}
                bind:value={search}
                oninput={scrollToStart}
            >
            <select
                aria-label={t('common.field.status')}
                bind:value={status}
                onchange={scrollToStart}
            >
                <option value="">{t('common.field.status')}</option>
                <option value="enabled">{t('use-hosts-table-widget.enabled')}</option>
                <option value="disabled">{t('use-hosts-table-widget.disabled')}</option>
                <option value="hidden">{t('use-hosts-table-widget.hidden')}</option>
            </select>
            <button
                type="button"
                disabled={!ordered.length || selected.length === ordered.length}
                onclick={selectAll}
            >
                {t('common.action.select-all')}
            </button>
        </div>
    {/if}

    {#if hosts.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else if hosts.isError}
        <p role="alert">{hosts.error.message}</p>
    {:else}
        <EntityTagBar items={ordered} active={tag} select={tagsChanged} />
        {#if selected.length}
            <div class="button-row toolbar">
                <button type="button" onclick={() => selection.clear()}>
                    {t('common.action.clear-selection')}
                </button>
                <span>{t('common.message.selected', { count: selected.length })}</span>
                <button type="button" disabled={update.isPending} onclick={openBulkEdit}>
                    {t('common.action.bulk-edit')}
                </button>
                {#each Object.entries(actionLabels) as [action, label] (action)}
                    <button
                        type="button"
                        disabled={update.isPending}
                        onclick={() => apply(action as Action, selected)}
                    >
                        {t(label)}
                    </button>
                {/each}
                {#if reorderable}
                    <button type="button" disabled={update.isPending} onclick={() => move('top')}>
                        Move to top
                    </button>
                    <button type="button" disabled={update.isPending} onclick={() => move('up')}>
                        Move up
                    </button>
                    <button type="button" disabled={update.isPending} onclick={() => move('down')}>
                        Move down
                    </button>
                    <button
                        type="button"
                        disabled={update.isPending}
                        onclick={() => move('bottom')}
                    >
                        Move to bottom
                    </button>
                {/if}
            </div>
        {/if}
        {#if update.error}
            <p role="alert">{update.error.message}</p>
        {/if}
        {#if notice}
            <p role="status">{notice}</p>
        {/if}
        {#if !filtered.length}
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
                    <ul aria-label={t('constants.hosts')}>
                        <li aria-hidden="true" style:height={`${range.from * rowHeight}px`}></li>
                        {#each rows as host (host.uuid)}
                            {@const status = stateOf(host)}
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
                                                <ActionIcon
                                                    icon="TbCirclesRelation"
                                                    label={t(
                                                        'base-host-form.allowed-internal-squads'
                                                    )}
                                                    tone="teal"
                                                    size={28}
                                                    iconSize={16}
                                                />
                                            {/if}
                                            <ActionIcon
                                                icon="XrayLogo"
                                                label={t('base-host-form.xray-json-template')}
                                                tone={host.xrayJsonTemplateUuid ? 'teal' : 'gray'}
                                                size={28}
                                                iconSize={16}
                                                onclick={() => {
                                                    if (host.xrayJsonTemplateUuid) {
                                                        window.open(
                                                            p('/dashboard/templates/:type/:uuid', {
                                                                params: {
                                                                    type: SUBSCRIPTION_TEMPLATE_TYPE.XRAY_JSON,
                                                                    uuid: host.xrayJsonTemplateUuid
                                                                }
                                                            }),
                                                            '_blank',
                                                            'noopener'
                                                        )
                                                    }
                                                }}
                                            />
                                            {#each hostParameters as param (param.field)}
                                                <ActionIcon
                                                    icon={param.icon}
                                                    label={param.label}
                                                    tone={host[param.field] ? 'teal' : 'gray'}
                                                    size={28}
                                                    iconSize={16}
                                                    onclick={() =>
                                                        inspect(param.title, host[param.field])}
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
                        <li
                            aria-hidden="true"
                            style:height={`${(filtered.length - range.to) * rowHeight}px`}
                        ></li>
                    </ul>
                {:else}
                    <table aria-label={t('constants.hosts')}>
                        <thead>
                            <tr>
                                <th scope="col">{t('common.action.select')}</th>
                                {#each Object.entries(columns) as [column, label] (column)}
                                    <th
                                        scope="col"
                                        aria-sort={sort?.column === column
                                            ? sort.descending
                                                ? 'descending'
                                                : 'ascending'
                                            : 'none'}
                                    >
                                        <button
                                            type="button"
                                            onclick={() => sortBy(column as Column)}
                                        >
                                            {t(label)}
                                        </button>
                                    </th>
                                {/each}
                                <th scope="col">{t('common.action.actions')}</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr aria-hidden="true" style:height={`${range.from * rowHeight}px`}>
                                <td colspan="8"></td>
                            </tr>
                            {#each rows as host (host.uuid)}
                                <tr style:height={`${rowHeight}px`}>
                                    <td>{@render check(host)}</td>
                                    <th scope="row">
                                        <a
                                            href={p('/dashboard/open/host/:id', {
                                                params: { id: host.uuid }
                                            })}
                                            >{host.remark}</a
                                        >
                                    </th>
                                    <td>{host.address}</td>
                                    <td>{host.port}</td>
                                    <td>{inboundOf(host)}</td>
                                    <td>{stateLabel(host)}</td>
                                    <td>{host.tags.join(', ')}</td>
                                    <td>{@render itemActions(host)}</td>
                                </tr>
                            {/each}
                            <tr
                                aria-hidden="true"
                                style:height={`${(filtered.length - range.to) * rowHeight}px`}
                            >
                                <td colspan="8"></td>
                            </tr>
                        </tbody>
                    </table>
                {/if}
            </div>
        {/if}
    {/if}

    {#if helpOpen}
        <HelpDrawer screen="PAGE_HOSTS" close={() => (helpOpen = false)} />
    {/if}
    <dialog
        bind:this={paramsDialog}
        aria-labelledby="host-param-heading"
        onclose={() => (hostParameter = undefined)}
    >
        <div class="button-row dialog-header">
            <h2 id="host-param-heading">{hostParameter?.title}</h2>
            <button type="button" onclick={() => paramsDialog.close()}>
                {t('common.action.close')}
            </button>
        </div>
        {#if hostParameter}
            <pre>{JSON.stringify(hostParameter.value, null, 2)}</pre>
        {/if}
    </dialog>
    <dialog
        bind:this={bulkDialog}
        onclose={() => (bulkIds = undefined)}
        aria-labelledby="host-bulk-heading"
    >
        {#if bulkIds}
            <div class="button-row dialog-header">
                <h2 id="host-bulk-heading">{t('common.action.bulk-edit')} · {bulkIds.length}</h2>
                <button type="button" onclick={() => bulkDialog.close()}>
                    {t('common.action.close')}
                </button>
            </div>
            <HostEditor host={undefined} {bulkIds} onsaved={closeBulkEdit} />
        {/if}
    </dialog>
    {#if editing !== null}
        <HostDialog uuid={editing || undefined} close={() => (editing = null)} />
    {/if}
</main>

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
        draggable={reorderable && !update.isPending}
        aria-label={`Reorder ${host.remark}`}
        title="Drag to reorder"
        ondragstart={(event) => startDrag(event, host.uuid)}
        ondragover={dragOver}
        ondrop={(event) => drop(event, host.uuid)}
        ondragend={() => (dragging = '')}
        onclick={() => select(host.uuid)}
    >
        <Icon name="RiDraggable" size={24} />
    </button>
{/snippet}

{#snippet itemActions(host: Host)}
    <div class="button-row actions">
        {@render grip(host)}
        <button
            type="button"
            disabled={update.isPending}
            onclick={() => apply(host.isDisabled ? 'enable' : 'disable', [host])}
        >
            {t(actionLabels[host.isDisabled ? 'enable' : 'disable'])}
        </button>
        <button type="button" disabled={update.isPending} onclick={() => apply('clone', [host])}>
            {t('common.action.clone')}
        </button>
        <button type="button" disabled={update.isPending} onclick={() => apply('delete', [host])}>
            {t('common.action.delete')}
        </button>
    </div>
{/snippet}

<style>
    .hosts-page {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .toolbar {
        align-items: center;
        margin-block-end: 0.75rem;
    }
    .scroll {
        max-height: min(65vh, 44rem);
        overflow: auto;
        overflow-anchor: none;
        border: 1px solid var(--gw-border);
        border-radius: 0.5rem;
    }
    ul {
        margin: 0;
        padding: 0;
        list-style: none;
    }
    .scroll.cards {
        padding: 0 var(--mantine-spacing-md);
        border: 0;
        margin-block-start: -6px;
    }

    .host-card {
        position: relative;
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
        appearance: none;
        display: grid;
        place-items: center;
        width: 24px;
        height: 24px;
        min-height: 24px;
        min-width: 24px;
        margin: 0;
        padding: 0;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-default);
        cursor: pointer;
    }
    input[type="checkbox"]:checked {
        background: var(--mantine-primary-color-filled);
    }
    input[type="checkbox"]:checked::after {
        width: 8px;
        height: 5px;
        border-width: 0 0 2px 2px;
        border-style: solid;
        border-color: #fff;
        transform: translateY(-1px) rotate(-45deg);
        content: "";
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

    table {
        width: 100%;
        border-collapse: collapse;
        text-align: start;
        white-space: nowrap;
    }
    thead {
        position: sticky;
        inset-block-start: 0;
        background: var(--gw-surface);
    }
    tbody tr:not([aria-hidden]) {
        border-block-end: 1px solid var(--gw-border);
    }
    th,
    td {
        padding-inline: 0.5rem;
        text-align: start;
    }
    button[draggable="true"] {
        cursor: grab;
    }
    dialog {
        width: min(calc(100% - 2rem), 58rem);
    }
    .dialog-header {
        justify-content: space-between;
        align-items: center;
        margin-block-end: 1rem;
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
