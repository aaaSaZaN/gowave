<script lang="ts">
    import {
        DeleteInfraBillingNodeCommand,
        DeleteInfraBillingRecordCommand,
        DeleteInfraProviderCommand,
        UpdateInfraBillingNodeCommand
    } from '@remnawave/backend-contract'
    import { createUrl } from '@shared/api/helpers/create-url'
    import { groupRecordsByMonth } from '@shared/lib/group-records-by-month'
    import { faviconResolver } from '@shared/utils/misc/favicon-resolver'
    import { formatCurrency, formatCurrencyWithIntl } from '@shared/utils/misc/number'
    import { formatTimeUtil } from '@shared/utils/time-utils'
    import { createInfiniteQuery, createQuery } from '@tanstack/svelte-query'
    import dayjs from 'dayjs'
    import { MediaQuery, SvelteSet } from 'svelte/reactivity'

    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import { queryClient } from '../query-client'
    import { p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import MetricCard from '../ui/MetricCard.svelte'
    import BillingDialog from './BillingDialog.svelte'
    import {
        type BillingNode,
        billingKey,
        billingNodes,
        billingProviders,
        billingRecords,
        type Provider
    } from './queries'

    const nodes = createQuery(() => billingNodes)
    const providers = createQuery(() => billingProviders)
    const history = createInfiniteQuery(() => billingRecords)
    const narrow = new MediaQuery('(max-width: 64rem)')
    const selected = new SvelteSet<string>()
    let t = $derived(locale.t)
    let selecting = $state(false)
    const selectMode = $derived(selecting && !narrow.current)
    let busy = $state(false)
    let error = $state('')
    let mobile = $state<'nodes' | 'history' | 'providers'>('nodes')
    let mode = $state<
        | { kind: 'node' }
        | { kind: 'provider'; provider?: Provider }
        | { kind: 'record' }
        | { kind: 'date'; nodes: BillingNode[] }
        | undefined
    >()
    let list = $derived(
        [...(nodes.data?.billingNodes ?? [])].sort(
            (a, b) => a.nextBillingAt.getTime() - b.nextBillingAt.getTime()
        )
    )
    let groups = $derived(
        groupRecordsByMonth(
            history.data?.pages.flatMap((page) => page.records) ?? [],
            locale.language
        )
    )
    const scheduled = $derived(
        Map.groupBy(list, (node) => dayjs.utc(node.nextBillingAt).format('YYYY-MM'))
    )
    let selectedNodes = $derived(list.filter((node) => selected.has(node.uuid)))
    const today = dayjs.utc().startOf('day')
    let overdue = $derived(
        list.filter((node) => dayjs.utc(node.nextBillingAt).isBefore(today)).length
    )
    let upcomingMonth = $derived(
        new Intl.DateTimeFormat(locale.language, { month: 'long' }).format(new Date())
    )
    const refreshing = $derived(nodes.isFetching || providers.isFetching || history.isFetching)

    function dateText(time: Date): string {
        return formatTimeUtil({ time, template: 'FULL_DATE', language: locale.language, utc: true })
    }

    function refreshAll(): void {
        void Promise.all([nodes.refetch(), providers.refetch(), history.refetch()])
    }

    function loadMore(): void {
        if (history.hasNextPage && !history.isFetching) void history.fetchNextPage()
    }

    async function run(action: () => Promise<unknown>): Promise<void> {
        if (busy) return
        busy = true
        error = ''
        try {
            await action()
        } catch (failure) {
            error = failure instanceof Error ? failure.message : t('common.message.error')
        } finally {
            await queryClient.invalidateQueries({ queryKey: billingKey })
            busy = false
        }
    }

    function remove(
        command:
            | typeof DeleteInfraBillingNodeCommand
            | typeof DeleteInfraProviderCommand
            | typeof DeleteInfraBillingRecordCommand,
        uuids: string[]
    ): void {
        if (!window.confirm(t('common.message.confirm-action-description'))) return
        void run(async () => {
            const results = await Promise.allSettled(
                uuids.map(async (uuid) => {
                    const params = command.RequestParamSchema.parse({ uuid })
                    await executeCommand({
                        ...command,
                        TSQ_url: createUrl(command.TSQ_url, undefined, params)
                    })
                    selected.delete(uuid)
                })
            )
            const failure = results.find((result) => result.status === 'rejected')
            if (failure) throw failure.reason
        })
    }

    function safeLink(url: string | null): string | undefined {
        return url && /^https?:\/\//i.test(url) ? url : undefined
    }

    function advance(node: BillingNode): void {
        void run(() =>
            submitCommand(UpdateInfraBillingNodeCommand, {
                uuids: [node.uuid],
                nextBillingAt: dayjs(node.nextBillingAt).add(1, 'month').toISOString()
            })
        )
    }
</script>

{#snippet iconLink(href: string, label: string, icon: IconName, tone: string, external = false)}
    <a
        {href}
        aria-label={label}
        title={label}
        class="billing-icon icon-link"
        style:--billing-tone={`var(--mantine-color-${tone}-6)`}
        style:--billing-icon={`var(--mantine-color-${tone}-4)`}
        target={external ? '_blank' : undefined}
        rel={external ? 'noopener noreferrer' : undefined}
    >
        <Icon name={icon} size={18} />
    </a>
{/snippet}

{#snippet providerIcon(name: string, favicon: string | null, tone: 'teal' | 'violet')}
    {@const src = faviconResolver(favicon)}
    <span
        class="billing-icon"
        style:--billing-tone={`var(--mantine-color-${tone}-6)`}
        style:--billing-icon={`var(--mantine-color-${tone}-4)`}
        aria-hidden="true"
    >
        <Icon name={tone === 'teal' ? 'TbCreditCard' : 'TbCloud'} size={20} />
    </span>
    <span class="provider-initials" aria-hidden="true">
        {name
            .split(/\s+/)
            .slice(0, 2)
            .map((word) => word.charAt(0))
            .join('')
            .toUpperCase()}
        {#if src}
            <img
                {src}
                alt=""
                loading="lazy"
                decoding="async"
                onerror={(event) => {
                    if (event.currentTarget instanceof HTMLImageElement) {
                        event.currentTarget.hidden = true
                    }
                }}
                onload={(event) => {
                    const image = event.currentTarget
                    if (
                        image instanceof HTMLImageElement &&
                        image.naturalWidth <= 24 &&
                        image.naturalHeight <= 24
                    )
                        image.hidden = true
                }}
            >
        {/if}
    </span>
{/snippet}

<svelte:head><title>{t('constants.infra-billing')} | Gowave</title></svelte:head>
<main>
    {#if error}
        <p role="alert">{error}</p>
    {/if}
    <div class="metrics">
        <MetricCard
            label={t('common.message.overdue')}
            mobileLabel={t('common.message.overdue')}
            value={overdue}
            icon="TbAlertTriangle"
            color={overdue ? 'red' : 'teal'}
            subtitle={t(overdue ? 'stats.widget.payment-overdue' : 'stats.widget.all-up-to-date')}
        />
        <MetricCard
            label={t('stats.widget.upcoming-in', { month: upcomingMonth })}
            mobileLabel={t('mobile-stats.widget.upcoming')}
            value={nodes.data?.stats.upcomingNodesCount ?? 0}
            icon="FaServer"
            color="orange"
            subtitle={t('stats.widget.nodes-pending-payment')}
        />
        <MetricCard
            label={t('stats.widget.payments-in', { month: upcomingMonth })}
            mobileLabel={t('mobile-stats.widget.per-month')}
            value={formatCurrency(nodes.data?.stats.currentMonthPayments ?? 0)}
            icon="MdPayment"
            color="green"
            subtitle={t('stats.widget.total-payments-made')}
        />
        <MetricCard
            label={t('stats.widget.total-spent')}
            mobileLabel={t('mobile-stats.widget.total-spent')}
            value={formatCurrency(nodes.data?.stats.totalSpent ?? 0)}
            icon="MdTrendingUp"
            color="violet"
            subtitle={t('stats.widget.lifetime-spending')}
        />
    </div>
    <nav class="button-row mobile" aria-label={t('constants.infra-billing')}>
        <button type="button" aria-pressed={mobile === 'nodes'} onclick={() => (mobile = 'nodes')}>
            <Icon name="TbServer" size={16} /> {t('constants.nodes')}
        </button>
        <button
            type="button"
            aria-pressed={mobile === 'history'}
            onclick={() => (mobile = 'history')}
        >
            <Icon name="TbCreditCard" size={16} /> {t('mobile-infra-billing.widget.history')}
        </button>
        <button
            type="button"
            aria-pressed={mobile === 'providers'}
            onclick={() => (mobile = 'providers')}
        >
            <Icon name="TbCloud" size={16} /> {t('mobile-infra-billing.widget.providers')}
        </button>
    </nav>
    <div class="mobile-actions">
        <ActionIcon
            icon="TbRefresh"
            tone="cyan"
            size={40}
            iconSize={22}
            label={t('common.action.refresh')}
            onclick={refreshAll}
            disabled={refreshing}
        />
        <ActionIcon
            icon="TbPlus"
            tone="teal"
            size={40}
            iconSize={22}
            label={t('common.action.create')}
            onclick={() =>
                (mode = {
                    kind: mobile === 'history' ? 'record' : mobile === 'nodes' ? 'node' : 'provider'
                })}
        />
    </div>
    <div class="columns">
        <section class:expanded={mobile === 'nodes'} aria-labelledby="billing-nodes">
            <header class="column-header">
                <h2 id="billing-nodes">
                    <Icon name="TbServer" size={18} />{t('constants.nodes')}
                    <span class="count">{list.length}</span>
                </h2>
                <div class="column-actions">
                    {#if selecting}
                        <span class="count">{selected.size}</span>
                        <ActionIcon
                            icon="PiTrashDuotone"
                            tone="red"
                            size={30}
                            iconSize={18}
                            label={t('common.action.delete')}
                            onclick={() =>
                                remove(
                                    DeleteInfraBillingNodeCommand,
                                    selectedNodes.map((node) => node.uuid)
                                )}
                            disabled={busy || !selectedNodes.length}
                        /><ActionIcon
                            icon="TbCalendar"
                            tone="teal"
                            size={30}
                            iconSize={18}
                            label={t('infra-billing-nodes.widget.update-multiple-nodes')}
                            onclick={() => (mode = { kind: 'date', nodes: selectedNodes })}
                            disabled={busy || !selectedNodes.length}
                        /><ActionIcon
                            icon="TbX"
                            tone="gray"
                            size={30}
                            iconSize={18}
                            label={t('common.action.close')}
                            onclick={() => {
                                selecting = false
                                selected.clear()
                            }}
                            subtle
                        />
                    {:else}
                        <ActionIcon
                            icon="TbListCheck"
                            tone="gray"
                            size={30}
                            iconSize={18}
                            label={t('common.action.select')}
                            onclick={() => (selecting = true)}
                            subtle
                        /><ActionIcon
                            icon="TbRefresh"
                            tone="cyan"
                            size={30}
                            iconSize={18}
                            label={t('common.action.refresh')}
                            disabled={nodes.isFetching}
                            onclick={() => void nodes.refetch()}
                        /><ActionIcon
                            icon="TbPlus"
                            tone="teal"
                            size={30}
                            iconSize={18}
                            label={t('infra-billing-nodes.widget.add-a-node')}
                            onclick={() => (mode = { kind: 'node' })}
                        />
                    {/if}
                </div>
            </header>
            {#if nodes.isPending}
                <p role="status">{t('common.message.loading')}</p>
            {/if}
            {#if nodes.error}
                <p role="alert">{nodes.error.message}</p>
            {/if}
            {#if nodes.isSuccess && list.length === 0}
                <EmptyState
                    icon="TbServer"
                    label={t('infra-billing-nodes.widget.no-nodes-found')}
                />
            {/if}
            {#if scheduled.size}
                <div class="column-scroll schedule">
                    {#each scheduled as [month, group] (month)}
                        <div class="card-list">
                            <h3 class="month centered">
                                {dayjs.utc(month).locale(locale.language).format('MMMM YYYY')}
                            </h3>
                            <ul class="card-list">
                                {#each group as node (node.uuid)}
                                    {@const name = node.node?.name ?? node.name ?? node.uuid}
                                    {@const url = safeLink(node.provider.loginUrl)}
                                    {@const tone = dayjs.utc(node.nextBillingAt).isBefore(today)
                                        ? 'red'
                                        : 'teal'}
                                    <li
                                        class="billing-card"
                                        class:selected={selectMode && selected.has(node.uuid)}
                                    >
                                        <div class="card-row">
                                            <div class="entity">
                                                {#if selectMode}
                                                    <input
                                                        type="checkbox"
                                                        aria-label={name}
                                                        checked={selected.has(node.uuid)}
                                                        onchange={(event) =>
                                                            event.currentTarget.checked
                                                                ? selected.add(node.uuid)
                                                                : selected.delete(node.uuid)}
                                                    >
                                                {/if}
                                                {#if node.node}
                                                    <span aria-hidden="true" class="country"
                                                        >{flag(node.node.countryCode)}</span
                                                    >
                                                {:else}
                                                    <span class="billing-icon node-icon"
                                                        ><Icon name="TbServer" size={20} /></span
                                                    >
                                                {/if}
                                                <div class="entity-text">
                                                    <strong title={name}>{name}</strong>
                                                    <small>{node.provider.name}</small>
                                                </div>
                                            </div>
                                            {#if !selectMode}
                                                <div class="card-actions">
                                                    {#if node.node}
                                                        {@render iconLink(
                                                            p('/dashboard/open/node/:id', {
                                                                params: { id: node.node.uuid }
                                                            }),
                                                            t('common.action.open'),
                                                            'TbCpu',
                                                            'blue'
                                                        )}
                                                    {/if}
                                                    {#if url}
                                                        {@render iconLink(
                                                            url,
                                                            node.provider.name,
                                                            'TbExternalLink',
                                                            'gray',
                                                            true
                                                        )}
                                                    {/if}
                                                    <ActionIcon
                                                        icon="TbCheck"
                                                        tone="teal"
                                                        size={30}
                                                        iconSize={18}
                                                        label={t(
                                                            'update-billing-date-modal.widget.next-month'
                                                        )}
                                                        disabled={busy}
                                                        onclick={() => advance(node)}
                                                    />
                                                </div>
                                            {/if}
                                        </div>
                                        <div
                                            class="card-row card-section"
                                            style:--status-ink={`var(--mantine-color-${tone}-5)`}
                                            style:--status-tint={`var(--mantine-color-${tone}-6)`}
                                        >
                                            <button
                                                type="button"
                                                class="billing-date mantine-UnstyledButton-root"
                                                title={t(
                                                    'update-billing-date-modal.widget.update-billing-date'
                                                )}
                                                disabled={selectMode}
                                                onclick={() =>
                                                    (mode = { kind: 'date', nodes: [node] })}
                                            >
                                                <Icon name="TbCalendar" size={16} />
                                                {dateText(node.nextBillingAt)}
                                            </button>
                                            <span class="status-badge">
                                                <Icon name="TbCreditCard" size={14} />
                                                {dayjs(node.nextBillingAt)
                                                    .startOf('day')
                                                    .locale(locale.language)
                                                    .fromNow()}
                                            </span>
                                        </div>
                                    </li>
                                {/each}
                            </ul>
                        </div>
                    {/each}
                </div>
            {/if}
        </section>
        <section class:expanded={mobile === 'history'} aria-labelledby="billing-history">
            <header class="column-header">
                <h2 id="billing-history">
                    <Icon name="TbCreditCard" size={18} />{t('mobile-infra-billing.widget.history')}
                    <span class="count">{history.data?.pages[0]?.total ?? 0}</span>
                </h2>
                <div class="column-actions">
                    <ActionIcon
                        icon="TbRefresh"
                        tone="cyan"
                        size={30}
                        iconSize={18}
                        label={t('common.action.refresh')}
                        disabled={history.isFetching}
                        onclick={() => void history.refetch()}
                    /><ActionIcon
                        icon="TbPlus"
                        tone="teal"
                        size={30}
                        iconSize={18}
                        label={t('create-infra-billing-record.modal.widget.bill-record')}
                        onclick={() => (mode = { kind: 'record' })}
                    />
                </div>
            </header>
            {#if history.isPending}
                <p role="status">{t('common.message.loading')}</p>
            {/if}
            {#if history.error}
                <p role="alert">{history.error.message}</p>
            {/if}
            {#if history.isSuccess && groups.length === 0}
                <EmptyState
                    icon="TbCreditCard"
                    label={t('infra-billing-records-table.widget.no-billing-records-found')}
                />
            {/if}
            {#if groups.length}
                <div
                    class="column-scroll history-scroll card-list"
                    onscroll={(event) => {
                        const scroll = event.currentTarget
                        if (scroll.scrollHeight - scroll.scrollTop - scroll.clientHeight < 120)
                            loadMore()
                    }}
                >
                    {#each groups as group (group.label)}
                        <h3 class="month">
                            {group.label}
                            <span class="month-line"></span>
                            <strong>{formatCurrencyWithIntl(group.total)}</strong>
                        </h3>
                        <ul class="card-list">
                            {#each group.records as record (record.uuid)}
                                <li class="billing-card card-row">
                                    <div class="entity">
                                        {@render providerIcon(
                                            record.provider.name,
                                            record.provider.faviconLink,
                                            'teal'
                                        )}
                                        <div class="entity-text">
                                            <strong title={record.provider.name}
                                                >{record.provider.name}</strong
                                            >
                                            <small>{dateText(record.billedAt)}</small>
                                        </div>
                                    </div>
                                    <div class="card-actions record-amount">
                                        <strong>{formatCurrencyWithIntl(record.amount)}</strong>
                                        <ActionIcon
                                            icon="TbTrash"
                                            tone="red"
                                            size={30}
                                            iconSize={16}
                                            label={t('common.action.delete')}
                                            disabled={busy}
                                            onclick={() =>
                                                remove(DeleteInfraBillingRecordCommand, [
                                                    record.uuid
                                                ])}
                                        />
                                    </div>
                                </li>
                            {/each}
                        </ul>
                    {/each}
                    {#if history.hasNextPage}
                        <button type="button" disabled={history.isFetching} onclick={loadMore}>
                            {t('common.action.load-more', { defaultValue: 'Load more' })}
                        </button>
                    {/if}
                </div>
            {/if}
        </section>
        <section class:expanded={mobile === 'providers'} aria-labelledby="billing-providers">
            <header class="column-header">
                <h2 id="billing-providers">
                    <Icon name="TbCloud" size={18} />
                    {t('mobile-infra-billing.widget.providers')}
                    <span class="count">{providers.data?.total ?? 0}</span>
                </h2>
                <div class="column-actions">
                    <ActionIcon
                        icon="TbRefresh"
                        tone="cyan"
                        size={30}
                        iconSize={18}
                        label={t('common.action.refresh')}
                        disabled={providers.isFetching}
                        onclick={() => void providers.refetch()}
                    /><ActionIcon
                        icon="TbPlus"
                        tone="teal"
                        size={30}
                        iconSize={18}
                        label={t('common.action.create')}
                        onclick={() => (mode = { kind: 'provider' })}
                    />
                </div>
            </header>
            {#if providers.isPending}
                <p role="status">{t('common.message.loading')}</p>
            {/if}
            {#if providers.error}
                <p role="alert">{providers.error.message}</p>
            {/if}
            {#if providers.isSuccess && !providers.data.total}
                <EmptyState
                    icon="TbCloud"
                    label={t('infra-providers-table.widget.no-providers-found')}
                />
            {/if}
            <ul class="column-scroll card-list">
                {#each providers.data?.providers ?? [] as provider (provider.uuid)}
                    {@const url = safeLink(provider.loginUrl)}
                    <li class="billing-card">
                        <div class="card-row">
                            <div class="entity">
                                {@render providerIcon(
                                    provider.name,
                                    provider.faviconLink,
                                    'violet'
                                )}
                                <div class="entity-text">
                                    <strong title={provider.name}>{provider.name}</strong>
                                </div>
                            </div>
                            <div class="card-actions">
                                {#if url}
                                    {@render iconLink(url, provider.name, 'TbLink', 'teal', true)}
                                {/if}
                                <ActionIcon
                                    icon="TbEdit"
                                    tone="blue"
                                    size={30}
                                    iconSize={16}
                                    label={t('common.action.edit')}
                                    onclick={() => (mode = { kind: 'provider', provider })}
                                />
                                <ActionIcon
                                    icon="TbTrash"
                                    tone="red"
                                    size={30}
                                    iconSize={16}
                                    label={t('common.action.delete')}
                                    disabled={busy}
                                    onclick={() =>
                                        remove(DeleteInfraProviderCommand, [provider.uuid])}
                                />
                            </div>
                        </div>
                        <dl class="provider-stats card-section">
                            <div>
                                <dd>{provider.billingNodes.length}</dd>
                                <dt>{t('constants.nodes')}</dt>
                            </div>
                            <div>
                                <dd>{provider.billingHistory.totalBills}</dd>
                                <dt>{t('mobile-providers-list.widget.invoices')}</dt>
                            </div>
                            <div>
                                <dd class="total">
                                    {formatCurrencyWithIntl(provider.billingHistory.totalAmount)}
                                </dd>
                                <dt>{t('common.field.total')}</dt>
                            </div>
                        </dl>
                        {#if provider.billingNodes.length}
                            <div class="provider-nodes card-section">
                                {#snippet linkedNode(node: Provider['billingNodes'][number])}
                                    {#if node.details}
                                        <a
                                            class="node-badge"
                                            href={p('/dashboard/open/node/:id', {
                                                params: { id: node.details.nodeUuid }
                                            })}
                                        >
                                            <span aria-hidden="true"
                                                >{flag(node.details.countryCode)}</span
                                            >{node.name}
                                        </a>
                                    {:else}
                                        <span class="node-badge"
                                            ><Icon name="TbServer" size={18} />{node.name}</span
                                        >
                                    {/if}
                                {/snippet}
                                {#each provider.billingNodes.slice(0, 3) as node (node)}
                                    {@render linkedNode(node)}
                                {/each}
                                {#if provider.billingNodes.length > 3}
                                    <details class="additional-nodes">
                                        <summary class="node-badge">
                                            +{provider.billingNodes.length - 3}
                                        </summary>
                                        {#each provider.billingNodes.slice(3) as node (node)}
                                            {@render linkedNode(node)}
                                        {/each}
                                    </details>
                                {/if}
                            </div>
                        {/if}
                    </li>
                {/each}
            </ul>
        </section>
    </div>
</main>

{#if mode}
    <BillingDialog
        {mode}
        providers={providers.data?.providers ?? []}
        available={nodes.data?.availableBillingNodes ?? []}
        close={() => (mode = undefined)}
    />
{/if}

<style>
    main,
    .metrics,
    .columns {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .metrics {
        grid-template-columns: repeat(4, minmax(0, 1fr));
    }
    .columns {
        grid-template-columns: repeat(3, minmax(0, 1fr));
        align-items: start;
        margin-top: var(--mantine-spacing-md);
    }
    section {
        min-width: 0;
        overflow-wrap: anywhere;
    }
    .column-header,
    h2,
    .column-actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .column-header {
        justify-content: space-between;
        min-height: 34px;
        margin-bottom: var(--mantine-spacing-xs);
    }
    h2 {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }
    .count {
        padding: 2px 8px;
        border: 1px solid rgb(134 142 150 / 30%);
        border-radius: var(--mantine-radius-sm);
        background: rgb(134 142 150 / 12%);
        color: var(--mantine-color-gray-4);
        font: 600 var(--mantine-font-size-xs) var(--mantine-font-family-monospace);
    }
    .column-scroll {
        min-width: 0;
        max-height: calc(100vh - 320px);
        padding-inline-end: var(--mantine-spacing-sm);
        overflow: auto;
        overscroll-behavior: contain;
    }
    .schedule,
    .card-list {
        display: grid;
        gap: var(--mantine-spacing-xs);
        align-content: start;
    }
    .schedule {
        gap: var(--mantine-spacing-md);
    }
    .history-scroll,
    .history-scroll .card-list {
        gap: 8px;
    }
    .history-scroll {
        padding-inline-end: 0;
    }
    ul.card-list {
        padding: 0;
        margin: 0;
        list-style: none;
    }
    ul.column-scroll.card-list {
        padding-inline-end: var(--mantine-spacing-sm);
    }
    .billing-card {
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        background: var(--gw-raised-bg);
        border-radius: var(--mantine-radius-md);
    }
    .billing-card.selected {
        border-color: var(--mantine-color-cyan-filled);
    }
    .card-row,
    .entity,
    .card-actions,
    .billing-date,
    .status-badge,
    .month,
    .provider-nodes {
        display: flex;
        align-items: center;
    }
    .card-row {
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
    }
    .entity {
        min-width: 0;
        gap: var(--mantine-spacing-sm);
    }
    .card-actions {
        flex: none;
        gap: 4px;
    }
    .record-amount {
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-teal-5);
        font-size: var(--mantine-font-size-sm);
        white-space: nowrap;
    }
    .entity-text {
        display: grid;
        min-width: 0;
    }
    .entity-text strong,
    .entity-text small {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }
    .entity-text strong {
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h4-font-size);
        line-height: var(--mantine-h4-line-height);
        font-weight: 600;
    }
    .entity-text small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .billing-icon {
        display: inline-grid;
        flex: none;
        width: 34px;
        height: 34px;
        place-items: center;
        border-radius: var(--mantine-radius-md);
        border: 1px solid color-mix(in srgb, var(--billing-tone) 30%, transparent);
        background: color-mix(in srgb, var(--billing-tone) 12%, transparent);
        color: var(--billing-icon);
    }
    .country {
        font-size: 24px;
        flex: none;
    }
    .node-icon {
        --billing-tone: var(--mantine-color-blue-6);
        --billing-icon: var(--mantine-color-blue-4);
    }
    .icon-link {
        width: 30px;
        height: 30px;
    }
    .icon-link:hover {
        background: color-mix(in srgb, var(--billing-tone) 22%, transparent);
    }
    .provider-initials {
        display: grid;
        flex: none;
        position: relative;
        place-items: center;
        width: 18px;
        height: 18px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-blue-light);
        color: var(--mantine-color-blue-light-color);
        font-size: 8px;
        font-weight: 700;
        overflow: hidden;
    }
    .provider-initials img {
        position: absolute;
        width: 100%;
        height: 100%;
        object-fit: cover;
    }
    .card-section {
        margin-block-start: var(--mantine-spacing-md);
        padding-block-start: var(--mantine-spacing-md);
        border-block-start: 1px solid
            color-mix(in srgb, var(--mantine-color-default-border) 30%, transparent);
    }
    .billing-date,
    .status-badge {
        gap: 6px;
        color: var(--status-ink);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        font-weight: 600;
    }
    .status-badge,
    .node-badge {
        min-height: 18px;
        padding-inline: 8px;
        border-radius: var(--mantine-radius-sm);
        border: 1px solid color-mix(in srgb, var(--status-tint) 30%, transparent);
        background: color-mix(in srgb, var(--status-tint) 12%, transparent);
        white-space: nowrap;
        font-size: 10px;
        font-weight: 700;
        line-height: 1.2;
        text-transform: uppercase;
        letter-spacing: 0.25px;
    }
    .month {
        margin: 0;
        gap: var(--mantine-spacing-sm);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
        font-weight: 600;
        text-transform: capitalize;
        white-space: nowrap;
    }
    .month.centered {
        gap: var(--mantine-spacing-xs);
    }
    .month strong,
    .provider-stats .total {
        color: var(--mantine-color-teal-5);
        font-weight: 700;
    }
    .month-line,
    .month.centered::before,
    .month.centered::after {
        flex: 1;
        height: 1px;
        background: var(--mantine-color-default-border);
        content: "";
    }
    .provider-stats {
        display: grid;
        gap: var(--mantine-spacing-md);
        grid-template-columns: repeat(3, minmax(0, 1fr));
        text-align: center;
        margin-block-end: 0;
    }
    .provider-stats dd {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        font-weight: 700;
    }
    .provider-stats dt {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .provider-nodes {
        gap: 4px;
        flex-wrap: wrap;
    }
    .node-badge {
        --status-tint: var(--mantine-color-gray-6);
        display: inline-flex;
        align-items: center;
        gap: 5px;
        min-height: 20px;
        padding-inline: 10px;
        border-radius: var(--mantine-radius-md);
        font-size: 11px;
        color: var(--mantine-color-gray-4);
        text-decoration: none;
    }
    .additional-nodes[open] {
        flex-basis: 100%;
    }
    .additional-nodes summary {
        cursor: pointer;
    }
    section :global(.empty h2) {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-md);
        line-height: var(--mantine-line-height-md);
    }
    .mobile,
    .mobile-actions {
        display: none;
    }
    @media (64rem < width < 75em) {
        .columns {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (max-width: 64rem) {
        .metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: var(--mantine-spacing-xs);
        }
        .columns {
            grid-template-columns: minmax(0, 1fr);
            margin-top: 0;
        }
        .column-scroll,
        ul.column-scroll.card-list {
            max-height: none;
            padding-inline-end: 0;
            overflow: visible;
        }
        .history-scroll {
            max-height: calc(100dvh - 280px);
            overflow: auto;
        }
        .mobile {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 8px;
            margin-block-start: var(--mantine-spacing-xs);
        }
        .mobile button {
            display: grid;
            grid-template-columns: 16px 1fr;
            align-items: center;
            gap: var(--mantine-spacing-xs);
            height: 38px;
            padding-inline: var(--mantine-spacing-md);
            border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
            border-radius: var(--mantine-radius-md);
            background: var(--gw-raised-bg);
            color: var(--mantine-color-text);
            font-size: var(--mantine-font-size-sm);
            font-weight: 500;
        }
        .mobile button:last-child {
            grid-column: 1 / -1;
        }
        .mobile button[aria-pressed="true"] {
            border-color: rgb(21 170 191 / 30%);
            background: var(--mantine-color-cyan-light);
            color: var(--mantine-color-cyan-4);
        }
        section:not(.expanded) {
            display: none;
        }
        .column-header {
            display: none;
        }
        .mobile-actions {
            display: flex;
            gap: var(--mantine-spacing-xs);
            position: fixed;
            z-index: 100;
            inset-inline-end: 15px;
            bottom: max(var(--mantine-spacing-md), env(safe-area-inset-bottom));
            padding: 6px;
            border-radius: var(--mantine-radius-md);
            background: var(--mantine-color-dark-6);
            box-shadow: 0 4px 12px rgb(0 0 0 / 40%);
        }
    }
</style>
