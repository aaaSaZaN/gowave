<script module lang="ts">
    export interface ConfigApplyCheck {
        savedAt: number
        previousStatuses: ReadonlyMap<string, number | undefined>
        beforeRefetch: number
    }
</script>

<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import { nodesQuery } from '../nodes/queries'
    import Icon from '../ui/Icon.svelte'

    let {
        profileUuid,
        check
    }: {
        profileUuid: string
        check: ConfigApplyCheck | undefined
    } = $props()

    let expired = $state(false)
    let expanded = $state<'checking' | 'applied' | 'failed'>()
    const detailsId = $props.id()
    const nodes = createQuery(() => ({
        ...nodesQuery,
        refetchInterval: check && !expired ? 2_000 : false
    }))
    const t = $derived(locale.t)

    $effect(() => {
        if (!check) return
        // The worker has to pick up the job before Xray can validate and restart.
        const timer = setTimeout(() => (expired = true), 60_000)
        return () => clearTimeout(timer)
    })

    const status = $derived.by(() => {
        const checking: string[] = []
        const applied: string[] = []
        const failed = new Map<string, string[]>()
        let failedCount = 0
        const fresh = check && nodes.dataUpdatedAt > check.beforeRefetch

        for (const node of nodes.data ?? []) {
            if (node.isDisabled || node.configProfile.activeConfigProfileUuid !== profileUuid)
                continue

            const error = node.lastStatusMessage === 'OK' ? null : node.lastStatusMessage
            if (check) {
                const changedAt = node.lastStatusChange?.getTime()
                const completed =
                    fresh &&
                    !node.isConnecting &&
                    changedAt !== undefined &&
                    changedAt !== check.previousStatuses.get(node.uuid) &&
                    changedAt >= check.savedAt

                if (!completed || (!error && !node.isConnected)) {
                    checking.push(node.name)
                    continue
                }
                if (!error) applied.push(node.name)
            }
            if (error && (check || node.isConnected)) {
                failedCount++
                const names = failed.get(error)
                if (names) names.push(node.name)
                else failed.set(error, [node.name])
            }
        }
        return { checking, applied, failed, failedCount }
    })
    const groups = $derived([
        {
            id: 'checking',
            count: status.checking.length,
            label: expired
                ? t('node-error-message.config-unconfirmed')
                : t('node-error-message.config-checking'),
            icon: 'TbAlertCircle'
        },
        {
            id: 'applied',
            count: status.applied.length,
            label: t('node-error-message.config-applied'),
            icon: 'TbCircleCheck'
        },
        {
            id: 'failed',
            count: status.failedCount,
            label: t('node-error-message.config-failed'),
            icon: 'TbAlertCircle'
        }
    ] as const)
    const active = $derived(groups.find((group) => group.count && group.id === expanded))
</script>

{#snippet nodeList(names: string[])}
    <ul class="node-list">
        {#each names as name}
            <li>{name}</li>
        {/each}
    </ul>
{/snippet}

{#if status.checking.length || status.applied.length || status.failed.size}
    <div class="application-status">
        <div class="status-summary" role="status">
            {#each groups as group (group.id)}
                {#if group.count}
                    <button
                        type="button"
                        class:success={group.id === 'applied'}
                        aria-expanded={active?.id === group.id}
                        aria-controls={detailsId}
                        onclick={() => (expanded = active?.id === group.id ? undefined : group.id)}
                    >
                        {#if group.id === 'checking' && !expired}
                            <span class="loading-spinner" aria-hidden="true"></span>
                        {:else}
                            <Icon name={group.icon} size={16} />
                        {/if}
                        {group.label} <strong>{group.count.toLocaleString(locale.language)}</strong>
                        <Icon name="TbChevronDown" size={14} />
                    </button>
                {/if}
            {/each}
        </div>
        <div id={detailsId} class="status-details" hidden={!active}>
            {#if active?.id === 'failed'}
                <div class="failure-groups">
                    {#each status.failed as [error, names] (error)}
                        <div class="node-error">
                            <code>{error}</code>
                            {@render nodeList(names)}
                        </div>
                    {/each}
                </div>
            {:else if active}
                {@render nodeList(status[active.id])}
            {/if}
        </div>
    </div>
{/if}

<style>
    .application-status {
        flex: none;
        display: grid;
        grid-template-rows: auto minmax(0, 1fr);
        max-height: min(40%, 16rem);
        border-top: 1px solid var(--gw-border);
        background: var(--gw-raised-bg);
        font-size: 0.9rem;
    }
    .status-summary {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
        padding: 6px 12px;
    }
    button {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 4px 8px;
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-sm);
        background: transparent;
        color: light-dark(var(--mantine-color-orange-9), var(--mantine-color-orange-4));
        cursor: pointer;
    }
    button:hover,
    button[aria-expanded="true"] {
        border-color: var(--gw-border);
        background: var(--mantine-color-default-hover);
    }
    button[aria-expanded="true"] :global(svg:last-child) {
        transform: rotate(180deg);
    }
    .success {
        color: light-dark(var(--mantine-color-teal-9), var(--mantine-color-teal-4));
    }
    .loading-spinner {
        --spinner-size: 16px;
        flex: none;
    }
    .status-details {
        min-height: 0;
        overflow: auto;
        overscroll-behavior: contain;
        padding: 8px 16px 12px;
        border-top: 1px solid var(--gw-border);
    }
    .failure-groups {
        display: grid;
        gap: var(--mantine-spacing-sm);
    }
    .node-error {
        display: grid;
        gap: 4px;
        overflow-wrap: anywhere;
    }
    code {
        white-space: pre-wrap;
    }
    .node-list {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr));
        gap: 4px 16px;
        margin: 0;
        padding-inline-start: 1rem;
        overflow-wrap: anywhere;
    }
</style>
