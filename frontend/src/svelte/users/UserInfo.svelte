<script lang="ts">
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { formatTimeUtil } from '@shared/utils/time-utils'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import type { User } from './queries'
    import UserStatusBadge from './UserStatusBadge.svelte'

    let { user, copy }: { user: User; copy: (value: string) => Promise<boolean> } = $props()
    let t = $derived(locale.t)
    let copied = $state('')
    let copyTimeout: ReturnType<typeof setTimeout> | undefined

    onDestroy(() => clearTimeout(copyTimeout))

    function date(time: Date | null): string | null {
        return time
            ? formatTimeUtil({ time, template: 'TIME_FIRST_DATETIME', language: locale.language })
            : null
    }

    async function copyRow(key: string, value: string): Promise<void> {
        if (!(await copy(value))) return
        clearTimeout(copyTimeout)
        copied = key
        copyTimeout = setTimeout(() => (copied = ''), 2000)
    }

    type Row = [string, string | number | null | undefined, monospace?: boolean]
    let sections = $derived<
        {
            title: string
            icon: IconName
            tone: string
            status?: boolean
            rows: Row[]
        }[]
    >([
        {
            title: t('detailed-user-info-drawer.widget.user-information'),
            icon: 'PiUserDuotone',
            tone: 'blue',
            status: true,
            rows: [
                ['ID', user.id, true],
                [t('detailed-user-info-drawer.widget.short-uuid'), user.shortUuid, true],
                [t('common.field.username'), user.username],
                [t('detailed-user-info-drawer.widget.email'), user.email],
                [t('detailed-user-info-drawer.widget.telegram-id'), user.telegramId],
                [t('common.field.description'), user.description],
                [t('common.field.tag'), user.tag]
            ]
        },
        {
            title: t('detailed-user-info-drawer.widget.traffic-information'),
            icon: 'PiArrowsDownUpDuotone',
            tone: 'teal',
            rows: [
                [
                    t('detailed-user-info-drawer.widget.used-traffic'),
                    prettifyBytesUtil(user.userTraffic.usedTrafficBytes)
                ],
                [
                    t('detailed-user-info-drawer.widget.lifetime-used-traffic'),
                    prettifyBytesUtil(user.userTraffic.lifetimeUsedTrafficBytes)
                ],
                [
                    t('detailed-user-info-drawer.widget.traffic-limit'),
                    prettifyBytesUtil(user.trafficLimitBytes)
                ],
                [
                    t('detailed-user-info-drawer.widget.traffic-limit-strategy'),
                    user.trafficLimitStrategy
                ],
                [
                    t('detailed-user-info-drawer.widget.last-traffic-reset'),
                    date(user.lastTrafficResetAt)
                ]
            ]
        },
        {
            title: t('detailed-user-info-drawer.widget.subscription-information'),
            icon: 'PiCalendarDotDuotone',
            tone: 'orange',
            rows: [
                [t('common.field.subscription-url'), user.subscriptionUrl, true],
                [t('detailed-user-info-drawer.widget.expires-at'), date(user.expireAt)],
                [t('detailed-user-info-drawer.widget.revoked-at'), date(user.subRevokedAt)]
            ]
        },
        {
            title: t('detailed-user-info-drawer.widget.connection-information'),
            icon: 'PiNetworkDuotone',
            tone: 'violet',
            rows: [
                [t('detailed-user-info-drawer.widget.trojan-password'), user.trojanPassword, true],
                ['Vless/Hysteria2 UUID', user.vlessUuid, true],
                [t('detailed-user-info-drawer.widget.ss-password'), user.ssPassword, true],
                [t('common.field.first-connected-at'), date(user.userTraffic.firstConnectedAt)],
                [
                    t('detailed-user-info-drawer.widget.last-online'),
                    date(user.userTraffic.onlineAt)
                ],
                [
                    t('detailed-user-info-drawer.widget.last-connected-node'),
                    user.userTraffic.lastConnectedNodeUuid,
                    true
                ]
            ]
        },
        {
            title: t('detailed-user-info-drawer.widget.active-internal-squads'),
            icon: 'PiTagDuotone',
            tone: 'green',
            rows: user.activeInternalSquads.map(({ name, uuid }): Row => [name, uuid, true])
        },
        {
            title: t('detailed-user-info-drawer.widget.timestamps'),
            icon: 'PiClockDuotone',
            tone: 'gray',
            rows: [
                [t('common.field.created-at'), date(user.createdAt)],
                [t('common.field.updated-at'), date(user.updatedAt)]
            ]
        }
    ])
</script>

<div class="info">
    {#each sections as section (section.title)}
        {#if section.rows.length}
            <FormCard title={section.title} icon={section.icon} tone={section.tone}>
                {#snippet actions()}
                    {#if section.status}
                        <UserStatusBadge status={user.status} />
                    {/if}
                {/snippet}
                <dl>
                    {#each section.rows as [label, value, monospace], index}
                        {@const key = `${section.title}:${index}`}
                        <div>
                            <dt>{label}</dt>
                            <dd>
                                {#if value !== null && value !== undefined && value !== ''}
                                    <button
                                        type="button"
                                        class="mantine-UnstyledButton-root"
                                        class:monospace
                                        class:copied={copied === key}
                                        title={t(
                                            copied === key
                                                ? 'common.message.copied'
                                                : 'common.action.copy'
                                        )}
                                        onclick={() => void copyRow(key, String(value))}
                                    >
                                        <span>{value}</span>
                                        <Icon
                                            name={copied === key ? 'PiCheck' : 'PiCopy'}
                                            size={14}
                                        />
                                    </button>
                                {:else}
                                    <span class="empty">—</span>
                                {/if}
                            </dd>
                        </div>
                    {/each}
                </dl>
            </FormCard>
        {/if}
    {/each}
</div>

<style>
    .info {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .info :global(h3) {
        font-size: var(--mantine-font-size-lg);
    }
    dl {
        min-width: 0;
        margin: 0;
    }
    dl > div + div {
        margin-block-start: var(--mantine-spacing-sm);
        padding-block-start: var(--mantine-spacing-sm);
        border-block-start: 1px solid var(--mantine-color-default-border);
    }
    dt {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    dd {
        margin: 0;
        margin-block-start: 2px;
    }
    .empty {
        color: var(--mantine-color-dimmed);
    }
    button {
        display: inline-flex;
        align-items: center;
        max-width: 100%;
        gap: 6px;
        margin-inline: -6px;
        padding-inline: 6px;
        border-radius: var(--mantine-radius-sm);
        color: inherit;
    }
    button > span {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    button.monospace > span {
        font-family: var(--mantine-font-family-monospace);
    }
    button :global(svg) {
        flex: none;
        opacity: 0;
    }
    button:hover,
    button:focus-visible {
        background: var(--mantine-color-default-hover);
    }
    button.copied {
        color: var(--mantine-color-teal-6);
    }
    button:is(:hover, :focus-visible, .copied) :global(svg) {
        opacity: 0.7;
    }
</style>
