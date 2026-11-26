<script lang="ts">
    import {
        fetchPanelUpdateStatus,
        getUpdateErrorMessage,
        type IPanelUpdateLogLine
    } from '@shared/api/panel-update'
    import {
        belongsToRequest,
        deriveUpdateView,
        HELPER_COMMAND,
        isFinished,
        JOURNAL_COMMAND,
        type PanelUpdateRequest,
        POLL_INTERVAL,
        type PollState,
        RELOAD_DELAY,
        STEP_LABELS
    } from '@shared/lib/panel-update-progress'
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'

    let { request, dismiss }: { request: PanelUpdateRequest; dismiss: () => void } = $props()
    const started = Date.now()
    let poll = $state.raw<PollState>({
        lastError: null,
        now: started,
        response: null,
        unreachableSince: null
    })
    let view = $derived(deriveUpdateView(request, poll, started))
    let own = $derived(
        belongsToRequest(poll.response?.status, request) ? poll.response?.status : undefined
    )
    let t = $derived(locale.t)
    let title = $derived(
        view.kind === 'running'
            ? (STEP_LABELS[view.status.step] ?? view.status.step)
            : {
                  done: 'Update complete',
                  failed: 'Update failed',
                  helperIdle: 'Update helper has not started',
                  legacy: 'Update in progress',
                  restarting: 'Restarting panel',
                  waiting: 'Updating the panel'
              }[view.kind]
    )
    let description = $derived(
        view.kind === 'running'
            ? view.status.message
            : {
                  done:
                      view.kind === 'done' && view.version
                          ? `Updated to ${view.version}. Reloading the page…`
                          : 'Update finished. Reloading the page…',
                  failed: view.kind === 'failed' ? view.error : '',
                  helperIdle:
                      'The request is still waiting. Check that the update helper is active.',
                  legacy: 'The installed helper predates progress reporting. Waiting for the panel to restart.',
                  restarting: 'The panel will reconnect automatically.',
                  waiting: 'Waiting for the update helper to start…'
              }[view.kind]
    )
    let command = $derived(
        view.kind === 'failed'
            ? view.command
            : view.kind === 'helperIdle'
              ? HELPER_COMMAND
              : view.kind === 'legacy'
                ? JOURNAL_COMMAND
                : undefined
    )
    let copyError = $state('')

    onMount(() => {
        const controller = new AbortController()
        let timer: ReturnType<typeof setTimeout> | undefined
        async function tick(): Promise<void> {
            try {
                const response = await fetchPanelUpdateStatus(controller.signal)
                if (controller.signal.aborted) return
                poll = { lastError: null, now: Date.now(), response, unreachableSince: null }
            } catch (error) {
                if (controller.signal.aborted) return
                const now = Date.now()
                poll = {
                    ...poll,
                    now,
                    lastError: getUpdateErrorMessage(error, 'Status request failed'),
                    unreachableSince: poll.unreachableSince ?? now
                }
            }
            const next = deriveUpdateView(request, poll, started)
            if (next.kind === 'done') timer = setTimeout(() => location.reload(), RELOAD_DELAY)
            else if (!isFinished(next)) timer = setTimeout(() => void tick(), POLL_INTERVAL)
        }
        void tick()
        return () => {
            controller.abort()
            clearTimeout(timer)
        }
    })

    function logText(log: IPanelUpdateLogLine[]): string {
        return log
            .map((line) => {
                const at = new Date(line.time)
                const time = Number.isNaN(at.getTime())
                    ? ''
                    : at.toLocaleTimeString(locale.language)
                return `${time} ${line.message}`.trim()
            })
            .join('\n')
    }

    async function copy(value: string): Promise<void> {
        copyError = ''
        try {
            await navigator.clipboard.writeText(value)
            notifications.show({
                title: t('common.message.copied'),
                message: value,
                color: 'teal'
            })
        } catch (failure) {
            copyError = getUpdateErrorMessage(failure, t('common.message.error'))
        }
    }
</script>

<section aria-label={title}>
    <header class:finished={isFinished(view)} class:failed={view.kind === 'failed'}>
        {#if isFinished(view)}
            <Icon name={view.kind === 'done' ? 'TbCheck' : 'TbAlertTriangle'} size={16} />
        {:else}
            <span class="spinner"></span>
        {/if}
        <h3>{title}</h3>
        {#if !isFinished(view)}
            <span>{own?.progress ?? 0}%</span>
        {/if}
    </header>
    {#if !isFinished(view)}
        <progress value={own?.progress ?? 0} max="100" aria-label={title}></progress>
    {/if}
    <p role={view.kind === 'failed' ? 'alert' : 'status'}>{description}</p>
    {#if !isFinished(view)}
        {#if own?.file && own.total}
            <p>
                {own.file}
                · {prettifyBytesUtil(own.downloaded ?? 0, true)} /
                {prettifyBytesUtil(own.total, true)}
            </p>
        {/if}
    {/if}
    {#if own?.log.length}
        <pre role="log" aria-label="Update log">{logText(own.log)}</pre>
    {/if}
    {#if command}
        <pre>{command}</pre>
        <Button onclick={() => void copy(command)}>{t('common.action.copy')}</Button>
    {/if}
    {#if copyError}
        <p role="alert">{copyError}</p>
    {/if}
    {#if view.kind === 'done'}
        <Button size="xs" variant="light" icon="TbRefresh" onclick={() => location.reload()}
            >Reload now</Button
        >
    {:else if view.kind === 'failed'}
        <Button size="xs" onclick={dismiss}>Close</Button>
    {/if}
</section>

<style>
    section {
        display: grid;
        gap: var(--mantine-spacing-sm);
    }
    header {
        display: flex;
        gap: var(--mantine-spacing-sm);
        align-items: center;
    }
    header span:last-child {
        margin-inline-start: auto;
    }
    .finished {
        color: var(--mantine-color-teal-light-color);
    }
    .failed {
        color: var(--mantine-color-error);
    }
    .spinner {
        width: 16px;
        height: 16px;
        border: 2px solid var(--mantine-color-cyan-light);
        border-top-color: var(--mantine-color-cyan-5);
        border-radius: 50%;
        animation: spin 800ms linear infinite;
    }
    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
    section p {
        margin: 0;
        overflow-wrap: anywhere;
    }
    h3 {
        font-size: var(--mantine-font-size-sm);
        margin-block: 0;
    }
    progress {
        width: 100%;
        height: 8px;
        accent-color: var(--mantine-color-cyan-filled);
        vertical-align: middle;
        margin-inline-end: 0.5rem;
    }
    pre {
        margin: 0;
        padding: var(--mantine-spacing-xs);
        background: var(--mantine-color-dark-8);
        border-radius: var(--mantine-radius-md);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
        max-height: 12rem;
        overflow: auto;
    }
</style>
