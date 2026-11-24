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
    oncancel={(event) => {
        if (installing) event.preventDefault()
    }}
>
    <div class="button-row header">
        <h2 id={title}>Manage versions · {node.name}</h2>
        <button type="button" disabled={installing} onclick={() => dialog.close()}>
            {t('common.action.close')}
        </button>
    </div>
    {#if !coreLocked}
        <p>
            Installing an update restarts the node service and briefly interrupts active
            connections.
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
        Installed: {current || '—'}{releases.data?.architecture
            ? ` · ${releases.data.os || 'linux'}/${releases.data.architecture}`
            : ''}
    </p>
    {#if coreLocked}
        <p role="status">
            {t('node-details-card.widget.local-core')}.
            {t('node-details-card.widget.local-core-update')}
            <code>gowave-node core update</code>
        </p>
    {:else}
        {#if releases.isPending}
            <p role="status">Loading releases…</p>
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
                    >Release channel
                    <select name="releaseChannel" bind:value={form.channel} onchange={changed}>
                        <option value="stable">Latest release</option>
                        <option value="prerelease">Latest pre-release</option>
                        <option value="version">Specific version</option>
                    </select>
                </label>
                {#if form.channel === 'version'}
                    <label
                        >Version
                        <select name="releaseVersion" bind:value={form.version} onchange={changed}>
                            <option value="">Select a version</option>
                            {#each compatible as item (item.tag_name)}
                                <option value={item.tag_name}>
                                    {item.tag_name}{item.prerelease ? ' · Pre-release' : ''}
                                </option>
                            {/each}
                        </select>
                    </label>
                {/if}
                {#if component === 'core'}
                    <label
                        >Direct HTTPS download
                        <input
                            type="url"
                            name="downloadUrl"
                            bind:value={download}
                            oninput={changed}
                        >
                    </label>
                    {#if source && !validHttpsDownload(source)}
                        <p role="alert">Enter a valid HTTPS URL.</p>
                    {/if}
                {/if}
                <p>
                    Target:
                    {source && validHttpsDownload(source)
                        ? 'Direct download'
                        : release?.tag_name || '—'}
                </p>
                {#if !source && !queryError && !releases.isPending && compatible.length === 0}
                    <p>No compatible releases for this node.</p>
                {/if}
                <label class="confirm">
                    <input
                        type="checkbox"
                        name="confirm"
                        bind:checked={confirmed}
                        disabled={!request || accepted}
                    >
                    Install {components[component]} and restart the node service
                </label>
                <button type="submit" disabled={!confirmed || !request || accepted}>
                    {installing ? 'Installing…' : 'Install now'}
                </button>
            </fieldset>
        </form>
        {#if installError}
            <p role="alert">{installError}</p>
        {/if}
        {#if accepted}
            <p role="status">
                Update accepted. The node reports the installed version after restarting.
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
