<script lang="ts">
    import { buildOpenEntityUrl, OPEN_ENTITY } from '@shared/constants/routes'
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { getConnectionStatusColorUtil, getTimeAgoUtil } from '@shared/utils/time-utils'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import dayjs from 'dayjs'
    import { locale } from '../i18n.svelte'
    import Metadata from '../Metadata.svelte'
    import { flag } from '../nodes/analytics'
    import NodeDetails from '../nodes/NodeDetails.svelte'
    import { nodesQuery } from '../nodes/queries'
    import UsageReport from '../nodes/UsageReport.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActiveSessions from '../sessions/ActiveSessions.svelte'
    import UserTorrentReports from '../torrent-reports/UserTorrentReports.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import ActionMenu from '../ui/ActionMenu.svelte'
    import Button from '../ui/Button.svelte'
    import CopyableField from '../ui/CopyableField.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import { changeUserStatus, deleteUser, resetUserTraffic, revokeSubscription } from './actions'
    import ConnectionKeys from './ConnectionKeys.svelte'
    import { type User, userMetadataQuery, userQuery, usersQueryKey } from './queries'
    import SubscriptionQr from './SubscriptionQr.svelte'
    import { userSelection } from './selection.svelte'
    import UserAccess from './UserAccess.svelte'
    import UserDevices from './UserDevices.svelte'
    import UserForm from './UserForm.svelte'
    import UserInfo from './UserInfo.svelte'
    import UserStatusBadge from './UserStatusBadge.svelte'

    const confirmAction = useConfirmation()

    let { id, close }: { id: number; close: () => void } = $props()
    const record = createQuery(() => userQuery(String(id)))
    const metadata = createQuery(() => userMetadataQuery(id))
    const nodes = createQuery(() => nodesQuery)
    let user = $derived(record.data)
    let showLoading = $state(false)
    $effect(() => {
        if (!record.isPending) return
        const timer = setTimeout(() => (showLoading = true), 650)
        return () => {
            clearTimeout(timer)
            showLoading = false
        }
    })
    let lastNode = $derived(
        user?.userTraffic.lastConnectedNodeUuid
            ? nodes.data?.find((item) => item.uuid === user.userTraffic.lastConnectedNodeUuid)
            : undefined
    )
    let metadataUser = $state<number>()
    let showTorrentReports = $state(false)
    let sessionUser = $state<number>()
    let usageUser = $state<number>()
    let nodeId = $state('')
    let delivery = $state<{ userId: number; kind: 'qr' | 'keys' | 'devices' }>()
    let detail = $state<'info' | 'access' | 'history'>()
    let t = $derived(locale.t)
    const shades = { ACTIVE: 'teal', DISABLED: 'gray', EXPIRED: 'red', LIMITED: 'yellow' } as const
    let expiryDays = $derived(user ? dayjs(user.expireAt).diff(dayjs(), 'day') : 0)
    let trafficPercent = $derived(
        user?.trafficLimitBytes
            ? Math.floor((user.userTraffic.usedTrafficBytes * 100) / user.trafficLimitBytes)
            : 0
    )
    let trafficTone = $derived(
        trafficPercent > 95 ? 'red' : trafficPercent > 80 ? 'yellow' : 'teal'
    )
    let subscriberHelp = $state(false)
    type Action = 'enable' | 'disable' | 'reset' | 'revoke' | 'passwords' | 'delete'

    const mutation = createMutation(() => ({
        mutationFn: ({ id, action }: { id: number; action: Action }): Promise<User | undefined> => {
            switch (action) {
                case 'enable':
                    return changeUserStatus(id, true)
                case 'disable':
                    return changeUserStatus(id, false)
                case 'reset':
                    return resetUserTraffic(id)
                case 'revoke':
                    return revokeSubscription(id, false)
                case 'passwords':
                    return revokeSubscription(id, true)
                case 'delete':
                    return deleteUser(id)
            }
        },
        onSuccess: async (updated, { id, action }) => {
            if (action === 'delete') {
                userSelection.delete(id)
                close()
                queryClient.removeQueries({ queryKey: userQuery(String(id)).queryKey })
            } else if (updated) {
                queryClient.setQueryData(userQuery(String(id)).queryKey, updated)
                notifications.show({
                    title: t('common.message.success'),
                    message: t('common.message.operation-completed'),
                    color: 'teal'
                })
            }
            await queryClient.invalidateQueries({ queryKey: usersQueryKey })
        },
        onError: report
    }))

    function report(reason: unknown): void {
        notifications.show({
            title: t('common.message.error'),
            message: reason instanceof Error ? reason.message : t('common.message.error'),
            color: 'red'
        })
    }

    async function run(action: Action, label: string): Promise<void> {
        if (!user || mutation.isPending || !(await confirmAction(`${label} · ${user.username}?`)))
            return
        mutation.mutate({ id: user.id, action })
    }

    async function copy(text: string): Promise<boolean> {
        try {
            await navigator.clipboard.writeText(text)
            notifications.show({
                title: t('common.message.copied'),
                message: text,
                color: 'teal'
            })
            return true
        } catch (cause) {
            report(cause)
            return false
        }
    }
</script>

{#if user}
    <UserForm {user} onclose={close}>
        {#snippet headerActions()}
            <ActionIcon
                icon="TbLink"
                label={t('common.action.copy-link')}
                subtle
                size={28}
                iconSize={16}
                onclick={() => copy(buildOpenEntityUrl(OPEN_ENTITY.USER, user.id))}
            />
        {/snippet}
        {#snippet identity()}
            <section
                class="identity identity-card"
                aria-label={t('user-identity-creation-card.user-identity')}
            >
                <div class="identity-heading identity-section">
                    <span
                        class="user-icon"
                        style:--identity-tone={`var(--mantine-color-${shades[user.status]}-6)`}
                        style:color={`var(--mantine-color-${shades[user.status]}-4)`}
                        ><Icon name="TbUser" size={20} /></span
                    >
                    <div class="user-title">
                        <button
                            class="mantine-UnstyledButton-root"
                            type="button"
                            onclick={() => copy(String(user.id))}
                        >
                            {user.id}
                        </button>
                        <button
                            class="mantine-UnstyledButton-root"
                            type="button"
                            onclick={() => copy(user.username)}
                        >
                            {user.username}
                        </button>
                    </div>
                    <UserStatusBadge status={user.status} />
                </div>
                <div class="identity-actions identity-section">
                    <div class="identity-group">
                        <ActionIcon
                            icon="TbQrcode"
                            label={t('view-user-modal.widget.qr-code')}
                            tone="teal"
                            size={34}
                            iconSize={22}
                            disabled={!user.subscriptionUrl}
                            onclick={() => (delivery = { userId: user.id, kind: 'qr' })}
                        />
                        <ActionIcon
                            icon="PiLinkBreak"
                            label={t('get-user-subscription-links.feature.connection-keys')}
                            tone="teal"
                            size={34}
                            iconSize={22}
                            onclick={() => (delivery = { userId: user.id, kind: 'keys' })}
                        />
                        <ActionIcon
                            icon="TbJson"
                            label="Metadata"
                            tone="teal"
                            size={34}
                            iconSize={22}
                            disabled={!metadata.data || metadata.isLoading}
                            onclick={() => (metadataUser = user.id)}
                        />
                    </div>
                    <span class="identity-divider" aria-hidden="true"></span>
                    <div class="identity-group">
                        <ActionIcon
                            icon="PiUserCircle"
                            label={t('view-user-modal.widget.detailed-info')}
                            tone="cyan"
                            size={34}
                            iconSize={22}
                            onclick={() => (detail = 'info')}
                        />
                        <ActionIcon
                            icon="TbServerCog"
                            label={t('view-user-modal.widget.accessible-nodes')}
                            tone="cyan"
                            size={34}
                            iconSize={22}
                            onclick={() => (detail = 'access')}
                        />
                    </div>
                    <span class="identity-divider" aria-hidden="true"></span>
                    <div class="identity-group">
                        <ActionIcon
                            icon="TbChartArcs"
                            label={t('common.field.usage-stats')}
                            tone="indigo"
                            size={34}
                            iconSize={24}
                            onclick={() => (usageUser = user.id)}
                        />
                        <ActionIcon
                            icon="TbFlame"
                            label={t('get-user-torrent-blocker-reports.feature.blocker-reports')}
                            tone="indigo"
                            size={34}
                            iconSize={22}
                            onclick={() => (showTorrentReports = true)}
                        />
                        <ActionIcon
                            icon="TbTimeline"
                            label={t(
                                'get-user-subscription-request-history.feature.request-history'
                            )}
                            tone="indigo"
                            size={34}
                            iconSize={22}
                            onclick={() => (detail = 'history')}
                        />
                        <ActionIcon
                            icon="TbDevices"
                            label={t('get-hwid-user-devices.feature.hwid-devices')}
                            tone="indigo"
                            size={34}
                            iconSize={22}
                            onclick={() => (delivery = { userId: user.id, kind: 'devices' })}
                        />
                        <ActionIcon
                            icon="TbRadar"
                            label={t('common.field.active-sessions')}
                            tone="indigo"
                            size={34}
                            iconSize={22}
                            onclick={() => (sessionUser = user.id)}
                        />
                    </div>
                </div>
                <div
                    class="traffic identity-section"
                    style:--traffic-color={`var(--mantine-color-${trafficTone}-6)`}
                >
                    <div class="traffic-values">
                        <strong
                            >{prettifyBytesUtil(user.userTraffic.usedTrafficBytes) ?? '0 B'}</strong
                        >
                        <small
                            >{user.trafficLimitBytes
                                ? prettifyBytesUtil(user.trafficLimitBytes)
                                : '∞'}</small
                        >
                    </div>
                    <progress
                        max="100"
                        value={user.trafficLimitBytes ? Math.min(trafficPercent, 100) : 100}
                        aria-label={t('use-table-columns.data-usage')}
                    ></progress>
                </div>
                <div class="identity-metrics identity-section">
                    <Tooltip block text={t('create-user-modal.widget.expiry-date')}>
                        <div
                            class="expiry"
                            class:expired={expiryDays <= 0}
                            class:ending={expiryDays > 0 && expiryDays <= 7}
                        >
                            <Icon name="TbCalendar" size={18} />
                            <span>{dayjs(user.expireAt).format('DD.MM.YYYY HH:mm')}</span>
                        </div>
                    </Tooltip>
                    <Tooltip
                        block
                        text={t('detailed-user-info-drawer.widget.lifetime-used-traffic')}
                    >
                        <div class="lifetime">
                            <Icon name="TbChartArcs" size={18} />
                            <span>
                                {prettifyBytesUtil(user.userTraffic.lifetimeUsedTrafficBytes) ??
                                    '0 B'}
                            </span>
                        </div>
                    </Tooltip>
                    {#if user.userTraffic.onlineAt}
                        <Tooltip block text={t('detailed-user-info-drawer.widget.last-online')}>
                            <div class="last-online">
                                <span
                                    style:color={getConnectionStatusColorUtil(
                                        user.userTraffic.onlineAt
                                    )}
                                    ><Icon name="TbWifi" size={18} /></span
                                >
                                <span>
                                    {getTimeAgoUtil(user.userTraffic.onlineAt, t, locale.language)}
                                </span>
                            </div>
                        </Tooltip>
                    {/if}
                    {#if lastNode}
                        <Tooltip
                            block
                            text={t('detailed-user-info-drawer.widget.last-connected-node')}
                        >
                            <button
                                type="button"
                                class="mantine-UnstyledButton-root last-node"
                                onclick={() => (nodeId = lastNode.uuid)}
                            >
                                {#if lastNode.countryCode}
                                    <span class="node-flag" aria-hidden="true">
                                        {flag(lastNode.countryCode)}
                                    </span>
                                {/if}
                                <span class="node-name">{lastNode.name}</span>
                            </button>
                        </Tooltip>
                    {/if}
                </div>
                <div class="subscription identity-section">
                    <div class="subscription-label">
                        <label for={`subscription-url-${user.id}`}
                            >{t('common.field.subscription-url')}</label
                        >
                        <ActionIcon
                            icon="HiQuestionMarkCircle"
                            label={t('common.field.subscription-url')}
                            subtle
                            size={18}
                            iconSize={16}
                            aria-expanded={subscriberHelp}
                            onclick={() => (subscriberHelp = !subscriberHelp)}
                        />
                    </div>
                    {#if subscriberHelp}
                        <p>
                            {t('view-user-modal.widget.subscription-url-description-line-1')}
                            <code>SUB_PUBLIC_DOMAIN</code>
                            {t('view-user-modal.widget.subscription-url-description-line-2')}
                        </p>
                    {/if}
                    <CopyableField
                        id={`subscription-url-${user.id}`}
                        leftIcon="PiLinkDuotone"
                        value={user.subscriptionUrl}
                    />
                </div>
            </section>
        {/snippet}
        {#snippet actions()}
            <ActionMenu
                label={t('common.action.more-actions')}
                text
                hover={false}
                preferAbove
                width={240}
            >
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    onclick={() => {
                        metadataUser = user.id
                    }}
                >
                    Metadata
                </button>
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    onclick={() => {
                        showTorrentReports = true
                    }}
                >
                    {t('constants.tb-reports')}
                </button>
                {#if user.status === 'DISABLED'}
                    <button
                        type="button"
                        role="menuitem"
                        tabindex="-1"
                        disabled={mutation.isPending}
                        onclick={() => run('enable', t('common.action.enable'))}
                    >
                        {t('common.action.enable')}
                    </button>
                {:else}
                    <button
                        type="button"
                        role="menuitem"
                        tabindex="-1"
                        disabled={mutation.isPending}
                        onclick={() => run('disable', t('common.action.disable'))}
                    >
                        {t('common.action.disable')}
                    </button>
                {/if}
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    disabled={mutation.isPending}
                    onclick={() => run('reset', t('common.action.reset-traffic'))}
                >
                    {t('common.action.reset-traffic')}
                </button>
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    disabled={mutation.isPending}
                    onclick={() => run('revoke', 'Revoke subscription')}
                >
                    Revoke subscription
                </button>
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    disabled={mutation.isPending}
                    onclick={() => run('passwords', 'Revoke passwords only')}
                >
                    Revoke passwords only
                </button>
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    disabled={mutation.isPending}
                    onclick={() => run('delete', t('common.action.delete'))}
                >
                    {t('common.action.delete')}
                </button>
            </ActionMenu>
        {/snippet}
        {#snippet overlays()}
            {#if metadataUser === user.id}
                <Metadata
                    target={{ kind: 'user', userId: user.id }}
                    close={() => (metadataUser = undefined)}
                />
            {/if}
            {#if sessionUser === user.id}
                <ActiveSessions
                    target={{ kind: 'user', id: user.id, name: user.username }}
                    close={() => (sessionUser = undefined)}
                />
            {/if}
            {#if usageUser === user.id}
                <UsageReport userId={user.id} close={() => (usageUser = undefined)} />
            {/if}
            {#if delivery?.userId === user.id}
                {#if delivery.kind === 'qr'}
                    <SubscriptionQr
                        value={user.subscriptionUrl}
                        label={t('view-user-modal.widget.subscription-qr-code')}
                        filename={`qr-${user.username}.png`}
                        close={() => (delivery = undefined)}
                    />
                {:else if delivery.kind === 'keys'}
                    <ConnectionKeys
                        userId={user.id}
                        shortUuid={user.shortUuid}
                        close={() => (delivery = undefined)}
                    />
                {:else}
                    <UserDevices userId={user.id} close={() => (delivery = undefined)} />
                {/if}
            {/if}
            {#if showTorrentReports}
                <UserTorrentReports userId={user.id} close={() => (showTorrentReports = false)} />
            {/if}
            {#if detail}
                <Overlay
                    drawer={detail !== 'history'}
                    width={detail === 'access' ? 800 : detail === 'history' ? 1000 : 620}
                    title={t(
                        detail === 'info'
                            ? 'detailed-user-info-drawer.widget.detailed-user-info'
                            : detail === 'history'
                              ? 'common.field.subscription-request-history'
                              : 'user-accessible-nodes.modal.widget.user-accessible-nodes'
                    )}
                    icon={detail === 'history'
                        ? 'TbTimeline'
                        : detail === 'access'
                          ? 'TbServer'
                          : 'PiUserDuotone'}
                    tone={detail === 'info' ? 'blue' : 'teal'}
                    close={() => (detail = undefined)}
                >
                    {#if detail === 'info'}
                        <UserInfo {user} {copy} />
                    {:else}
                        <UserAccess userId={user.id} section={detail} />
                    {/if}
                </Overlay>
            {/if}
            {#if nodeId}
                <NodeDetails {nodeId} onclose={() => (nodeId = '')} />
            {/if}
        {/snippet}
    </UserForm>
{:else if record.error || showLoading}
    <Overlay
        title={t('view-user-modal.widget.edit-user-headline')}
        icon="TbUser"
        width={1000}
        {close}
    >
        {#if record.error}
            <p role="alert">{record.error.message}</p>
            <Button onclick={() => void record.refetch()}>{t('common.action.try-again')}</Button>
        {:else}
            <RouteLoading compact />
        {/if}
    </Overlay>
{/if}

<style>
    .identity-card {
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .identity-heading,
    .identity-actions,
    .identity-group,
    .traffic-values {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
    }
    .identity-section + .identity-section {
        margin-block-start: var(--mantine-spacing-md);
        padding-block-start: var(--mantine-spacing-md);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    .user-title {
        display: grid;
        flex: 1;
    }
    .user-title button:first-child {
        font-size: var(--mantine-h5-font-size);
        font-weight: 700;
        line-height: var(--mantine-h5-line-height);
        color: var(--mantine-color-bright);
    }
    .user-title button:last-child {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .user-icon {
        place-items: center;
        display: grid;
        width: 34px;
        height: 34px;
        border: 1px solid color-mix(in srgb, var(--identity-tone) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--identity-tone) 12%, transparent);
    }
    .identity-actions {
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: 10px;
    }
    .identity-group {
        gap: 5px;
    }
    .identity-divider {
        flex: 0 0 1px;
        align-self: stretch;
        background: var(--mantine-color-default-border);
        opacity: 0.3;
    }
    .traffic-values {
        justify-content: space-between;
    }
    .traffic-values strong {
        color: var(--mantine-color-gray-3);
        font:
            600 var(--mantine-font-size-sm) / var(--mantine-line-height-sm)
            var(--mantine-font-family-monospace);
    }
    .traffic-values small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    progress {
        appearance: none;
        display: block;
        width: 100%;
        height: 5px;
        margin-top: 6px;
        border: 0;
    }
    progress::-webkit-progress-bar {
        background: var(--mantine-color-dark-4);
        border-radius: var(--mantine-radius-md);
    }
    progress::-webkit-progress-value {
        background: var(--traffic-color);
        border-radius: var(--mantine-radius-md);
    }
    progress::-moz-progress-bar {
        background: var(--traffic-color);
        border-radius: var(--mantine-radius-md);
    }
    .identity-metrics {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-xs);
    }
    .identity-metrics div,
    .identity-metrics .last-node {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        min-height: 42px;
        min-width: 0;
        width: 100%;
        padding: var(--mantine-spacing-xs);
        border: 1px solid;
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
        text-align: center;
        font-weight: 600;
    }
    .identity-metrics .expiry {
        border-color: rgb(45 212 191 / 20%);
        background: rgb(45 212 191 / 8%);
        color: var(--mantine-color-teal-5);
    }
    .identity-metrics .expired {
        color: var(--mantine-color-red-5);
        border-color: rgb(239 68 68 / 20%);
        background: rgb(239 68 68 / 8%);
    }
    .identity-metrics .ending {
        color: var(--mantine-color-yellow-4);
        border-color: rgb(251 191 36 / 20%);
        background: rgb(251 191 36 / 10%);
    }
    .identity-metrics .lifetime {
        border-color: rgb(99 102 241 / 20%);
        background: rgb(99 102 241 / 8%);
        color: var(--mantine-color-indigo-5);
    }
    .identity-metrics .last-online {
        border-color: rgb(139 92 246 / 20%);
        background: rgb(139 92 246 / 8%);
        color: var(--mantine-color-violet-4);
        font-size: var(--mantine-font-size-xs);
    }
    .identity-metrics .last-node {
        border-color: rgb(6 182 212 / 20%);
        background: rgb(6 182 212 / 8%);
        color: var(--mantine-color-cyan-5);
        cursor: pointer;
    }
    .identity-metrics .last-node:hover {
        border-color: rgb(6 182 212 / 40%);
    }
    .node-flag {
        font-size: 18px;
        line-height: 1;
        flex: 0 0 auto;
    }
    .node-name {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        min-width: 0;
    }
    .subscription {
        display: grid;
        gap: 2px;
        font-size: var(--mantine-font-size-sm);
    }
    .subscription p {
        margin: 0 0 6px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .subscription-label {
        display: flex;
        align-items: center;
        gap: 4px;
        font-weight: 500;
    }
    @media (max-width: 36em) {
        .identity-metrics {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
