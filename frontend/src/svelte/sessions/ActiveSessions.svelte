<script lang="ts">
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import {
        dropUserSessions,
        loadNodeSessions,
        loadUserSessions,
        type SessionDrop,
        type UserSessionNode,
        type UserSessionProgress
    } from './queries'
    import SessionIps from './SessionIps.svelte'
    import { sessionActivityCounts } from './scan'

    type Target =
        | { kind: 'node'; uuid: string; name: string }
        | { kind: 'user'; id: number; name: string }
    type NodeUser = Awaited<ReturnType<typeof loadNodeSessions>>[number]

    let { target, close }: { target: Target; close: () => void } = $props()
    let dialog: HTMLDialogElement
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
        dialog.showModal()
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
            if (controller === request) {
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

<dialog bind:this={dialog} onclose={close} aria-labelledby="active-sessions-title">
    <div class="header">
        <h2 id="active-sessions-title">{t('common.field.active-sessions')} · {target.name}</h2>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
    <details>
        <summary>{t('active-sessions-drawer.widget.requirements')}</summary>
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
    </details>
    {#if actionError}
        <p role="alert">{actionError}</p>
    {/if}
    {#if notice}
        <p role="status">{notice}</p>
    {/if}
    {#if loading}
        <p role="status">{t('active-sessions-drawer.widget.fetching')}</p>
        {#if progress && progress.total > 0}
            <progress
                max={progress.total}
                value={progress.completed}
                aria-label={t('active-sessions-drawer.widget.fetching')}
            ></progress>
            <p role="status">{t('active-sessions-drawer.widget.progress', progress)}</p>
        {/if}
    {:else}
        <div class="button-row">
            <button type="button" disabled={sending} onclick={() => void refresh()}>
                {t('common.action.refresh')}
            </button>
            {#if target.kind === 'user'}
                <button
                    type="button"
                    disabled={sending}
                    onclick={() => void drop({ kind: 'all' }, target.name)}
                >
                    {dropLabel({ kind: 'all' })}
                </button>
            {/if}
        </div>
        {#if error}
            <p role="alert">{t('active-sessions-drawer.widget.job-failed-description')} {error}</p>
        {:else}
            <dl>
                {#if target.kind === 'node'}
                    <div>
                        <dt>{t('node-active-sessions.drawer.widget.active-users-on-this-node')}</dt>
                        <dd>{format.format(users.length)}</dd>
                    </div>
                {/if}
                <div>
                    <dt>{t('active-sessions-drawer.widget.active-ips-across-nodes')}</dt>
                    <dd>{format.format(ips.length)}</dd>
                </div>
                <div>
                    <dt>{t('active-sessions-drawer.widget.distinct-ips')}</dt>
                    <dd>{format.format(distinctIps)}</dd>
                </div>
                {#if ips.length}
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
                {/if}
            </dl>
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
                                        void drop(ipSelection, `${session.ip} · ${node.nodeName}`)}
                                >
                                    {dropLabel(ipSelection)}
                                </button>
                            {/snippet}
                        </SessionIps>
                    </section>
                {:else}
                    <p>{t('common.message.nothing-found')}</p>
                {/each}
            {:else if users.length}
                <label>
                    Sort users by
                    <select bind:value={sort} onchange={() => (page = 0)}>
                        <option value="default">
                            {t('common.action.default', { defaultValue: 'Default' })}
                        </option>
                        <option value="asc">IPs ↑</option>
                        <option value="desc">IPs ↓</option>
                    </select>
                </label>
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
                <p>{t('common.message.nothing-found')}</p>
            {/if}
        {/if}
    {/if}
</dialog>

<style>
    dialog {
        width: min(48rem, calc(100vw - 1.5rem));
    }
    .header,
    dl {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 0.5rem 1rem;
    }
    .header {
        justify-content: space-between;
    }
    h2,
    h3 {
        margin-block: 0;
    }
    h3 {
        font-size: 1rem;
    }
    details,
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
    progress {
        max-width: 100%;
    }
</style>
