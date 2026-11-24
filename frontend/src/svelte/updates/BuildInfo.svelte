<script lang="ts">
    import {
        fetchPanelUpdateStatus,
        fetchUpdateCheck,
        getUpdateErrorMessage,
        requestPanelUpdate,
        type UpdateChannel
    } from '@shared/api/panel-update'
    import { type PanelUpdateRequest, UPDATE_TIMEOUT } from '@shared/lib/panel-update-progress'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import BrandIcon from '../ui/BrandIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { type Metadata, updateCheckQuery } from './queries'
    import UpdateProgress from './UpdateProgress.svelte'

    let { metadata, close }: { metadata: Metadata; close: () => void } = $props()
    let t = $derived(locale.t)
    let current = $derived<UpdateChannel>(
        metadata.git?.backend?.branch === 'dev' ? 'dev' : 'stable'
    )
    let selected = $state<UpdateChannel>()
    let target = $derived(selected ?? current)
    let checkChannel = $derived(target === current ? undefined : target)
    const check = createQuery(() => updateCheckQuery(checkChannel))
    let request = $state<PanelUpdateRequest>()
    const recent = createQuery(() => ({
        queryKey: ['panel-update-status'],
        queryFn: ({ signal }) => fetchPanelUpdateStatus(signal),
        retry: false,
        refetchInterval: (query) => (query.state.data?.requestPending && !request ? 1500 : false),
        gcTime: 0
    }))
    let docker = $derived(check.data?.isDocker ?? false)
    let switching = $derived(!docker && target !== current)
    let busy = $state(false)
    let checking = $state(false)
    let checkError = $state('')
    let updateError = $state('')
    let copied = $state('')
    $effect(() => {
        if (!copied) return
        const timer = setTimeout(() => (copied = ''), 2000)
        return () => clearTimeout(timer)
    })
    let copyError = $state('')
    let dismissed = $state<string>()
    let status = $derived(recent.data?.status)
    let age = $derived(
        recent.data && status
            ? Date.parse(recent.data.serverTime) - Date.parse(status.updatedAt)
            : Infinity
    )
    let active = $derived.by((): PanelUpdateRequest | undefined => {
        if (request) return request
        if (status?.state !== 'running' || status.startedAt === dismissed || age >= UPDATE_TIMEOUT)
            return undefined
        return {
            channel: status.channel,
            fromVersion: status.fromVersion,
            requestedAt: status.startedAt
        }
    })
    let previousFailure = $derived(
        status?.state === 'failed' &&
            status.startedAt !== dismissed &&
            status.fromVersion === metadata.version &&
            age < 24 * 60 * 60_000
            ? status
            : undefined
    )
    let serverCommand = $derived(check.data?.canAutoUpdate ? undefined : check.data?.updateCommand)
    let release = $derived(check.data?.latestVersion ?? '')
    let commitUrl = $derived(metadata.git?.backend?.commitUrl)
    let checkedAt = $derived(check.data?.checkedAt ? new Date(check.data.checkedAt) : undefined)

    async function forceCheck(): Promise<void> {
        checking = true
        checkError = ''
        const channel = checkChannel
        const key = updateCheckQuery(channel).queryKey
        try {
            await queryClient.cancelQueries({ queryKey: key })
            const data = await fetchUpdateCheck(channel, true)
            queryClient.setQueryData(key, data)
            if (!channel || channel === data.channel) {
                queryClient.setQueryData(updateCheckQuery().queryKey, data)
            }
        } catch (error) {
            if (checkChannel === channel) {
                checkError = getUpdateErrorMessage(error, 'Could not check for updates')
            }
        } finally {
            checking = false
        }
    }

    async function applyUpdate(): Promise<void> {
        if (
            busy ||
            active ||
            recent.data?.requestPending ||
            !check.data?.canAutoUpdate ||
            !check.data.hasUpdate ||
            docker
        )
            return
        busy = true
        updateError = ''
        try {
            const response = await requestPanelUpdate(target)
            if (!response.success) throw new Error('The panel rejected the update request')
            request = {
                channel: response.channel,
                fromVersion: metadata.version,
                requestedAt: response.requestedAt
            }
        } catch (error) {
            updateError = getUpdateErrorMessage(error, 'Could not start the update')
        } finally {
            busy = false
        }
    }

    function dismiss(): void {
        dismissed = status?.startedAt ?? request?.requestedAt
        request = undefined
        void recent.refetch()
    }

    async function copy(text: string): Promise<void> {
        copied = ''
        copyError = ''
        try {
            await navigator.clipboard.writeText(text)
            copied = text
        } catch (error) {
            copyError = getUpdateErrorMessage(error, t('common.message.error'))
        }
    }
</script>

<Overlay
    class="build-info"
    title="Build Info"
    icon="Logo"
    tone="teal"
    width={440}
    fullscreenOnMobile={false}
    {close}
>
    <div class="stack">
        {#if !docker}
            <fieldset disabled={busy || !!active || recent.data?.requestPending}>
                <legend>Update channel</legend>
                <div class="segments">
                    <span
                        class="segment-indicator"
                        class:dev={target === 'dev'}
                        aria-hidden="true"
                    ></span>
                    {#each ['stable', 'dev'] as channel (channel)}
                        <label class:chosen={target === channel}>
                            <input
                                type="radio"
                                name="update-channel"
                                checked={target === channel}
                                onchange={() => {
                                    selected = channel === 'dev' ? 'dev' : 'stable'
                                    checkError = ''
                                    updateError = ''
                                }}
                            >
                            {channel === 'stable' ? 'Stable (releases)' : 'Dev (every commit)'}
                        </label>
                    {/each}
                </div>
            </fieldset>
        {/if}
        {#if switching}
            <div class="alert" class:danger={target === 'dev'}>
                <Icon name="TbAlertTriangle" size={16} />
                {target === 'dev'
                    ? 'Dev builds are made from every commit to main and may be unstable. Make a backup first.'
                    : 'The latest stable release may be older than this dev build. Database changes made by newer dev builds stay in place; make a backup first.'}
            </div>
        {/if}
        {#if active}
            <div class="update-card">
                {#key active.requestedAt}
                    <UpdateProgress request={active} {dismiss} />
                {/key}
            </div>
        {:else}
            {#if previousFailure}
                <section class="failure">
                    <strong>The last panel update failed</strong>
                    <p role="alert">{previousFailure.error || previousFailure.message}</p>
                    {#if previousFailure.log.length}
                        <pre>{previousFailure.log.map((line) => line.message).join('\n')}</pre>
                    {/if}
                    <Button size="xs" onclick={dismiss}>Dismiss</Button>
                </section>
            {/if}
            {#if recent.data?.requestPending}
                <p role="status">An update is waiting for the helper.</p>
            {/if}
            {#if updateError}
                <p role="alert">{updateError}</p>
            {/if}
            <div class="row">
                {#if checkError || check.error}
                    <span role="alert"
                        >{checkError ||
                            getUpdateErrorMessage(check.error, 'Could not check for updates')}</span
                    >
                {:else if checking || check.isPending}
                    <span role="status">Checking for updates…</span>
                {:else}
                    <span
                        >{check.data?.hasUpdate
                            ? ''
                            : `You are running the latest ${target} build. `}Checked
                        {checkedAt && !Number.isNaN(checkedAt.valueOf())
                            ? `at ${checkedAt.toLocaleTimeString()}`
                            : 'just now'}</span
                    >
                {/if}
                <Button
                    icon="TbRefresh"
                    size="compact-xs"
                    variant="subtle"
                    disabled={checking || busy || check.isPending}
                    onclick={forceCheck}
                    >Check now</Button
                >
            </div>
            {#if check.data?.hasUpdate}
                <section class="update-card">
                    <div class="row">
                        <span class="update-icon"><BrandIcon /></span>
                        <div class="update-text">
                            <strong
                                >{switching ? `Switch to ${target}` : 'Update available'}</strong
                            >
                            <div>
                                {switching ? `Installs ${release}` : `${release} is available`}
                            </div>
                        </div>
                        {#if check.data.canAutoUpdate && !docker}
                            <Button
                                size="xs"
                                variant="light"
                                color="teal"
                                icon="TbDownload"
                                disabled={busy || checking || recent.data?.requestPending}
                                onclick={applyUpdate}
                                >{switching ? 'Switch' : 'Update panel'}</Button
                            >
                        {/if}
                    </div>
                    {#if serverCommand}
                        <p>
                            {docker
                                ? 'The panel runs in Docker. Update it on the server:'
                                : 'Update the panel on the server:'}
                        </p>
                        <div class="row command">
                            <code>{serverCommand}</code>
                            <ActionIcon
                                icon={copied === serverCommand ? 'TbCheck' : 'TbCopy'}
                                label="Copy command"
                                subtle
                                size={28}
                                iconSize={14}
                                onclick={() => copy(serverCommand ?? '')}
                            />
                        </div>
                    {/if}
                </section>
            {/if}
        {/if}
        <section class="metadata">
            <div class="row">
                <div class="badges">
                    <span class="badge"><BrandIcon size={16} />{metadata.version}</span>
                    <span class="badge branch" class:dev={current === 'dev'}
                        ><Icon name="TbGitBranch" size={16} />
                        {metadata.git?.backend?.branch || 'main'}</span
                    >
                </div>
                <ActionIcon
                    icon={copied && copied !== serverCommand ? 'TbCheck' : 'TbCopy'}
                    label="Copy build info"
                    subtle
                    size={28}
                    iconSize={14}
                    onclick={() => copy(JSON.stringify(metadata, null, 2))}
                />
            </div>
            {#if metadata.git?.backend?.commitSha}
                <div class="commit">
                    <Icon name="TbGitCommit" size={14} />
                    {#if commitUrl && /^https?:\/\//i.test(commitUrl)}
                        <a href={commitUrl} target="_blank" rel="noreferrer"
                            >{metadata.git.backend.commitSha.slice(0, 7)}</a
                        >
                    {:else}
                        {metadata.git.backend.commitSha.slice(0, 7)}
                    {/if}
                </div>
            {/if}
        </section>
        <a class="github" href="https://github.com/aaaSaZaN/gowave" target="_blank" rel="noreferrer"
            ><Icon name="TbBrandGithub" size={16} />GitHub</a
        >
        {#if copyError}
            <p role="alert">{copyError}</p>
        {/if}
    </div>
</Overlay>

<style>
    :global(.build-info[open]) {
        max-width: calc(100% - 40px);
    }
    .stack {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-md);
    }
    fieldset {
        border: 0;
        margin: 0;
        padding: 0;
        min-width: 0;
    }
    legend,
    .row,
    .update-card {
        font-size: var(--mantine-font-size-xs);
        color: var(--mantine-color-dimmed);
    }
    legend {
        margin-bottom: 6px;
    }
    .segments {
        position: relative;
        display: flex;
        padding: 3px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
    }
    .segment-indicator {
        position: absolute;
        inset-block: 3px;
        inset-inline-start: 3px;
        width: calc(50% - 3px);
        border: 1px solid rgb(21 170 191 / 30%);
        border-radius: var(--mantine-radius-md);
        background: rgb(21 170 191 / 12%);
        transition: transform 180ms cubic-bezier(0.2, 0, 0, 1);
        pointer-events: none;
    }
    .segment-indicator.dev {
        transform: translateX(100%);
    }
    .segments:dir(rtl) .segment-indicator.dev {
        transform: translateX(-100%);
    }
    .segments label {
        position: relative;
        z-index: 1;
        flex: 1;
        min-inline-size: 0;
        text-align: center;
        overflow-wrap: anywhere;
        padding: 2px 4px;
        font-size: var(--mantine-font-size-xs);
        font-weight: 500;
        color: var(--mantine-color-dimmed);
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-md);
        cursor: pointer;
    }
    .segments input {
        position: absolute;
        opacity: 0;
        inset: 0;
        width: 100%;
        height: 100%;
        margin: 0;
        cursor: inherit;
    }
    .segments label:focus-within {
        outline: 2px solid var(--gw-accent);
    }
    .segments .chosen {
        color: var(--mantine-color-cyan-4);
    }
    .row,
    .badges,
    .badge,
    .commit,
    .alert,
    .github {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .row {
        justify-content: space-between;
    }
    .row :global(button) {
        flex-shrink: 0;
    }
    .badges {
        flex-wrap: wrap;
    }
    .metadata,
    .update-card,
    .alert,
    .failure {
        padding: var(--mantine-spacing-md);
        border-radius: var(--mantine-radius-md);
    }
    .metadata {
        background: linear-gradient(
            135deg,
            var(--mantine-color-dark-6),
            var(--mantine-color-dark-7)
        );
        border: 1px solid var(--mantine-color-dark-4);
    }
    .badge {
        gap: 4px;
        padding: 1px 10px;
        border-radius: var(--mantine-radius-xl);
        font-weight: 700;
        text-transform: uppercase;
        font-size: var(--mantine-font-size-sm);
        color: var(--mantine-color-cyan-light-color);
        background: var(--mantine-color-cyan-light);
    }
    .branch {
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
    }
    .dev,
    .danger,
    .failure {
        color: var(--mantine-color-red-light-color);
        background: var(--mantine-color-red-light);
    }
    .commit {
        gap: 6px;
        margin-top: var(--mantine-spacing-sm);
        font: var(--mantine-font-size-xs) var(--mantine-font-family-monospace);
    }
    .github {
        height: 36px;
        justify-content: center;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-text);
        background: var(--mantine-color-default);
        font-weight: 600;
        font-size: var(--mantine-font-size-sm);
        text-decoration: none;
    }
    .github:hover {
        background: var(--mantine-color-default-hover);
    }
    .update-card {
        background: rgb(45 212 191 / 8%);
        border: 1px solid rgb(45 212 191 / 20%);
    }
    .update-text {
        flex: 1;
        min-inline-size: 0;
        overflow-wrap: anywhere;
    }
    .update-text strong {
        color: var(--mantine-color-teal-4);
        font-size: var(--mantine-font-size-sm);
    }
    .update-icon {
        display: grid;
        padding: 10px;
        border-radius: var(--mantine-radius-md);
        background: rgb(45 212 191 / 15%);
    }
    .alert {
        align-items: flex-start;
        font-size: var(--mantine-font-size-xs);
    }
    .alert:not(.danger) {
        background: var(--mantine-color-yellow-light);
    }
    .update-card p {
        margin: var(--mantine-spacing-sm) 0 6px;
    }
    .command code {
        flex: 1;
        min-width: 0;
        overflow: auto;
        overflow-wrap: normal;
        padding: 10px;
        background: var(--mantine-color-dark-6);
        white-space: pre-wrap;
        color: var(--mantine-color-text);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
    }
    pre {
        white-space: pre-wrap;
        overflow-wrap: anywhere;
    }
</style>
