<script lang="ts">
    import {
        chooseRelease,
        compatibleReleases,
        type ReleaseChannel,
        type UpdateComponent
    } from '@shared/lib/node-updates'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import BrandIcon from '../ui/BrandIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { dismissDialog } from '../ui/dialog-motion'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import {
        installNodeUpdate,
        LocalCoreUpdateError,
        type NodeUpdate,
        nodeReleasesQuery,
        validHttpsDownload
    } from './node-update-queries'
    import { type NodeDetail, nodesKey } from './queries'

    let {
        node,
        initialComponent,
        close
    }: {
        node: NodeDetail
        initialComponent: UpdateComponent
        close: () => void
    } = $props()

    let t = $derived(locale.t)
    let selected = $state<UpdateComponent>()
    let component = $derived(selected ?? initialComponent)
    const components = { node: 'Gowave Node Go', core: 'Xray Core' }
    const tabs = ['node', 'core'] as const
    let choices = $state<Record<UpdateComponent, { channel: ReleaseChannel; version: string }>>({
        node: { channel: 'stable', version: '' },
        core: { channel: 'stable', version: '' }
    })
    let download = $state('')
    let confirmed = $state(false)
    let installing = $state(false)
    let accepted = $state(false)
    let installError = $state('')
    let localCoreConflict = $state(false)
    let coreLocked = $derived(
        component === 'core' && (node.versions?.coreSource === 'local' || localCoreConflict)
    )
    let form = $derived(choices[component])
    let source = $derived(component === 'core' ? download.trim() : '')
    const releases = createQuery(() => ({
        ...nodeReleasesQuery(node.uuid, component),
        enabled: !coreLocked
    }))
    let queryError = $derived(releases.error?.message || releases.data?.error)
    let compatible = $derived(releases.data ? compatibleReleases(component, releases.data) : [])
    let release = $derived(chooseRelease(compatible, form.channel, form.version))
    let stablePreview = $derived(chooseRelease(compatible, 'stable', '')?.tag_name)
    let prereleasePreview = $derived(chooseRelease(compatible, 'prerelease', '')?.tag_name)
    let current = $derived(
        component === 'node'
            ? releases.data?.currentVersion || node.versions?.node
            : releases.data?.currentCoreVersion || node.versions?.xray
    )
    let targetLabel = $derived(
        source && validHttpsDownload(source) ? t('node-versions.direct') : release?.tag_name || '—'
    )
    let hasTarget = $derived(Boolean((source && validHttpsDownload(source)) || release?.tag_name))
    let installedParts = $derived(t('node-versions.installed', { version: '\0' }).split('\0'))
    let targetParts = $derived(t('node-versions.target', { target: '\0' }).split('\0'))
    let channelOptions = $derived([
        {
            value: 'stable',
            label: `${t('node-versions.stable')}${form.channel === 'stable' && stablePreview ? ` · ${stablePreview}` : ''}`
        },
        {
            value: 'prerelease',
            label: `${t('node-versions.prerelease')}${form.channel === 'prerelease' && prereleasePreview ? ` · ${prereleasePreview}` : ''}`
        },
        { value: 'version', label: t('node-versions.specific') }
    ])
    let versionOptions = $derived(
        compatible.map((item) => ({
            value: item.tag_name,
            label: `${item.tag_name}${item.prerelease ? ` · ${t('node-versions.prerelease-label')}` : ''}`
        }))
    )
    let request = $derived.by((): NodeUpdate | undefined => {
        if (coreLocked) return undefined
        if (source)
            return validHttpsDownload(source) ? { type: 'core', downloadUrl: source } : undefined
        if (release && !queryError) return { type: component, version: release.tag_name }
        return undefined
    })

    function changed(): void {
        confirmed = false
        accepted = false
        installError = ''
    }

    function requestClose(event: MouseEvent): void {
        if (installing) return
        const dialog =
            event.currentTarget instanceof Element ? event.currentTarget.closest('dialog') : null
        if (dialog instanceof HTMLDialogElement) dismissDialog(dialog)
        else close()
    }

    async function submit(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (!confirmed || !request || installing || accepted) return
        installing = true
        installError = ''
        try {
            await installNodeUpdate(node.uuid, request)
            confirmed = false
            accepted = true
            notifications.show({
                title: t('common.message.success'),
                message: t('node-versions.accepted'),
                color: 'teal'
            })
            void queryClient.invalidateQueries({ queryKey: nodesKey })
        } catch (error) {
            if (error instanceof LocalCoreUpdateError) {
                localCoreConflict = true
                confirmed = false
            } else {
                installError = error instanceof Error ? error.message : String(error)
            }
        } finally {
            installing = false
        }
    }
</script>

<Overlay
    title={t('node-versions.title')}
    subtitle={node.name}
    icon="TbCloudDownload"
    tone="blue"
    width={640}
    preventClose={installing}
    {close}
>
    <div class="node-versions-body">
        {#if !coreLocked}
            <div class="alert alert-orange" role="note">
                <span class="alert-icon"><Icon name="TbAlertTriangle" size={20} /></span>
                <p>{t('node-versions.restart-warning')}</p>
            </div>
        {/if}

        <div class="tabs" role="tablist" aria-label={t('node-versions.title')}>
            {#each tabs as tab (tab)}
                <button
                    type="button"
                    role="tab"
                    class="tab"
                    class:active={component === tab}
                    aria-selected={component === tab}
                    disabled={installing}
                    onclick={() => {
                        selected = tab
                        changed()
                    }}
                >
                    {#if tab === 'node'}
                        <BrandIcon size={15} />
                    {:else}
                        <Icon name="TbCpu" size={16} />
                    {/if}
                    <span>{components[tab]}</span>
                </button>
            {/each}
        </div>

        {#if coreLocked}
            <div class="alert alert-blue" role="status">
                <span class="alert-icon"><Icon name="TbCpu" size={20} /></span>
                <div class="alert-stack">
                    <strong>{t('node-details-card.widget.local-core')}</strong>
                    <p>{t('node-details-card.widget.local-core-override')}</p>
                    <code>gowave-node core update</code>
                </div>
            </div>
        {:else if releases.isPending}
            <div class="loading-box" role="status">
                <span class="spinner" aria-hidden="true"></span>
                <span>{t('node-versions.loading')}</span>
            </div>
        {:else}
            {#if queryError}
                <div class="alert alert-red" role="alert">
                    <span class="alert-icon"><Icon name="TbAlertTriangle" size={18} /></span>
                    <div class="alert-stack">
                        <p>{queryError}</p>
                        <div>
                            <Button
                                size="xs"
                                variant="default"
                                disabled={releases.isFetching || installing}
                                onclick={() => void releases.refetch()}
                            >
                                {t('common.action.try-again')}
                            </Button>
                        </div>
                    </div>
                </div>
            {/if}

            <form class="versions-form" onsubmit={(event) => void submit(event)} novalidate>
                <fieldset disabled={installing}>
                    <div class="installed-row">
                        <div class="installed-group">
                            <span class="muted-label">{installedParts[0] ?? ''}</span>
                            <span class="badge badge-gray mono">{current || '—'}</span>
                            {#if installedParts[1]}
                                <span class="muted-label">{installedParts[1]}</span>
                            {/if}
                        </div>
                        {#if releases.data?.architecture}
                            <span class="badge badge-outline mono">
                                {releases.data.os || 'linux'}/{releases.data.architecture}
                            </span>
                        {/if}
                    </div>

                    <hr class="divider">

                    <SelectField
                        name="releaseChannel"
                        label={t('node-versions.channel')}
                        placeholder={t('node-versions.channel')}
                        searchable={false}
                        clearable={false}
                        options={channelOptions}
                        value={form.channel}
                        onchoose={(value) => {
                            form.channel = value as ReleaseChannel
                            changed()
                        }}
                    />

                    {#if form.channel === 'version'}
                        <SelectField
                            name="releaseVersion"
                            label={t('node-versions.version')}
                            placeholder={t('node-versions.select')}
                            clearable={false}
                            options={versionOptions}
                            value={form.version}
                            onchoose={(value) => {
                                form.version = value
                                changed()
                            }}
                        />
                    {/if}

                    {#if component === 'core'}
                        <div class="mantine-InputWrapper-root url-field">
                            <label for="node-core-url" class="mantine-InputWrapper-label">
                                {t('node-versions.url')}
                            </label>
                            <input
                                id="node-core-url"
                                type="url"
                                name="downloadUrl"
                                class="mantine-Input-input"
                                placeholder="https://example.org/Xray-linux-64.zip"
                                bind:value={download}
                                oninput={changed}
                            >
                            {#if source && !validHttpsDownload(source)}
                                <p class="field-error" role="alert">
                                    {t('node-versions.invalid-url')}
                                </p>
                            {/if}
                        </div>
                    {/if}

                    {#if !source && !queryError && compatible.length === 0}
                        <p class="empty-note">{t('node-versions.empty')}</p>
                    {/if}

                    <div class="target-row">
                        <span class="muted-xs">{targetParts[0]?.trim() || 'Target:'}</span>
                        <span class="badge badge-outline mono">{current || '—'}</span>
                        <span class="arrow-icon" aria-hidden="true">
                            <Icon name="TbArrowRight" size={14} />
                        </span>
                        <span
                            class="badge mono"
                            class:badge-blue-filled={hasTarget}
                            class:badge-gray={!hasTarget}
                        >
                            {targetLabel}
                        </span>
                    </div>

                    <hr class="divider">

                    <label class="confirm">
                        <input
                            type="checkbox"
                            name="confirm"
                            bind:checked={confirmed}
                            disabled={!request || accepted}
                        >
                        <span
                            >{t('node-versions.confirm', {
                                component: components[component]
                            })}</span
                        >
                    </label>

                    {#if installError}
                        <div class="alert alert-red" role="alert">
                            <span class="alert-icon"
                                ><Icon name="TbAlertTriangle" size={18} /></span
                            >
                            <p>{installError}</p>
                        </div>
                    {/if}

                    {#if accepted}
                        <div class="alert alert-teal" role="status">
                            <span class="alert-icon"><Icon name="TbCheck" size={18} /></span>
                            <p>{t('node-versions.accepted')}</p>
                        </div>
                    {/if}

                    <div class="modal-actions">
                        <Button variant="default" disabled={installing} onclick={requestClose}>
                            {t('common.action.close')}
                        </Button>
                        <Button
                            type="submit"
                            variant="filled"
                            color="blue"
                            disabled={!confirmed || !request || accepted || installing}
                            aria-busy={installing || undefined}
                        >
                            {installing
                                ? t('node-versions.installing')
                                : t('node-versions.install')}
                        </Button>
                    </div>
                </fieldset>
            </form>
        {/if}
    </div>
</Overlay>

<style>
    .node-versions-body {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-md);
    }

    .alert {
        display: flex;
        align-items: flex-start;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md);
        border-radius: var(--mantine-radius-md);
        border: 1px solid transparent;
        font-size: var(--mantine-font-size-sm);
        line-height: 1.45;
    }

    .alert p {
        margin: 0;
    }

    .alert-icon {
        display: inline-flex;
        flex-shrink: 0;
        margin-top: 1px;
    }

    .alert-stack {
        display: grid;
        gap: var(--mantine-spacing-xs);
        min-width: 0;
    }

    .alert-orange {
        background: var(--mantine-color-orange-light);
        color: var(--mantine-color-orange-light-color);
        border-color: rgba(253, 126, 20, 0.3);
    }

    .alert-blue {
        background: var(--mantine-color-blue-light);
        color: var(--mantine-color-blue-light-color);
        border-color: rgba(34, 139, 230, 0.3);
    }

    .alert-red {
        background: var(--mantine-color-red-light);
        color: var(--mantine-color-red-light-color);
        border-color: rgba(250, 82, 82, 0.3);
    }

    .alert-teal {
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
        border-color: rgba(18, 184, 134, 0.3);
    }

    .alert code {
        display: inline-block;
        padding: 2px 6px;
        border-radius: var(--mantine-radius-xs);
        background: rgba(var(--gw-fg-rgb), 0.08);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
    }

    .tabs {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        border-bottom: 1px solid rgba(var(--gw-fg-rgb), 0.12);
    }

    .tab {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        padding: 10px var(--mantine-spacing-md);
        border: 0;
        border-bottom: 2px solid transparent;
        margin-bottom: -1px;
        background: transparent;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        cursor: pointer;
        transition:
            color 120ms ease,
            background-color 120ms ease,
            border-color 120ms ease;
    }

    .tab:hover:not(:disabled) {
        background: rgba(var(--gw-fg-rgb), 0.04);
        color: var(--mantine-color-text);
    }

    .tab.active {
        border-bottom-color: var(--mantine-primary-color-filled);
        color: var(--mantine-color-bright);
        font-weight: 600;
    }

    .loading-box {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-xl);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }

    .spinner {
        width: 18px;
        height: 18px;
        border-radius: 999px;
        border: 2px solid rgba(34, 139, 230, 0.25);
        border-top-color: var(--mantine-color-blue-filled);
        animation: spin 750ms linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }

    .versions-form fieldset {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
        margin: 0;
        padding: 0;
        border: 0;
    }

    .installed-row {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
    }

    .installed-group,
    .target-row {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }

    .muted-label {
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        color: var(--mantine-color-text);
    }

    .muted-xs,
    .arrow-icon,
    .empty-note {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }

    .arrow-icon {
        display: inline-flex;
        align-items: center;
    }

    .badge {
        display: inline-flex;
        align-items: center;
        padding: 3px 10px;
        border-radius: 999px;
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
        line-height: 1.4;
    }

    .mono {
        font-family: var(--mantine-font-family-monospace);
    }

    .badge-gray {
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
    }

    .badge-outline {
        border: 1px solid var(--mantine-color-default-border);
        color: var(--mantine-color-dimmed);
    }

    .badge-blue-filled {
        background: var(--mantine-color-blue-filled);
        color: var(--mantine-color-white);
    }

    .divider {
        width: 100%;
        height: 1px;
        margin: 0;
        border: 0;
        background: rgba(var(--gw-fg-rgb), 0.08);
    }

    .url-field {
        display: grid;
        gap: 4px;
    }

    .url-field .mantine-Input-input {
        --input-height: var(--input-height-sm);
        --input-padding: calc(var(--input-height) / 3);
        --input-padding-inline-start: var(--input-padding);
        --input-padding-inline-end: var(--input-padding);
        --input-line-height: calc(var(--input-height) - 2px);
        --input-radius: var(--mantine-radius-default);
        --input-fz: var(--mantine-font-size-sm);
    }

    .field-error {
        margin: 0;
        color: var(--mantine-color-red-text);
        font-size: var(--mantine-font-size-xs);
    }

    .confirm {
        display: flex;
        align-items: flex-start;
        gap: var(--mantine-spacing-sm);
        font-size: var(--mantine-font-size-sm);
        color: var(--mantine-color-text);
        cursor: pointer;
    }

    .confirm input {
        margin-top: 3px;
        accent-color: var(--mantine-color-orange-filled);
        width: 16px;
        height: 16px;
        flex-shrink: 0;
    }

    .modal-actions {
        display: flex;
        justify-content: flex-end;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        margin-top: var(--mantine-spacing-xs);
    }
</style>
