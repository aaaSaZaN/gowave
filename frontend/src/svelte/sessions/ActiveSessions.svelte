<script lang="ts">
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import {
        dropUserSessions,
        loadNodeSessions,
        loadUserSessions,
        type SessionDrop,
        type UserSessionNode,
        type UserSessionProgress
    } from './queries'
    import SessionIps from './SessionIps.svelte'
    import SessionScanProgress from './SessionScanProgress.svelte'
    import { sessionActivityCounts } from './scan'

    type Target =
        | { kind: 'node'; uuid: string; name: string }
        | { kind: 'user'; id: number; name: string }
    type NodeUser = Awaited<ReturnType<typeof loadNodeSessions>>[number]

    let { target, close }: { target: Target; close: () => void } = $props()
    let controller: AbortController | undefined
    let t = $derived(locale.t)
    let loading = $state(true)
    let sending = $state(false)
    let error = $state('')
    let actionError = $state('')
    let notice = $state('')
    let progress = $state<UserSessionProgress>()
    let nodes = $state.raw<UserSessionNode[]>([])
    let users = $state.raw<NodeUser[]>([])
    let sort = $state('default')
    let requirements = $state(false)
    let page = $state(0)
    const pageSize = 24
    let sortedUsers = $derived(
        sort === 'default'
            ? users
            : [...users].sort((a, b) =>
                  sort === 'asc' ? a.ips.length - b.ips.length : b.ips.length - a.ips.length
              )
    )
    let ips = $derived((target.kind === 'user' ? nodes : users).flatMap((entry) => entry.ips))
    let distinctIps = $derived(new Set(ips.map(({ ip }) => ip)).size)
    let now = $state(Date.now())
    let counts = $derived(sessionActivityCounts(ips, now))
    let format = $derived(new Intl.NumberFormat(locale.language))

    onMount(() => {
        void refresh()
        const clock = window.setInterval(() => (now = Date.now()), 30_000)
        return () => {
            window.clearInterval(clock)
            controller?.abort()
        }
    })

    async function refresh(): Promise<void> {
        controller?.abort()
        const request = new AbortController()
        controller = request
        const started = performance.now()
        const { signal } = request
        loading = true
        error = ''
        progress = undefined
        page = 0
        users = []
        nodes = []
        try {
            if (target.kind === 'node') {
                const result = await loadNodeSessions(target, signal)
                if (!signal.aborted) users = result
            } else {
                const result = await loadUserSessions(target.id, signal, (value) => {
                    progress = value
                })
                if (!signal.aborted) nodes = result
            }
        } catch (failure) {
            if (!signal.aborted) error = text(failure)
        } finally {
            const remaining = 450 - (performance.now() - started)
            if (!signal.aborted && remaining > 0)
                await new Promise<void>((resolve) => setTimeout(resolve, remaining))
            if (controller === request && !signal.aborted) {
                controller = undefined
                loading = false
            }
        }
    }

    function text(failure: unknown): string {
        return failure instanceof Error ? failure.message : t('common.message.error')
    }

    function dropLabel(selection: SessionDrop): string {
        switch (selection.kind) {
            case 'all':
                return t('user-active-session-drawer.widget.drop-all-user-connections-all-nodes')
            case 'node':
                return t('user-active-session-drawer.widget.drop-all-user-connections-this-node')
            case 'ip':
                return t('user-active-session-drawer.widget.drop-this-connection-on-this-node')
        }
    }

    async function drop(selection: SessionDrop, detail: string): Promise<void> {
        if (target.kind !== 'user' || sending || loading) return
        if (!window.confirm(`${dropLabel(selection)} — ${detail}`)) return
        sending = true
        actionError = ''
        notice = ''
        try {
            await dropUserSessions(target.id, selection)
            notice = t('common.message.event-sent')
        } catch (failure) {
            actionError = text(failure)
        } finally {
            sending = false
        }
    }

    function copy(ip: string): void {
        notice = ''
        actionError = ''
        void navigator.clipboard.writeText(ip).then(
            () => (notice = t('common.message.copied')),
            (failure: unknown) => (actionError = text(failure))
        )
    }
</script>

<Overlay
    title={t('common.field.active-sessions')}
    icon="TbRadar"
    tone="teal"
    width={500}
    drawer
    {close}
>
    {#if actionError}
        <p role="alert">{actionError}</p>
    {/if}
    {#if notice}
        <p role="status">{notice}</p>
    {/if}
    {#if loading}
        <SessionScanProgress
            completed={progress?.completed ?? 0}
            total={progress?.total ?? 0}
            percent={progress?.percent ?? 0}
        />
    {:else}
        <div class="results">
            <FormCard
                title={format.format(target.kind === 'node' ? users.length : ips.length)}
                description={target.kind === 'node'
                    ? t('node-active-sessions.drawer.widget.active-users-on-this-node')
                    : t('active-sessions-drawer.widget.active-ips-across-nodes')}
                icon={target.kind === 'user' ? 'TbRadar' : 'TbUser'}
                headingSize="lg"
                tone="teal"
            >
                {#snippet actions()}
                    <ActionIcon
                        icon="TbQuestionMark"
                        label={t('active-sessions-drawer.widget.requirements')}
                        size={34}
                        iconSize={20}
                        tone="yellow"
                        onclick={() => (requirements = true)}
                    />
                    {#if target.kind === 'user'}
                        <ActionIcon
                            icon="PiLinkBreak"
                            tone="orange"
                            size={34}
                            iconSize={20}
                            label={dropLabel({ kind: 'all' })}
                            disabled={sending}
                            onclick={() => void drop({ kind: 'all' }, target.name)}
                        />
                    {/if}
                    <ActionIcon
                        icon="TbRefresh"
                        label={t('common.action.refresh')}
                        size={34}
                        iconSize={20}
                        tone="indigo"
                        disabled={loading || sending}
                        onclick={() => void refresh()}
                    />
                {/snippet}
            </FormCard>
            {#if error}
                <p role="alert">
                    {t('active-sessions-drawer.widget.job-failed-description')} {error}
                </p>
            {:else}
                {#if ips.length}
                    <dl>
                        <div>
                            <dt>{t('active-sessions-drawer.widget.distinct-ips')}</dt>
                            <dd>{format.format(distinctIps)}</dd>
                        </div>
                        <div>
                            <dt>Last seen in 5 minutes</dt>
                            <dd>{format.format(counts.recent)}</dd>
                        </div>
                        <div>
                            <dt>5–60 minutes ago</dt>
                            <dd>{format.format(counts.idle)}</dd>
                        </div>
                        <div>
                            <dt>Over an hour ago</dt>
                            <dd>{format.format(counts.stale)}</dd>
                        </div>
                    </dl>
                {/if}
                {#if target.kind === 'user'}
                    {#each nodes as node (node.nodeUuid)}
                        {@const selection = { kind: 'node' as const, nodeUuid: node.nodeUuid }}
                        <section>
                            <h3>
                                <a
                                    href={p('/dashboard/open/node/:id', {
                                        params: { id: node.nodeUuid }
                                    })}
                                    >{node.nodeName}</a
                                >
                                <small>{node.countryCode} · {format.format(node.ips.length)}</small>
                            </h3>
                            <button
                                type="button"
                                disabled={sending}
                                onclick={() => void drop(selection, node.nodeName)}
                            >
                                {dropLabel(selection)}
                            </button>
                            <SessionIps ips={node.ips} {copy} {now}>
                                {#snippet actions(session)}
                                    {@const ipSelection = {
                                        kind: 'ip' as const,
                                        nodeUuid: node.nodeUuid,
                                        ip: session.ip
                                    }}
                                    <button
                                        type="button"
                                        disabled={sending}
                                        onclick={() =>
                                            void drop(
                                                ipSelection,
                                                `${session.ip} · ${node.nodeName}`
                                            )}
                                    >
                                        {dropLabel(ipSelection)}
                                    </button>
                                {/snippet}
                            </SessionIps>
                        </section>
                    {:else}
                        <EmptyState
                            icon="TbRadar"
                            label={t('common.message.nothing-found')}
                            iconSize={48}
                        />
                    {/each}
                {:else if users.length}
                    <div class="session-sort">
                        {#each [
                            ['default', t('common.action.default')],
                            ['asc', 'IPs ↑'],
                            ['desc', 'IPs ↓']
                        ] as [value, label] (value)}
                            <Button
                                variant={sort === value ? 'light' : 'subtle'}
                                aria-pressed={sort === value}
                                onclick={() => {
                                    sort = value ?? 'default'
                                    page = 0
                                }}
                                >{label}</Button
                            >
                        {/each}
                    </div>
                    {#each sortedUsers.slice(
                        page * pageSize,
                        (page + 1) * pageSize
                    ) as user (user.userId)}
                        <section>
                            <h3>
                                <a
                                    href={p('/dashboard/open/user/:id', {
                                        params: { id: String(user.userId) }
                                    })}
                                    title={t('node-active-session.item.widget.view-user')}
                                    >#{user.userId}</a
                                >
                                <small>{format.format(user.ips.length)}</small>
                            </h3>
                            <SessionIps ips={user.ips} {copy} {now} />
                        </section>
                    {/each}
                    {#if sortedUsers.length > pageSize}
                        <div class="button-row">
                            <button type="button" disabled={page === 0} onclick={() => page--}>
                                Previous
                            </button>
                            <button
                                type="button"
                                disabled={(page + 1) * pageSize >= sortedUsers.length}
                                onclick={() => page++}
                            >
                                {t('common.action.next')}
                            </button>
                        </div>
                    {/if}
                {:else}
                    <EmptyState
                        icon="TbRadar"
                        label={t('common.message.nothing-found')}
                        iconSize={48}
                    />
                {/if}
            {/if}
        </div>
    {/if}
</Overlay>
{#if requirements}
    <Overlay
        title={t('active-sessions-drawer.widget.requirements')}
        icon="TbAlertTriangle"
        tone="yellow"
        width={480}
        close={() => (requirements = false)}
    >
        {#if target.kind === 'user'}
            <p>
                {t('active-sessions-drawer.widget.warning-version', { version: '2.7.0' }).replace(
                    /<\/?highlight>/g,
                    ''
                )}
            </p>
        {:else}
            <p>
                {t('active-sessions-drawer.widget.warning-n-plus-one').replace(
                    /<\/?highlight>/g,
                    ''
                )}
            </p>
        {/if}
        <p>{t('active-sessions-drawer.widget.warning-activity').replace(/<\/?highlight>/g, '')}</p>
    </Overlay>
{/if}

<style>
    .session-sort {
        display: flex;
        gap: var(--mantine-spacing-xs);
        border-block-end: 1px solid var(--gw-border);
    }

    dl {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 0.5rem 1rem;
    }
    h3 {
        margin-block: 0;
    }
    h3 {
        font-size: 1rem;
    }
    section {
        margin-block: 0.75rem;
    }
    section {
        display: grid;
        justify-items: start;
        gap: 0.4rem;
        border-top: 1px solid var(--gw-border);
        padding-block-start: 0.75rem;
    }
    dd {
        margin: 0;
        font-weight: 600;
    }
    small {
        font-weight: 400;
    }
    .results {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
</style>
