<script lang="ts">
    import { buildOpenEntityUrl, OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import type { UpdateComponent } from '@shared/lib/node-updates'
    import semver from '@shared/lib/version'
    import {
        prettifyBytesUtil,
        prettySiRealtimeBytesUtil
    } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
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
                    {@const system = node.system}
                    {#if system}
                        {@const memoryTotal = system.info.memoryTotal}
                        {@const memoryUsed = system.stats.memoryUsed}
                        {@const memoryPercent = memoryTotal
                            ? Math.min(100, Math.round((memoryUsed * 100) / memoryTotal))
                            : 0}
                        {@const network = system.stats.interface}
                        <FormCard
                            title={t('node-system-card.widget.system-info')}
                            tone="violet"
                            icon="TbDeviceDesktop"
                        >
                            {#snippet actions()}
                                <div class="system-badges">
                                    <Tooltip text={`${system.info.platform} / ${system.info.arch}`}>
                                        <span class="system-platform"
                                            >{system.info.platform}
                                            / {system.info.arch}</span
                                        >
                                    </Tooltip>
                                    <span class="system-uptime">
                                        <Icon name="PiClockCountdown" size={14} />
                                        {formatDurationUtil(system.stats.uptime)}
                                    </span>
                                </div>
                            {/snippet}
                            <section class="system-panel memory-panel">
                                <div class="system-panel-heading">
                                    {t('node-system-card.widget.memory')}
                                </div>
                                <div class="memory-values">
                                    <strong
                                        >{prettifyBytesUtil(memoryUsed, true)}
                                        / {prettifyBytesUtil(memoryTotal, true)}</strong
                                    >
                                    <span>({memoryPercent}%)</span>
                                </div>
                                <div
                                    class="system-meter"
                                    role="progressbar"
                                    aria-label="Memory usage"
                                    aria-valuenow={memoryPercent}
                                    aria-valuemin="0"
                                    aria-valuemax="100"
                                >
                                    <span style:width={`${memoryPercent}%`}></span>
                                </div>
                            </section>
                            {#if network}
                                <section class="system-panel network-panel">
                                    <div class="system-panel-heading">
                                        <span>{t('common.field.interface')}</span>
                                        <b>{network.interface || '—'}</b>
                                    </div>
                                    <div class="network-stats">
                                        <div>
                                            <span
                                                ><Icon name="PiArrowDownDuotone" size={14} />
                                                RX</span
                                            >
                                            <strong
                                                >{prettySiRealtimeBytesUtil(
                                                    network.rxBytesPerSec,
                                                    true,
                                                    true
                                                )}</strong
                                            >
                                            <small
                                                >{t('common.field.total')}:
                                                {prettifyBytesUtil(network.rxTotal, true)}</small
                                            >
                                        </div>
                                        <div>
                                            <span
                                                ><Icon name="PiArrowUpDuotone" size={14} />
                                                TX</span
                                            >
                                            <strong
                                                >{prettySiRealtimeBytesUtil(
                                                    network.txBytesPerSec,
                                                    true,
                                                    true
                                                )}</strong
                                            >
                                            <small
                                                >{t('common.field.total')}:
                                                {prettifyBytesUtil(network.txTotal, true)}</small
                                            >
                                        </div>
                                    </div>
                                </section>
                            {/if}
                            <section class="system-panel hardware-panel">
                                <span class="system-panel-heading">{t('common.field.system')}</span>
                                <div class="hardware-stats">
                                    <div>
                                        <span><Icon name="PiCpuDuotone" size={13} /> CPU</span>
                                        <strong>{system.info.cpus} x {system.info.cpuModel}</strong>
                                    </div>
                                    <div>
                                        <span>OS</span>
                                        <strong
                                            >{system.info.platform} {system.info.version}</strong
                                        >
                                    </div>
                                    <div>
                                        <span>{t('node-system-card.widget.kernel')}</span>
                                        <strong>{system.info.release}</strong>
                                    </div>
                                    <div>
                                        <span>{t('node-system-card.widget.network')}</span>
                                        <strong>{system.info.networkInterfaces.join(', ')}</strong>
                                    </div>
                                </div>
                            </section>
                        </FormCard>
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
    .system-uptime {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: 4px 8px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        text-transform: uppercase;
    }
    .system-platform {
        padding: 2px 8px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-violet-light);
        color: var(--mantine-color-violet-light-color);
        font: bold 10px var(--mantine-font-family-monospace);
        text-transform: uppercase;
    }
    .system-panel {
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-sm);
        background: var(--gw-raised-bg);
    }
    .memory-panel {
        border-color: color-mix(in srgb, var(--mantine-color-teal-5) 25%, transparent);
        background: color-mix(in srgb, var(--mantine-color-teal-6) 8%, var(--gw-raised-bg));
    }
    .network-panel {
        border-color: color-mix(in srgb, var(--mantine-color-cyan-5) 25%, transparent);
        background: color-mix(in srgb, var(--mantine-color-cyan-6) 7%, var(--gw-raised-bg));
    }
    .hardware-panel {
        border-color: color-mix(in srgb, var(--mantine-color-violet-5) 25%, transparent);
        background: color-mix(in srgb, var(--mantine-color-violet-6) 7%, var(--gw-raised-bg));
    }
    .system-panel-heading,
    .network-stats > div > span,
    .hardware-stats > div > span {
        display: flex;
        align-items: center;
        gap: 6px;
        color: var(--mantine-color-dimmed);
        font-size: 10px;
        font-weight: 700;
        letter-spacing: 0.06em;
        text-transform: uppercase;
    }
    .system-panel-heading {
        margin-block-end: var(--mantine-spacing-xs);
    }
    .system-badges {
        display: flex;
        flex-wrap: nowrap;
        align-items: center;
        justify-content: flex-end;
        gap: var(--mantine-spacing-xs);
        min-width: 0;
        max-width: 100%;
    }
    .system-uptime {
        flex-shrink: 0;
        white-space: nowrap;
    }
    .system-platform {
        max-width: 22ch;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    @media (max-width: 40em) {
        .system-platform {
            display: none;
        }
    }
    .memory-values {
        display: flex;
        gap: 4px;
        margin-block-end: 5px;
        font: 700 var(--mantine-font-size-xs) var(--mantine-font-family-monospace);
    }
    .memory-panel .memory-values span {
        color: var(--mantine-color-dimmed);
    }
    .network-panel .system-panel-heading b {
        margin-inline-start: auto;
        padding: 1px 6px;
        border: 1px solid var(--mantine-color-cyan-light-hover);
        border-radius: var(--mantine-radius-xl);
        color: var(--mantine-color-cyan-light-color);
        background: var(--mantine-color-cyan-light);
        font-family: var(--mantine-font-family-monospace);
    }
    .system-meter {
        height: 5px;
        overflow: hidden;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-default-border);
    }
    .memory-panel .system-meter > span {
        display: block;
        height: 100%;
        border-radius: inherit;
        background: var(--mantine-color-teal-6);
    }
    .network-stats {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-md);
    }
    .system-panel .network-stats > div,
    .system-panel .hardware-stats > div {
        display: grid;
        min-width: 0;
        gap: 2px;
    }
    .network-stats strong,
    .hardware-stats strong {
        overflow: hidden;
        color: var(--mantine-color-text);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .network-stats small {
        color: var(--mantine-color-dimmed);
        font-size: 10px;
    }
    .hardware-stats {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: var(--mantine-spacing-sm);
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
        .hardware-stats {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
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
</style>
