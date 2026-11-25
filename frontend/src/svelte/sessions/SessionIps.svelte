<script lang="ts">
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import { type SessionIp, sessionActivity } from './scan'

    let {
        actions,
        compact = false,
        ips,
        now,
        search = '',
        copy
    }: {
        actions?: Snippet<[SessionIp]>
        compact?: boolean
        ips: SessionIp[]
        now?: number
        search?: string
        copy: (ip: string) => void
    } = $props()
    let t = $derived(locale.t)
    let limit = $state(20)
    let visible = $derived(
        search
            ? [
                  ...ips.filter(({ ip }) => ip.includes(search)),
                  ...ips.filter(({ ip }) => !ip.includes(search))
              ]
            : ips
    )
    let date = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'medium' })
    )
</script>

<ul class:compact>
    {#each visible.slice(0, limit) as session}
        {@const lastSeen = new Date(session.lastSeen)}
        {@const activity = now === undefined ? undefined : sessionActivity(lastSeen, now)}
        <li class:matching={search && session.ip.includes(search)}>
            {#if compact}
                <ActionIcon
                    icon="TbExternalLink"
                    label={`IP Info: ${session.ip}`}
                    href={`https://ipinfo.io/${encodeURIComponent(session.ip)}`}
                    target="_blank"
                    rel="noreferrer"
                    tone="cyan"
                    size={36}
                    iconSize={18}
                />
                <div class="ip-field">
                    <time
                        datetime={lastSeen.toISOString()}
                        title={date.format(lastSeen)}
                        class:recent={activity === 'recent'}
                        class:idle={activity === 'idle'}
                        class:stale={activity === 'stale'}
                    >
                        <span aria-hidden="true"><Icon name="TbClock" size={16} /></span>
                        <span class="sr-only">{date.format(lastSeen)}</span>
                    </time>
                    <button
                        type="button"
                        aria-label={`${t('common.action.copy')} ${session.ip}`}
                        title={t('common.action.copy')}
                        onclick={() => copy(session.ip)}
                    >
                        {session.ip}
                        <Icon name="TbCopy" size={16} />
                    </button>
                </div>
            {:else}
                <button
                    type="button"
                    title={t('common.action.copy')}
                    onclick={() => copy(session.ip)}
                >
                    {session.ip}
                </button>
                <time
                    datetime={lastSeen.toISOString()}
                    title="Last seen"
                    class:recent={activity === 'recent'}
                    class:idle={activity === 'idle'}
                    class:stale={activity === 'stale'}
                >
                    <span aria-hidden="true">◷</span> {date.format(lastSeen)}
                </time>
                <a
                    href={`https://ipinfo.io/${encodeURIComponent(session.ip)}`}
                    target="_blank"
                    rel="noreferrer"
                    >IP info</a
                >
            {/if}
            {@render actions?.(session)}
        </li>
    {/each}
</ul>
{#if visible.length > limit}
    <button type="button" onclick={() => (limit += 20)}>Show more</button>
{/if}

<style>
    ul {
        list-style: none;
        margin: 0 0 0.5rem;
        padding: 0;
    }
    li {
        display: flex;
        flex-wrap: wrap;
        align-items: baseline;
        gap: 0.5rem;
        padding: 0.25rem;
        border-block-start: 1px solid var(--gw-border);
        font-size: 0.9rem;
    }
    time {
        margin-inline-end: auto;
    }
    time.recent span {
        color: #20a880;
    }
    time.idle span {
        color: #b78221;
    }
    time.stale span {
        color: #df6473;
    }
    li button {
        min-height: 1.75rem;
        padding-block: 0.2rem;
        overflow-wrap: anywhere;
    }
    .compact {
        margin: 0;
    }
    .compact li {
        align-items: center;
        flex-wrap: nowrap;
        gap: var(--mantine-spacing-xs);
        padding: 0.25rem 0;
        border-block-start-color: rgba(var(--gw-fg-rgb), 0.08);
    }
    .ip-field {
        display: flex;
        align-items: center;
        flex: 1;
        min-width: 0;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
    }
    .ip-field time {
        display: flex;
        flex: none;
        margin: 0;
        padding-inline-start: 0.6rem;
        cursor: help;
    }
    .ip-field time span[aria-hidden] {
        display: flex;
    }
    .ip-field button {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.45rem;
        flex: 1;
        min-width: 0;
        min-height: 34px;
        padding: 0.25rem 0.6rem;
        border: 0;
        color: var(--mantine-color-text);
        background: transparent;
        font-family: var(--mantine-font-family-monospace);
        text-align: start;
        cursor: copy;
    }
    .ip-field button :global(svg) {
        flex: none;
    }
    .ip-field button:hover {
        color: var(--mantine-color-bright);
    }
    .matching {
        outline: 1px solid var(--gw-accent);
    }
    .compact .matching {
        outline-offset: -1px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-cyan-light);
    }
    .sr-only {
        position: absolute;
        width: 1px;
        height: 1px;
        overflow: hidden;
        clip-path: inset(50%);
        white-space: nowrap;
    }
</style>
