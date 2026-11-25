<script lang="ts">
    import {
        chooseRelease,
        compatibleReleases,
        type ReleaseChannel,
        type UpdateComponent
    } from '@shared/lib/node-updates'
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
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
    let dialog: HTMLDialogElement
    const title = $props.id()
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
    let current = $derived(
        component === 'node'
            ? releases.data?.currentVersion || node.versions?.node
            : releases.data?.currentCoreVersion || node.versions?.xray
    )
    let request = $derived.by((): NodeUpdate | undefined => {
        if (coreLocked) return undefined
        if (source)
            return validHttpsDownload(source) ? { type: 'core', downloadUrl: source } : undefined
        if (release && !queryError) return { type: component, version: release.tag_name }
        return undefined
    })

    onMount(() => dialog.showModal())

    function changed(): void {
        confirmed = false
        accepted = false
        installError = ''
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

<dialog
    class="node-versions"
    bind:this={dialog}
    aria-labelledby={title}
    onclose={close}
    oncancel={(event) => cancelDialog(event, installing)}
>
    <div class="button-row header">
        <h2 id={title}>{t('node-versions.title')} · {node.name}</h2>
        <button type="button" disabled={installing} onclick={() => dismissDialog(dialog)}>
            {t('common.action.close')}
        </button>
    </div>
    {#if !coreLocked}
        <p>
            {t('node-versions.restart-warning')}
        </p>
    {/if}
    <div class="button-row">
        {#each tabs as key (key)}
            <button
                type="button"
                disabled={installing}
                aria-pressed={component === key}
                onclick={() => {
                    selected = key
                    changed()
                }}
            >
                {components[key]}
            </button>
        {/each}
    </div>
    <p>
        {t('node-versions.installed', { version: current || '—' })}{releases.data?.architecture
            ? ` · ${releases.data.os || 'linux'}/${releases.data.architecture}`
            : ''}
    </p>
    {#if coreLocked}
        <p role="status">
            {t('node-details-card.widget.local-core')}.
            {t('node-details-card.widget.local-core-override')}
            <code>gowave-node core update</code>
        </p>
    {:else}
        {#if releases.isPending}
            <p role="status">{t('node-versions.loading')}</p>
        {/if}
        {#if queryError}
            <p role="alert">{queryError}</p>
            <button
                type="button"
                disabled={releases.isFetching || installing}
                onclick={() => void releases.refetch()}
            >
                {t('common.action.try-again')}
            </button>
        {/if}
        <form onsubmit={(event) => void submit(event)} novalidate>
            <fieldset disabled={installing}>
                <label
                    >{t('node-versions.channel')}
                    <select name="releaseChannel" bind:value={form.channel} onchange={changed}>
                        <option value="stable">{t('node-versions.stable')}</option>
                        <option value="prerelease">{t('node-versions.prerelease')}</option>
                        <option value="version">{t('node-versions.specific')}</option>
                    </select>
                </label>
                {#if form.channel === 'version'}
                    <label
                        >{t('node-versions.version')}
                        <select name="releaseVersion" bind:value={form.version} onchange={changed}>
                            <option value="">{t('node-versions.select')}</option>
                            {#each compatible as item (item.tag_name)}
                                <option value={item.tag_name}>
                                    {item.tag_name}{item.prerelease
                                        ? ` · ${t('node-versions.prerelease-label')}`
                                        : ''}
                                </option>
                            {/each}
                        </select>
                    </label>
                {/if}
                {#if component === 'core'}
                    <label
                        >{t('node-versions.url')}
                        <input
                            type="url"
                            name="downloadUrl"
                            bind:value={download}
                            oninput={changed}
                        >
                    </label>
                    {#if source && !validHttpsDownload(source)}
                        <p role="alert">{t('node-versions.invalid-url')}</p>
                    {/if}
                {/if}
                <p>
                    {t('node-versions.target', {
                        target:
                            source && validHttpsDownload(source)
                                ? t('node-versions.direct')
                                : release?.tag_name || '—'
                    })}
                </p>
                {#if !source && !queryError && !releases.isPending && compatible.length === 0}
                    <p>{t('node-versions.empty')}</p>
                {/if}
                <label class="confirm">
                    <input
                        type="checkbox"
                        name="confirm"
                        bind:checked={confirmed}
                        disabled={!request || accepted}
                    >
                    {t('node-versions.confirm', { component: components[component] })}
                </label>
                <button type="submit" disabled={!confirmed || !request || accepted}>
                    {installing ? t('node-versions.installing') : t('node-versions.install')}
                </button>
            </fieldset>
        </form>
        {#if installError}
            <p role="alert">{installError}</p>
        {/if}
        {#if accepted}
            <p role="status">
                {t('node-versions.accepted')}
            </p>
        {/if}
    {/if}
</dialog>

<style>
    .node-versions {
        width: min(36rem, calc(100vw - 2rem));
    }
    .header {
        align-items: center;
        justify-content: space-between;
    }
    h2 {
        margin: 0;
    }
    fieldset {
        display: grid;
        border: 0;
        padding: 0;
        gap: 0.6rem;
        min-width: 0;
    }
    label {
        display: grid;
        gap: 0.25rem;
    }
    .confirm {
        display: flex;
        align-items: baseline;
    }
    fieldset p {
        margin: 0;
    }
</style>
