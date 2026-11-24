<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { connectionKeyLabel } from './delivery'
    import { connectionKeysQuery, rawSubscriptionQuery } from './queries'
    import SubscriptionQr from './SubscriptionQr.svelte'

    let { userId, shortUuid, close }: { userId: number; shortUuid: string; close: () => void } =
        $props()
    const title = $props.id()
    let t = $derived(locale.t)
    let dialog: HTMLDialogElement
    let showRaw = $state(false)
    let qr = $state<{ key: string; label: string }>()
    let error = $state('')
    let notice = $state('')
    const keys = createQuery(() => connectionKeysQuery(userId))
    const raw = createQuery(() => ({ ...rawSubscriptionQuery(shortUuid), enabled: showRaw }))
    let groups = $derived([
        { label: 'Active', links: keys.data?.enabledKeys ?? [] },
        { label: 'Hidden', links: keys.data?.hiddenKeys ?? [] },
        { label: 'Disabled', links: keys.data?.disabledKeys ?? [] }
    ])
    let json = $derived(showRaw && raw.data ? JSON.stringify(raw.data, null, 4) : '')

    onMount(() => dialog.showModal())

    function copy(value: string): void {
        error = ''
        notice = ''
        void navigator.clipboard.writeText(value).then(
            () => (notice = t('common.message.copied')),
            (reason: unknown) =>
                (error = reason instanceof Error ? reason.message : t('common.message.error'))
        )
    }
</script>

<dialog class="connection-keys" bind:this={dialog} aria-labelledby={title} onclose={close}>
    <div class="header">
        <h2 id={title}>{t('get-user-subscription-links.feature.connection-keys')}</h2>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
    <div class="button-row">
        <button type="button" aria-pressed={!showRaw} onclick={() => (showRaw = false)}>
            {t('get-user-subscription-links.feature.connection-keys')}
        </button>
        <button type="button" aria-pressed={showRaw} onclick={() => (showRaw = true)}>
            Raw Subscription
        </button>
    </div>
    {#if error}
        <p role="alert">{error}</p>
    {/if}
    {#if notice}
        <p role="status">{notice}</p>
    {/if}
    {#if showRaw}
        {#if raw.isPending}
            <p role="status">{t('common.message.loading')}</p>
        {:else if raw.isError}
            <p role="alert">{raw.error.message}</p>
            <button type="button" onclick={() => void raw.refetch()}>
                {t('common.action.try-again')}
            </button>
        {:else}
            <button class="copy-json" type="button" onclick={() => copy(json)}>
                {t('common.action.copy')}
            </button>
            <pre>{json}</pre>
        {/if}
    {:else if keys.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if keys.isError}
        <p role="alert">{keys.error.message}</p>
        <button type="button" onclick={() => void keys.refetch()}>
            {t('common.action.try-again')}
        </button>
    {:else if groups.every(({ links }) => links.length === 0)}
        <p>{t('common.message.no-data-available')}</p>
    {:else}
        {#each groups as group (group.label)}
            {#if group.links.length}
                <section aria-label={group.label}>
                    <h3>{group.label} ({group.links.length.toLocaleString(locale.language)})</h3>
                    {#each group.links as key}
                        {@const label = connectionKeyLabel(key, t('base-host-form.host'))}
                        <div class="key">
                            <label>{label}<input type="text" readonly value={key}></label>
                            <div class="button-row">
                                <button type="button" onclick={() => copy(key)}>
                                    {t('common.action.copy')}
                                </button>
                                <button type="button" onclick={() => (qr = { key, label })}>
                                    {t('view-user-modal.widget.qr-code')}
                                </button>
                            </div>
                        </div>
                    {/each}
                </section>
            {/if}
        {/each}
    {/if}
    {#if qr}
        <SubscriptionQr
            value={qr.key}
            label={qr.label}
            filename="connection-key.png"
            close={() => (qr = undefined)}
        />
    {/if}
</dialog>

<style>
    .connection-keys {
        width: min(50rem, calc(100vw - 2rem));
    }
    .header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.75rem;
        margin-block-end: 0.75rem;
    }
    h2 {
        margin: 0;
    }
    h3,
    .key,
    .copy-json {
        margin-block: 0.75rem;
    }
    .key {
        display: flex;
        flex-wrap: wrap;
        align-items: end;
        gap: 0.5rem;
    }
    label {
        display: grid;
        flex: 1 1 15rem;
        min-width: 0;
        gap: 0.25rem;
        overflow-wrap: anywhere;
    }
    input {
        min-width: 0;
    }
    pre {
        max-width: 100%;
        overflow: auto;
        white-space: pre-wrap;
        overflow-wrap: anywhere;
    }
</style>
