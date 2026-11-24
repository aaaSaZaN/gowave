<script lang="ts">
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { type SessionIp, sessionActivity } from './scan'

    let {
        actions,
        ips,
        now,
        search = '',
        copy
    }: {
        actions?: Snippet<[SessionIp]>
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

<ul>
    {#each visible.slice(0, limit) as session}
        {@const lastSeen = new Date(session.lastSeen)}
        {@const activity = now === undefined ? undefined : sessionActivity(lastSeen, now)}
        <li class:matching={search && session.ip.includes(search)}>
            <button type="button" title={t('common.action.copy')} onclick={() => copy(session.ip)}>
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
    .matching {
        outline: 1px solid var(--gw-accent);
    }
</style>
