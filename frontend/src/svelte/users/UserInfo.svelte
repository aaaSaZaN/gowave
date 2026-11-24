<script lang="ts">
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { locale } from '../i18n.svelte'
    import type { User } from './queries'

    let { user, copy }: { user: User; copy: (value: string) => void } = $props()
    let open = $state(false)
    let t = $derived(locale.t)
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )

    function date(value: Date | null): string | null {
        return value ? dates.format(value) : null
    }

    let sections = $derived([
        {
            title: t('detailed-user-info-drawer.widget.user-information'),
            rows: [
                [t('detailed-user-info-drawer.widget.short-uuid'), user.shortUuid],
                [t('detailed-user-info-drawer.widget.email'), user.email],
                [t('detailed-user-info-drawer.widget.telegram-id'), user.telegramId]
            ]
        },
        {
            title: t('detailed-user-info-drawer.widget.traffic-information'),
            rows: [
                [
                    t('detailed-user-info-drawer.widget.lifetime-used-traffic'),
                    prettifyBytesUtil(user.userTraffic.lifetimeUsedTrafficBytes)
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
            rows: [[t('detailed-user-info-drawer.widget.revoked-at'), date(user.subRevokedAt)]]
        },
        {
            title: t('detailed-user-info-drawer.widget.connection-information'),
            rows: [
                [t('detailed-user-info-drawer.widget.trojan-password'), user.trojanPassword],
                ['VLESS/Hysteria2 UUID', user.vlessUuid],
                [t('detailed-user-info-drawer.widget.ss-password'), user.ssPassword],
                [t('common.field.first-connected-at'), date(user.userTraffic.firstConnectedAt)],
                [
                    t('detailed-user-info-drawer.widget.last-connected-node'),
                    user.userTraffic.lastConnectedNodeUuid
                ]
            ]
        },
        {
            title: t('detailed-user-info-drawer.widget.active-internal-squads'),
            rows: user.activeInternalSquads.map(({ name, uuid }) => [name, uuid])
        },
        {
            title: t('detailed-user-info-drawer.widget.timestamps'),
            rows: [
                [t('common.field.created-at'), date(user.createdAt)],
                [t('common.field.updated-at'), date(user.updatedAt)]
            ]
        }
    ])
</script>

<details bind:open>
    <summary>{t('detailed-user-info-drawer.widget.detailed-user-info')}</summary>
    {#if open}
        <div class="grid">
            {#each sections as section (section.title)}
                {#if section.rows.length}
                    <section>
                        <h2>{section.title}</h2>
                        <dl>
                            {#each section.rows as [label, value]}
                                <div>
                                    <dt>{label}</dt>
                                    <dd>
                                        {#if value !== null && value !== undefined && value !== ''}
                                            <button
                                                type="button"
                                                title={t('common.action.copy')}
                                                onclick={() => copy(String(value))}
                                            >
                                                {value}
                                            </button>
                                        {:else}
                                            —
                                        {/if}
                                    </dd>
                                </div>
                            {/each}
                        </dl>
                    </section>
                {/if}
            {/each}
        </div>
    {/if}
</details>

<style>
    details {
        margin-block: 0.75rem;
        padding: 0.55rem 0.7rem;
        border: 1px solid var(--gw-border);
        border-radius: 0.375rem;
    }
    summary {
        cursor: pointer;
    }
    .grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(min(100%, 19rem), 1fr));
        gap: 0.75rem;
        margin-block-start: 0.75rem;
    }
    section {
        min-width: 0;
    }
    h2 {
        font-size: 1rem;
        margin: 0;
    }
    dl div {
        margin-block: 0.25rem;
    }
    dd {
        margin: 0;
    }
    button {
        max-width: 100%;
        overflow-wrap: anywhere;
        text-align: start;
    }
</style>
