<script lang="ts">
    import { formatRelativeDateUtil, formatTimeUtil } from '@shared/utils/time-utils'
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import CopyableField from '../ui/CopyableField.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Tooltip from '../ui/Tooltip.svelte'
    import { type SessionIp, sessionActivity } from './scan'

    let {
        actions,
        compact = false,
        ips,
        now,
        search = '',
        copy
    }: {
        actions?: Snippet<[SessionIp]> | undefined
        compact?: boolean | undefined
        ips: SessionIp[]
        now?: number | undefined
        search?: string | undefined
        copy?: ((ip: string) => void) | undefined
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

    function activityMeta(activity: ReturnType<typeof sessionActivity> | undefined): {
        icon: IconName
        color: string
    } {
        switch (activity) {
            case 'recent':
                return { icon: 'TbClockCheck', color: 'var(--mantine-color-teal-6)' }
            case 'idle':
                return { icon: 'TbClockPause', color: 'var(--mantine-color-yellow-6)' }
            case 'stale':
                return { icon: 'TbClockExclamation', color: 'var(--mantine-color-red-6)' }
            default:
                return { icon: 'TbClock', color: 'var(--mantine-color-dimmed)' }
        }
    }
</script>

<ul class:compact>
    {#each visible.slice(0, limit) as session}
        {@const lastSeen = new Date(session.lastSeen)}
        {@const activity = now === undefined ? undefined : sessionActivity(lastSeen, now)}
        {@const meta = activityMeta(activity)}
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
                <div class="ip-copy-wrap">
                    <CopyableField
                        variant="unstyled"
                        value={session.ip}
                        ariaLabel={session.ip}
                        oncopy={(ok) => {
                            if (ok) copy?.(session.ip)
                        }}
                    >
                        {#snippet left()}
                            <Tooltip multiline placement="top">
                                {#snippet tip()}
                                    <div class="seen-tip">
                                        <strong
                                            >{formatRelativeDateUtil(
                                                lastSeen,
                                                t,
                                                locale.language
                                            )}</strong
                                        >
                                        <span class="seen-tip-date"
                                            >{formatTimeUtil({
                                                time: lastSeen,
                                                template: 'TIME_FIRST_DATETIME',
                                                language: locale.language
                                            })}</span
                                        >
                                    </div>
                                {/snippet}
                                <span
                                    class="seen-clock"
                                    style:color={meta.color}
                                    role="img"
                                    aria-label={date.format(lastSeen)}
                                >
                                    <Icon name={meta.icon} size={16} />
                                </span>
                            </Tooltip>
                        {/snippet}
                    </CopyableField>
                </div>
            {:else}
                <button
                    type="button"
                    title={t('common.action.copy')}
                    onclick={() => copy?.(session.ip)}
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
    <Button onclick={() => (limit += 20)}>Show more</Button>
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
        padding: 4px 0;
        border: 0;
    }
    .ip-copy-wrap {
        flex: 1;
        min-width: 0;
    }
    .seen-clock {
        display: inline-flex;
        align-items: center;
        cursor: help;
    }
    .seen-tip {
        display: grid;
        gap: 2px;
        padding: 2px;
        font-size: var(--mantine-font-size-xs);
    }
    .seen-tip strong {
        color: var(--mantine-color-white);
        font-weight: 600;
    }
    .seen-tip-date {
        color: var(--mantine-color-dimmed);
        font-family: var(--mantine-font-family-monospace);
    }
    .matching {
        outline: 1px solid var(--gw-accent);
    }
    .compact .matching {
        outline-offset: -1px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-cyan-light);
    }
</style>
