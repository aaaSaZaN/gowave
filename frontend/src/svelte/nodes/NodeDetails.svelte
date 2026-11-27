<script lang="ts">
    import { buildOpenEntityUrl, OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import type { UpdateComponent } from '@shared/lib/node-updates'
    import semver from '@shared/lib/version'
    import {
        prettifyBytesUtil,
        prettySiBytesUtil,
        prettySiRealtimeBytesUtil
    } from '@shared/utils/bytes'
    import { isGoNodeVersion } from '@shared/utils/misc/node-versions'
    import {
        formatDurationUtil,
        getNodeResetDaysUtil,
        getXrayUptimeUtil
    } from '@shared/utils/time-utils'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Metadata, { nodeMetadataQuery } from '../Metadata.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, route } from '../router'
    import ActiveSessions from '../sessions/ActiveSessions.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import ActionMenu from '../ui/ActionMenu.svelte'
    import BrandIcon from '../ui/BrandIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import EntityStage from '../ui/EntityStage.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import { type NodeAction, nodeActionLabels, runNodeAction } from './actions'
    import { flag } from './analytics'
    import NodeEditor from './NodeEditor.svelte'
    import NodeGeocheck from './NodeGeocheck.svelte'
    import NodeHosts from './NodeHosts.svelte'
    import NodeLogs from './NodeLogs.svelte'
    import NodeUsersUsage from './NodeUsersUsage.svelte'
    import NodeVersions from './NodeVersions.svelte'
    import { nodeQuery, nodesKey, trafficSummaryQuery } from './queries'

    const confirmAction = useConfirmation()

    let { nodeId, onclose }: { nodeId?: string; onclose?: () => void } = $props()
    let id = $derived(nodeId ?? route.params['id'] ?? '')
    let t = $derived(locale.t)
    let metadataNode = $state('')
    let geoNode = $state('')
    let showLogs = $state(false)
    let sessionNode = $state('')
    let usageNode = $state('')
    let versionDialog = $state<{ id: string; component: UpdateComponent }>()
    let hostMode = $state<'linked' | 'inbounds'>()
    let copiedMessage = $state('')
    let copyTimeout: ReturnType<typeof setTimeout> | undefined
    onDestroy(() => clearTimeout(copyTimeout))
    const record = createQuery(() => ({ ...nodeQuery(id), refetchInterval: 5_000 }))
    const metadata = createQuery(() => ({ ...nodeMetadataQuery(id), enabled: !!record.data }))
    const minGeocheckVersion = '3.3.0'
    let canGeocheck = $derived.by(() => {
        const versions = record.data?.versions
        const version = semver.coerce(versions?.node)
        return (
            isGoNodeVersion(versions) ||
            (version !== null && semver.gte(version, minGeocheckVersion))
        )
    })
    const trafficSummary = createQuery(() => trafficSummaryQuery)
    let lifetime = $derived(
        trafficSummary.data?.nodes.find((item) => item.uuid === id)?.lifetimeTrafficBytes
    )
    const update = createMutation(() => ({
        mutationFn: (action: NodeAction) => runNodeAction(id, action),
        onSuccess: async (_data, action) => {
            if (action === 'delete') {
                const key = nodeQuery(id).queryKey
                if (onclose) onclose()
                else await navigate(ROUTES.DASHBOARD.MANAGEMENT.NODES)
                queryClient.removeQueries({ queryKey: key })
            }
            await queryClient.invalidateQueries({ queryKey: nodesKey })
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        }
    }))

    function close(): void {
        if (onclose) onclose()
        else void navigate(ROUTES.DASHBOARD.MANAGEMENT.NODES, { replace: true })
    }

    async function apply(action: NodeAction): Promise<void> {
        if (
            action !== 'enable' &&
            action !== 'disable' &&
            !(await confirmAction(t('common.message.confirm-action-description')))
        )
            return
        update.mutate(action)
    }

    function copy(text: string, oncopied?: () => void): void {
        void navigator.clipboard.writeText(text).then(
            () => {
                notifications.show({
                    title: t('common.message.copied'),
                    message: text,
                    color: 'teal'
                })
                oncopied?.()
            },
            (error: unknown) => {
                notifications.show({
                    title: t('common.message.error'),
                    message: error instanceof Error ? error.message : String(error),
                    color: 'red'
                })
            }
        )
    }
</script>

{#snippet rate(direction: 'RX' | 'TX', speed: number, total: number)}
    <div>
        <dt>
            <Icon
                name={direction === 'RX' ? 'PiArrowDownDuotone' : 'PiArrowUpDuotone'}
                color={direction === 'RX'
                    ? 'var(--mantine-color-teal-5)'
                    : 'var(--mantine-color-cyan-5)'}
                size={12}
            />
            {direction}
        </dt>
        <dd class="stat">{prettySiRealtimeBytesUtil(speed, true, true) || '0 B/s'}</dd>
        <dd class="total">{t('common.field.total')}: {prettySiBytesUtil(total) || '0 B'}</dd>
    </div>
{/snippet}

{#snippet fact(label: string, value: string, icon?: IconName, description = value)}
    <dt>
        {#if icon}
            <Icon name={icon} size={12} />
        {/if}
        {label}
    </dt>
    <dd>
        <Tooltip block text={description} disabled={!description}>
            <span class="stat">{value || '—'}</span>
        </Tooltip>
    </dd>
{/snippet}

<svelte:head>
    {#if !nodeId}
        <title>{record.data?.name ?? t('constants.nodes')} | Gowave</title>
    {/if}
</svelte:head>
{#if !nodeId}
    <EntityStage {id} title={t('constants.nodes')} icon="TbCpu" />
{/if}
<Overlay
    class="node-details"
    title={record.data?.name ?? t('edit-node-modal.widget.edit-node')}
    titlePrefix={record.data ? flag(record.data.countryCode) : undefined}
    icon="TbCpu"
    tone="teal"
    width={1000}
    inlineActions
    {close}
>
    {#snippet actions()}
        {#if record.data}
            <ActionIcon
                icon="TbLink"
                label={t('common.action.copy-link')}
                subtle
                size={26}
                iconSize={16}
                onclick={() => copy(buildOpenEntityUrl(OPEN_ENTITY.NODE, id))}
            />
        {/if}
    {/snippet}
    {#if record.isPending}
        <RouteLoading compact />
    {:else if record.isError && !record.data}
        <p role="alert">{record.error.message}</p>
        <Button onclick={() => void record.refetch()}>{t('common.action.try-again')}</Button>
    {:else if record.data}
        {@const node = record.data}
        {@const missing =
            !node.configProfile.activeConfigProfileUuid ||
            !node.configProfile.activeInbounds.length}
        {@const limit = node.isTrafficTrackingActive ? node.trafficLimitBytes : null}
        {@const percent = limit
            ? Math.min(100, Math.floor((100 * (node.trafficUsedBytes ?? 0)) / limit))
            : 100}
        {#key id}
            <NodeEditor {node} onsaved={() => undefined}>
                {#snippet details()}
                    {#if node.lastStatusMessage && node.lastStatusMessage !== 'OK'}
                        <FormCard
                            title={t(
                                node.isConnected
                                    ? 'node-error-message.config-not-applied'
                                    : 'common.message.last-error-message'
                            )}
                            description={node.isConnected
                                ? t('node-error-message.config-not-applied-description')
                                : undefined}
                            tone={node.isConnected ? 'orange' : 'red'}
                            icon="TbAlertTriangle"
                        >
                            <div class="error-message">
                                <p role="alert" class="feedback">{node.lastStatusMessage}</p>
                                <ActionIcon
                                    icon={copiedMessage === node.lastStatusMessage
                                        ? 'PiCheck'
                                        : 'PiCopy'}
                                    label={t('common.action.copy')}
                                    size={26}
                                    iconSize={14}
                                    subtle
                                    class="error-copy"
                                    data-copied={copiedMessage === node.lastStatusMessage}
                                    onclick={() => {
                                        const message = node.lastStatusMessage ?? ''
                                        copy(message, () => {
                                            copiedMessage = message
                                            clearTimeout(copyTimeout)
                                            copyTimeout = setTimeout(
                                                () => (copiedMessage = ''),
                                                2000
                                            )
                                        })
                                    }}
                                />
                            </div>
                        </FormCard>
                    {/if}
                    <FormCard
                        title={t('node-details-card.widget.node-details')}
                        tone={missing
                            ? 'red'
                            : node.isDisabled
                              ? 'gray'
                              : node.isConnected
                                ? 'teal'
                                : node.isConnecting
                                  ? 'yellow'
                                  : 'red'}
                        icon={missing
                            ? 'PiWarningCircle'
                            : node.isDisabled
                              ? 'TbWifiOff'
                              : node.isConnected
                                ? 'TbWifi'
                                : node.isConnecting
                                  ? 'PiCloudArrowUpDuotone'
                                  : 'PiWarningCircle'}
                    >
                        {#snippet actions()}
                            <div class="connection-status">
                                {#if node.isConnected && !node.isDisabled}
                                    <span class="uptime"
                                        ><Icon name="XrayLogo" size={14} />
                                        {getXrayUptimeUtil(node.xrayUptime)}</span
                                    >
                                {/if}
                                <ActionIcon
                                    icon="TbPower"
                                    tone={node.isDisabled || missing ? 'teal' : 'red'}
                                    class={missing ? 'missing-config' : 'power-action'}
                                    label={t(
                                        missing
                                            ? 'node-details-card.widget.config-profile-or-inbounds-is-missing'
                                            : node.isDisabled
                                              ? 'node-details-card.widget.enable-node'
                                              : 'node-details-card.widget.disable-node'
                                    )}
                                    size={28}
                                    iconSize={16}
                                    disabled={update.isPending || missing}
                                    onclick={() => apply(node.isDisabled ? 'enable' : 'disable')}
                                />
                            </div>
                        {/snippet}
                        <div class="detail-actions">
                            <div>
                                <ActionIcon
                                    icon="TbJson"
                                    tone="teal"
                                    size={34}
                                    iconSize={22}
                                    label="Metadata"
                                    disabled={!metadata.data}
                                    onclick={() => (metadataNode = node.uuid)}
                                />
                                <ActionIcon
                                    icon="TbTerminal2"
                                    tone="cyan"
                                    size={34}
                                    iconSize={22}
                                    label="Логи ноды / Xray"
                                    onclick={() => (showLogs = true)}
                                />
                            </div>
                            <div>
                                <ActionIcon
                                    icon="TbServerCog"
                                    tone="cyan"
                                    size={34}
                                    iconSize={22}
                                    label={t('get-node-linked-hosts.feature.linked-hosts')}
                                    onclick={() => (hostMode = 'linked')}
                                />
                                <ActionIcon
                                    icon="TbSitemap"
                                    tone="teal"
                                    size={34}
                                    iconSize={22}
                                    label={t('common.field.inbounds-hosts')}
                                    onclick={() => (hostMode = 'inbounds')}
                                />
                            </div>
                            <div>
                                <ActionIcon
                                    icon="TbChartArcs"
                                    tone="indigo"
                                    size={34}
                                    iconSize={22}
                                    label={t('common.field.usage-stats')}
                                    onclick={() => (usageNode = node.uuid)}
                                />
                                <ActionIcon
                                    icon="TbRadar"
                                    tone="indigo"
                                    size={34}
                                    iconSize={22}
                                    label={t('common.field.active-sessions')}
                                    onclick={() => (sessionNode = node.uuid)}
                                />
                                <ActionIcon
                                    icon="TbMapSearch"
                                    tone="indigo"
                                    size={34}
                                    iconSize={22}
                                    label={canGeocheck
                                        ? t('node-geocheck.title')
                                        : t('node-geocheck.requires-node-version', {
                                              version: minGeocheckVersion
                                          })}
                                    disabled={!canGeocheck}
                                    onclick={() => (geoNode = node.uuid)}
                                />
                            </div>
                        </div>
                        <div class="traffic-card">
                            <div>
                                <strong
                                    >{prettifyBytesUtil(node.trafficUsedBytes ?? 0) ||
                                        '0 B'}</strong
                                >
                                <span>{limit ? prettifyBytesUtil(limit) : '∞'}</span>
                            </div>
                            <div
                                class="progress"
                                role="progressbar"
                                aria-label={t('use-nodes-table-widget.traffic-used')}
                                aria-valuenow={percent}
                            >
                                <span
                                    style:width={`${percent}%`}
                                    style:--progress-color={limit && percent > 95
                                        ? 'var(--mantine-color-red-6)'
                                        : limit && percent > 80
                                          ? 'var(--mantine-color-yellow-6)'
                                          : 'var(--mantine-color-teal-7)'}
                                ></span>
                            </div>
                            {#if node.isTrafficTrackingActive && node.trafficResetDay}
                                <div class="traffic-note">
                                    <Icon name="PiArrowsCounterClockwise" size={12} />
                                    {t('node-stats.card.traffic-refill-in-days')}
                                    {getNodeResetDaysUtil(node.trafficResetDay)}
                                </div>
                            {/if}
                            {#if lifetime !== undefined}
                                <div
                                    class="traffic-note"
                                    title={t('node-stats.card.lifetime-traffic-description')}
                                >
                                    <Icon name="TbSum" size={12} />
                                    {t('node-stats.card.lifetime-traffic')}:
                                    <b>{prettifyBytesUtil(lifetime) || '0 B'}</b>
                                </div>
                            {/if}
                        </div>
                        {#if node.isConnected || isGoNodeVersion(node.versions)}
                            <div class="details-metrics">
                                {#if node.isConnected}
                                    <Button
                                        size="md"
                                        variant="soft"
                                        color={node.usersOnline > 0 ? 'teal' : 'gray'}
                                        class="online-users"
                                        data-empty={node.usersOnline === 0 || undefined}
                                        icon="PiUsersDuotone"
                                        title={t('nodes-quick-stats.widget.users-online')}
                                        onclick={() => (sessionNode = node.uuid)}
                                        >{node.usersOnline}</Button
                                    >
                                {/if}
                                {#if node.versions}
                                    {@const isGoNode = isGoNodeVersion(node.versions)}
                                    {@const localCore = node.versions.coreSource === 'local'}
                                    {@const coreLabel = t(
                                        localCore
                                            ? 'node-details-card.widget.local-core'
                                            : 'node-details-card.widget.xray-core-version'
                                    )}
                                    <Tooltip block text={`${coreLabel}: ${node.versions.xray}`}>
                                        <Button
                                            class="core-version"
                                            size="md"
                                            variant="soft"
                                            color="teal"
                                            block
                                            disabled={!isGoNode}
                                            aria-label={`${coreLabel} · ${node.versions.xray}`}
                                            onclick={() =>
                                                (versionDialog = {
                                                    id: node.uuid,
                                                    component: 'core'
                                                })}
                                        >
                                            <Icon name="XrayLogo" size={16} />
                                            <span class="version-text" class:version={isGoNode}
                                                >{node.versions.xray}</span
                                            >
                                            {#if isGoNode}
                                                <Icon name="TbChevronRight" size={14} />
                                            {/if}
                                        </Button>
                                    </Tooltip>
                                {/if}
                                {#if node.isConnected && node.xrayUptime}
                                    <span
                                        class="core-uptime"
                                        title={t(
                                            'node-stats.card.represents-the-uptime-of-the-xray-core'
                                        )}
                                    >
                                        <Icon name="XrayLogo" size={16} />
                                        {getXrayUptimeUtil(node.xrayUptime)}
                                    </span>
                                {/if}
                                {#if node.versions}
                                    {@const isGoNode = isGoNodeVersion(node.versions)}
                                    {@const nodeVersionLabel = t(
                                        'node-details-card.widget.gowave-node-version'
                                    )}
                                    <Tooltip
                                        block
                                        text={`${nodeVersionLabel}: ${node.versions.node}`}
                                    >
                                        <Button
                                            class="node-version"
                                            size="md"
                                            variant="soft"
                                            color="indigo"
                                            block
                                            disabled={!isGoNode}
                                            aria-label={isGoNode
                                                ? t(
                                                      'node-details-card.widget.select-node-version',
                                                      {
                                                          version: node.versions.node
                                                      }
                                                  )
                                                : nodeVersionLabel}
                                            onclick={() =>
                                                (versionDialog = {
                                                    id: node.uuid,
                                                    component: 'node'
                                                })}
                                        >
                                            <BrandIcon size={16} />
                                            <span class="version-text" class:version={isGoNode}
                                                >{node.versions.node}</span
                                            >
                                            {#if isGoNode}
                                                <Icon name="TbChevronRight" size={14} />
                                            {/if}
                                        </Button>
                                    </Tooltip>
                                {/if}
                            </div>
                        {/if}
                    </FormCard>
                {/snippet}
                {#snippet system()}
                    {#if node.system}
                        {@const { info, stats } = node.system}
                        {@const memoryPercent =
                            info.memoryTotal > 0
                                ? Math.round(
                                      Math.min(
                                          1,
                                          Math.max(0, stats.memoryUsed / info.memoryTotal)
                                      ) * 100
                                  )
                                : 0}
                        {@const memoryTone =
                            memoryPercent > 90 ? 'red' : memoryPercent > 70 ? 'yellow' : 'teal'}
                        {@const platform = [info.platform || info.type, info.arch]
                            .filter(Boolean)
                            .join(' / ')}
                        {@const os =
                            [info.platform, info.version].filter(Boolean).join(' ') || info.type}
                        {@const interfaces = info.networkInterfaces.filter((name) => name !== 'lo')}
                        {@const visibleInterfaces = interfaces.slice(0, 3).join(', ')}
                        <div class="node-system-card">
                            <FormCard
                                title={t('node-system-card.widget.system-info')}
                                tone="violet"
                                icon="TbDeviceDesktop"
                                gap="sm"
                            >
                                {#snippet actions()}
                                    <div class="badges">
                                        {#if platform}
                                            <Tooltip text={platform} class="system-platform">
                                                <span class="badge" data-tone="violet"
                                                    >{platform}</span
                                                >
                                            </Tooltip>
                                        {/if}
                                        <span class="badge system-uptime" data-tone="teal">
                                            <Icon name="PiTimerDuotone" size={12} />
                                            {formatDurationUtil(Math.floor(stats.uptime))}
                                        </span>
                                    </div>
                                {/snippet}

                                <section class="panel memory">
                                    <h4>{t('node-system-card.widget.memory')}</h4>
                                    <div class="memory-value">
                                        {prettifyBytesUtil(stats.memoryUsed) || '0 B'}
                                        /
                                        {prettifyBytesUtil(info.memoryTotal) || '0 B'}
                                        <span>({memoryPercent}%)</span>
                                    </div>
                                    <div
                                        class="meter"
                                        role="progressbar"
                                        aria-label={t('node-system-card.widget.memory')}
                                        aria-valuenow={memoryPercent}
                                        aria-valuemin="0"
                                        aria-valuemax="100"
                                    >
                                        <span
                                            style:width={`${memoryPercent}%`}
                                            style:background={`var(--mantine-color-${memoryTone}-6)`}
                                        ></span>
                                    </div>
                                </section>

                                {#if stats.interface}
                                    {@const network = stats.interface}
                                    <section class="panel network">
                                        <div class="panel-heading">
                                            <h4>{t('common.field.interface')}</h4>
                                            {#if network.interface}
                                                <span
                                                    class="badge"
                                                    data-tone="cyan"
                                                    title={network.interface}
                                                    >{network.interface}</span
                                                >
                                            {/if}
                                        </div>
                                        <dl class="network-stats">
                                            {@render rate(
                                                'RX',
                                                network.rxBytesPerSec,
                                                network.rxTotal
                                            )}
                                            {@render rate(
                                                'TX',
                                                network.txBytesPerSec,
                                                network.txTotal
                                            )}
                                        </dl>
                                    </section>
                                {/if}

                                <section class="panel hardware">
                                    <h4>{t('common.field.system')}</h4>
                                    <dl class="hardware-stats">
                                        <div class="cpu">
                                            {@render fact(
                                                'CPU',
                                                `${info.cpus} × ${info.cpuModel}`,
                                                'PiCpuDuotone'
                                            )}
                                        </div>
                                        <div>{@render fact('OS', os)}</div>
                                        <div>
                                            {@render fact(
                                                t('node-system-card.widget.kernel'),
                                                info.release
                                            )}
                                        </div>
                                        {#if interfaces.length}
                                            <div>
                                                {@render fact(
                                                    t('node-system-card.widget.network'),
                                                    interfaces.length > 3
                                                        ? `${visibleInterfaces} +${interfaces.length - 3}`
                                                        : visibleInterfaces,
                                                    'PiNetworkDuotone',
                                                    interfaces.join(', ')
                                                )}
                                            </div>
                                        {/if}
                                    </dl>
                                </section>
                            </FormCard>
                        </div>
                    {/if}
                {/snippet}
                {#snippet actions()}
                    <ActionMenu
                        label={t('common.action.more-actions')}
                        text
                        hover={false}
                        preferAbove
                        width={0}
                    >
                        <button
                            type="button"
                            role="menuitem"
                            class="node-menu-item"
                            tabindex="-1"
                            data-tone="red"
                            onclick={() => {
                                apply('delete')
                            }}
                        >
                            <Icon name="PiTrashDuotone" size={16} />
                            {t('common.action.delete')}
                        </button>
                        <div class="action-label">{t('base-node-form.management')}</div>
                        <button
                            type="button"
                            role="menuitem"
                            class="node-menu-item"
                            tabindex="-1"
                            onclick={() => {
                                copy(node.uuid)
                            }}
                        >
                            <Icon name="TbCopy" size={16} />{t('common.action.copy-uuid')}
                        </button>
                        {#each [
                            'reset',
                            'restart',
                            node.isDisabled ? 'enable' : 'disable'
                        ] as action (action)}
                            <button
                                type="button"
                                role="menuitem"
                                class="node-menu-item"
                                tabindex="-1"
                                disabled={update.isPending}
                                data-tone={action === 'reset'
                                    ? undefined
                                    : action === 'disable'
                                      ? 'red'
                                      : 'teal'}
                                onclick={() => {
                                    apply(action as NodeAction)
                                }}
                            >
                                <Icon
                                    name={action === 'reset'
                                        ? 'TbRefresh'
                                        : action === 'restart'
                                          ? 'TbRotate2'
                                          : action === 'disable'
                                            ? 'PiWifiSlash'
                                            : 'TbPower'}
                                    size={16}
                                />
                                {t(nodeActionLabels[action as NodeAction])}
                            </button>
                        {/each}
                    </ActionMenu>
                {/snippet}
            </NodeEditor>
        {/key}
        {#if update.error}
            <p role="alert">{update.error.message}</p>
        {/if}
        {#if metadataNode === node.uuid}
            <Metadata
                target={{ kind: 'node', uuid: node.uuid }}
                close={() => (metadataNode = '')}
            />
        {/if}
        {#if geoNode === node.uuid}
            <NodeGeocheck {node} close={() => (geoNode = '')} />
        {/if}
        {#if sessionNode === node.uuid}
            <ActiveSessions
                target={{ kind: 'node', uuid: node.uuid, name: node.name }}
                close={() => (sessionNode = '')}
            />
        {/if}
        {#if usageNode === node.uuid}
            <NodeUsersUsage target={node.uuid} name={node.name} close={() => (usageNode = '')} />
        {/if}
        {#if versionDialog?.id === node.uuid}
            <NodeVersions
                {node}
                initialComponent={versionDialog.component}
                close={() => (versionDialog = undefined)}
            />
        {/if}
        {#if hostMode}
            <NodeHosts {node} mode={hostMode} close={() => (hostMode = undefined)} />
        {/if}
        {#if showLogs}
            <Overlay
                title="Логи ноды и Xray"
                subtitle={node.name}
                icon="TbTerminal2"
                width={1150}
                close={() => (showLogs = false)}
            >
                <NodeLogs uuid={id} />
            </Overlay>
        {/if}
    {/if}
</Overlay>

<style>
    .connection-status,
    .uptime,
    .detail-actions,
    .detail-actions > div,
    .traffic-card > div:first-child {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .uptime {
        padding: 3px 8px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
    }
    .connection-status :global(.missing-config:disabled) {
        background: rgba(107, 114, 128, 0.15);
        border-color: rgba(107, 114, 128, 0.3);
        color: var(--mantine-color-teal-4);
        opacity: 0.7;
    }
    .detail-actions {
        justify-content: end;
        flex-wrap: wrap;
        gap: 10px;
    }
    .detail-actions > div + div {
        border-inline-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        padding-inline-start: 10px;
    }
    .traffic-card {
        padding-block-start: var(--mantine-spacing-md);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .traffic-card > div:first-child {
        justify-content: space-between;
        margin-block-end: 6px;
    }
    .traffic-note {
        display: flex;
        justify-content: center;
        align-items: center;
        gap: 4px;
        margin-top: 6px;
    }
    .traffic-note b {
        font-family: var(--mantine-font-family-monospace);
    }
    .traffic-card strong {
        font-family: var(--mantine-font-family-monospace);
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
    }
    .progress {
        height: calc(0.3125rem * var(--mantine-scale));
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-default-border);
        overflow: hidden;
    }
    .progress span {
        display: block;
        height: 100%;
        background: var(--progress-color);
    }
    .details-metrics {
        display: grid;
        padding-block-start: var(--mantine-spacing-md);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        grid-template-columns: minmax(0, 0.72fr) minmax(0, 1fr) minmax(0, 1.28fr);
        gap: var(--mantine-spacing-xs);
    }
    .details-metrics :global(.mantine-Button-root) {
        height: 40px;
        min-height: 40px;
        min-width: 0;
        padding: 6px 10px;
        overflow: hidden;
        white-space: nowrap;
        line-height: var(--mantine-line-height-sm);
        font-size: var(--mantine-font-size-sm);
    }
    .details-metrics :global(.mantine-Button-root:is(.online-users, .core-version)),
    .core-uptime {
        background: rgba(20, 184, 166, 0.08);
        border-color: rgba(20, 184, 166, 0.25);
    }
    .details-metrics :global(.mantine-Button-root.online-users[data-empty]) {
        background: rgb(107 114 128 / 8%);
        border-color: rgb(107 114 128 / 20%);
        color: var(--mantine-color-gray-6);
    }
    .details-metrics :global(.mantine-Button-root.node-version) {
        background: rgba(99, 102, 241, 0.08);
        border-color: rgba(99, 102, 241, 0.25);
    }
    .details-metrics :global(.mantine-Button-label) {
        flex-wrap: nowrap;
        justify-content: center;
        align-items: center;
        min-width: 0;
        max-width: 100%;
        height: auto;
        white-space: nowrap;
        gap: 6px;
    }
    .details-metrics :global(.mantine-Button-label svg) {
        flex-shrink: 0;
    }
    .version-text {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .core-uptime {
        display: none;
        min-height: 42px;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        border: 1px solid;
        border-radius: var(--mantine-radius-md);
        border-color: rgb(20 184 166 / 20%);
        color: var(--mantine-color-teal-5);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        text-transform: uppercase;
    }
    .version {
        text-decoration: underline dotted;
        text-underline-offset: 3px;
    }
    .error-message :global(.error-copy) {
        color: var(--mantine-color-dimmed);
    }
    .error-message :global(.error-copy[data-copied="true"]) {
        color: var(--mantine-color-teal-4);
    }
    .connection-status :global(.power-action:not(:disabled)) {
        box-shadow: 0 0 10px color-mix(in srgb, var(--action-edge) 25%, transparent);
    }
    .error-message {
        position: relative;
        padding: 12px 44px 12px 14px;
        background: rgba(var(--gw-surface-rgb), 0.6);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-inline-start: 2px solid var(--mantine-color-red-5);
        border-radius: var(--mantine-radius-md);
    }
    .error-message :global(button) {
        position: absolute;
        inset-block-start: 8px;
        inset-inline-end: 8px;
    }
    .feedback {
        font: var(--mantine-font-size-xs) / 1.6 var(--mantine-font-family-monospace);
        color: var(--mantine-color-red-1);
        white-space: pre-wrap;
        max-height: 240px;
        overflow-y: auto;
        margin: 0;
        overflow-wrap: anywhere;
    }
    .node-menu-item[role="menuitem"] {
        min-height: 35px;
        padding-inline: 12px;
        white-space: nowrap;
    }
    .action-label {
        min-height: 38px;
        font-weight: 600;
        padding: 7px 10px 3px;
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    @media (max-width: 48em) {
        .details-metrics {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
        .core-uptime {
            display: flex;
        }
        .connection-status .uptime {
            display: none;
        }
    }
    @media (max-width: 36em) {
        .details-metrics {
            grid-template-columns: minmax(0, 1fr);
        }
    }

    .node-system-card {
        container-type: inline-size;

        .badges,
        .panel-heading,
        dt,
        .badge {
            display: flex;
            align-items: center;
            gap: 4px;
        }
        .badges {
            min-width: 0;
            flex: 0 1 auto;
            max-width: 100%;
            gap: var(--mantine-spacing-xs);
        }
        .badge {
            display: inline-block;
            min-width: 0;
            max-width: 100%;
            padding: 2px 8px;
            overflow: hidden;
            border-radius: var(--mantine-radius-md);
            font: 700 10px / 1.4 var(--mantine-font-family-monospace);
            text-transform: uppercase;
            text-overflow: ellipsis;
            white-space: nowrap;
            color: var(--badge-ink);
            background: var(--badge-bg);
        }
        .badge[data-tone="violet"] {
            --badge-ink: var(--mantine-color-violet-light-color);
            --badge-bg: var(--mantine-color-violet-light);
        }
        .badge[data-tone="teal"] {
            --badge-ink: var(--mantine-color-teal-light-color);
            --badge-bg: var(--mantine-color-teal-light);
        }
        .badge[data-tone="cyan"] {
            --badge-ink: var(--mantine-color-cyan-light-color);
            --badge-bg: var(--mantine-color-cyan-light);
        }
        .badges :global(.system-platform) {
            max-width: 16ch;
        }
        .badge.system-uptime {
            display: flex;
            flex: none;
            font-size: var(--mantine-font-size-xs);
        }

        .panel {
            display: grid;
            gap: 6px;
            min-width: 0;
            padding: 8px 12px;
            border: 1px solid color-mix(in srgb, var(--panel-tint) 12%, transparent);
            border-radius: var(--mantine-radius-sm);
            background: color-mix(in srgb, var(--panel-tint) 6%, transparent);
        }
        .memory {
            --panel-tint: var(--mantine-color-teal-4);
        }
        .network {
            --panel-tint: var(--mantine-color-cyan-4);
        }
        .hardware {
            --panel-tint: var(--mantine-color-violet-4);
        }
        .panel-heading {
            justify-content: space-between;
        }
        h4 {
            margin: 0;
            color: var(--mantine-color-dimmed);
            font-size: 10px;
            font-weight: 600;
            letter-spacing: 0.08em;
            line-height: 1.2;
            text-transform: uppercase;
        }
        .memory-value,
        .stat,
        .total {
            font-family: var(--mantine-font-family-monospace);
        }
        .memory-value,
        .stat {
            font-size: var(--mantine-font-size-xs);
            font-weight: 600;
        }
        .memory-value span,
        .total {
            color: var(--mantine-color-dimmed);
        }
        .meter {
            height: calc(0.3125rem * var(--mantine-scale));
            overflow: hidden;
            border-radius: var(--mantine-radius-xl);
            background: var(--mantine-color-default-border);
        }
        .meter span {
            display: block;
            height: 100%;
            border-radius: inherit;
        }
        dl,
        dd {
            min-width: 0;
            margin: 0;
        }
        dl {
            display: grid;
            gap: 6px var(--mantine-spacing-sm);
        }
        dl > div {
            min-width: 0;
        }
        dt {
            color: var(--mantine-color-dimmed);
            font-size: var(--mantine-font-size-xs);
            font-weight: 500;
            line-height: 1.2;
        }
        .stat {
            display: block;
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .total {
            font-size: 10px;
            overflow-wrap: anywhere;
        }
        .network-stats {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
        .hardware-stats {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }
        .cpu {
            grid-column: 1 / -1;
        }

        @container (max-width: 24rem) {
            .badges :global(.system-platform) {
                display: none;
            }
            .hardware-stats {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }
        }
    }
</style>
