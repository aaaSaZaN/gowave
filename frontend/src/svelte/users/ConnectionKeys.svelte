<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import JsonTree from '../ui/JsonTree.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { connectionKeyLabel } from './delivery'
    import { connectionKeysQuery, rawSubscriptionQuery } from './queries'
    import SubscriptionQr from './SubscriptionQr.svelte'

    let { userId, shortUuid, close }: { userId: number; shortUuid: string; close: () => void } =
        $props()
    let t = $derived(locale.t)
    let showRaw = $state(false)
    let qr = $state<{ key: string; label: string }>()
    let error = $state('')
    let notice = $state('')
    const keys = createQuery(() => connectionKeysQuery(userId))
    const raw = createQuery(() => ({ ...rawSubscriptionQuery(shortUuid), enabled: showRaw }))
    let groups = $derived([
        { label: 'Active', tone: 'teal', icon: 'TbWifi', links: keys.data?.enabledKeys ?? [] },
        { label: 'Hidden', tone: 'gray', icon: 'TbEyeOff', links: keys.data?.hiddenKeys ?? [] },
        { label: 'Disabled', tone: 'red', icon: 'TbWifiOff', links: keys.data?.disabledKeys ?? [] }
    ] as const)

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

<Overlay
    drawer
    width={440}
    title={t('get-user-subscription-links.feature.connection-keys')}
    icon="PiLinkBreak"
    tone="teal"
    {close}
>
    <div class="content">
        <Button block variant="soft" color="teal" icon="TbJson" onclick={() => (showRaw = true)}
            >Raw Subscription</Button
        >
        {#if error}
            <p role="alert">{error}</p>
        {/if}
        {#if notice}
            <p role="status">{notice}</p>
        {/if}
        {#if keys.isPending}
            <p role="status">{t('common.message.loading')}</p>
        {:else if keys.isError}
            <p role="alert">{keys.error.message}</p>
            <Button variant="default" onclick={() => void keys.refetch()}
                >{t('common.action.try-again')}</Button
            >
        {:else if groups.every(({ links }) => links.length === 0)}
            <EmptyState icon="PiEmpty" label={t('common.message.no-data-available')} />
        {:else}
            {#each groups as group (group.label)}
                {#if group.links.length}
                    <FormCard title={group.label} icon={group.icon} tone={group.tone}>
                        {#snippet actions()}
                            <span class="count">{group.links.length}</span>
                        {/snippet}
                        {#each group.links as key}
                            {@const label = connectionKeyLabel(key, t('base-host-form.host'))}
                            <div class="key">
                                <span>{label}</span>
                                <div class="control">
                                    <input aria-label={label} type="text" readonly value={key}>
                                    <ActionIcon
                                        class="left"
                                        icon="PiCopy"
                                        label={t('common.action.copy')}
                                        size={30}
                                        iconSize={16}
                                        subtle
                                        onclick={() => copy(key)}
                                    />
                                    <ActionIcon
                                        class="right"
                                        icon="TbQrcode"
                                        label={t('view-user-modal.widget.qr-code')}
                                        size={30}
                                        iconSize={16}
                                        subtle
                                        onclick={() => (qr = { key, label })}
                                    />
                                </div>
                            </div>
                        {/each}
                    </FormCard>
                {/if}
            {/each}
        {/if}
    </div>
</Overlay>

{#if showRaw}
    <Overlay
        title="Raw Subscription"
        icon="TbJson"
        tone="teal"
        width={760}
        close={() => (showRaw = false)}
    >
        {#if raw.isPending}
            <p role="status">{t('common.message.loading')}</p>
        {:else if raw.isError}
            <p role="alert">{raw.error.message}</p>
            <Button variant="default" onclick={() => void raw.refetch()}
                >{t('common.action.try-again')}</Button
            >
        {:else}
            <div class="content">
                <Button
                    variant="default"
                    icon="PiCopy"
                    onclick={() => copy(JSON.stringify(raw.data, null, 4))}
                    >{t('common.action.copy')}</Button
                >
                {#if error}
                    <p role="alert">{error}</p>
                {/if}
                {#if notice}
                    <p role="status">{notice}</p>
                {/if}
                <JsonTree value={raw.data} indent={4} />
            </div>
        {/if}
    </Overlay>
{/if}

{#if qr}
    <SubscriptionQr
        value={qr.key}
        label={qr.label}
        filename="connection-key.png"
        close={() => (qr = undefined)}
    />
{/if}

<style>
    .content {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .key {
        display: grid;
        min-width: 0;
        gap: 4px;
        font-size: var(--mantine-font-size-sm);
    }
    .control {
        position: relative;
        display: grid;
    }
    .control input {
        width: 100%;
        padding-inline: 36px;
        font-family: var(--mantine-font-family-monospace);
    }
    .control :global(.action-icon) {
        position: absolute;
        inset-block-start: 3px;
    }
    .control :global(.left) {
        inset-inline-start: 3px;
    }
    .control :global(.right) {
        inset-inline-end: 3px;
    }
    .count {
        padding-inline: 8px;
        border-radius: var(--mantine-radius-xl);
        background: var(--mantine-color-default-hover);
        font-size: var(--mantine-font-size-xs);
    }
</style>
