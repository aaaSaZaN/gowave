<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { untrack } from 'svelte'
    import { profilesQuery } from '../config-profiles/queries'
    import HostInboundPicker from '../hosts/HostInboundPicker.svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { Node } from './queries'

    let {
        current,
        onchange
    }: { onchange?: (dirty: boolean) => void; current: Node['configProfile'] | undefined } =
        $props()
    const profiles = createQuery(() => ({ ...profilesQuery, refetchOnMount: 'always' }))
    const original = untrack(() => current)
    let profile = $state(original?.activeConfigProfileUuid ?? '')
    const originalInbounds = new Set(original?.activeInbounds.map(({ uuid }) => uuid))
    let selected = $state([...originalInbounds])
    let t = $derived(locale.t)
    let choices = $derived(profiles.data?.configProfiles ?? [])
    let open = $state(false)
    let active = $derived(choices.find((item) => item.uuid === profile))
    let ports = $derived([
        ...new Set(
            active?.inbounds
                .filter((item) => selected.includes(item.uuid))
                .map((item) => item.port)
                .filter(Boolean)
        )
    ])
</script>
<input type="hidden" name="profile" value={profile}>
{#each selected as uuid (uuid)}
    <input type="hidden" name="inbounds" value={uuid}>
{/each}
{#if profiles.error}
    <p role="alert">{profiles.error.message}</p>
{:else if profiles.isPending}
    <p role="status">{t('common.message.loading')}</p>
{:else}
    <div class="summary">
        <div class="line">
            <span class:active={!!active} class="logo"><Icon name="XrayLogo" size={20} /></span>
            {#if active}
                <strong>{active.name}</strong
                ><span class="count"><Icon name="TbTag" size={12} />{selected.length}</span>
                <ActionIcon
                    icon="TbEdit"
                    label={t('common.action.edit')}
                    size={34}
                    onclick={() => (open = true)}
                />
            {:else}
                <div>
                    {t(
                        'show-config-profiles-with-inbounds.feature.no-config-profile-selected'
                    )}<small
                        >{t(
                            'show-config-profiles-with-inbounds.feature.choose-a-profile-to-configure-inbounds-for-this-node'
                        )}</small
                    >
                </div>
            {/if}
        </div>
        {#if active}
            {#if ports.length}
                <div class="ports">
                    {#each ports as port (port)}
                        <b>{port}</b>
                    {/each}
                </div>
            {/if}
        {:else}
            <Button
                size="sm"
                variant="light"
                color="cyan"
                icon="TbFilePlus"
                block
                onclick={() => (open = true)}
                >{t('common.action.select')}</Button
            >
        {/if}
    </div>
{/if}
{#if open}
    <HostInboundPicker
        profiles={choices}
        current={profile}
        currentInbounds={selected}
        close={() => (open = false)}
        onselectMany={(choice, ids) => {
            profile = choice.uuid
            selected = ids
            open = false
            onchange?.(
                profile !== (original?.activeConfigProfileUuid ?? '') ||
                    selected.length !== originalInbounds.size ||
                    selected.some((uuid) => !originalInbounds.has(uuid))
            )
        }}
    />
{/if}
<style>
    .summary {
        display: grid;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        font-size: var(--mantine-font-size-sm);
    }
    .line,
    .logo,
    .count,
    .ports {
        display: flex;
        align-items: center;
        gap: 6px;
    }
    .line {
        gap: 12px;
    }
    .logo,
    .ports b {
        border: 1px solid var(--mantine-color-default-border);
        border-radius: 6px;
        padding: 6px;
    }
    .logo.active,
    .count {
        background: var(--mantine-color-cyan-light);
        color: var(--mantine-color-cyan-light-color);
    }
    .count {
        flex: none;
        height: calc(1.625rem * var(--mantine-scale));
        margin-inline-start: auto;
        padding-inline: 12px;
        border-radius: var(--mantine-radius-md);
        font-size: calc(0.8125rem * var(--mantine-scale));
        font-weight: 700;
    }
    small {
        display: block;
        color: var(--mantine-color-dimmed);
    }
    strong {
        min-width: 0;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        font-weight: 600;
        font-family: var(--mantine-font-family-monospace);
    }
    .ports b {
        display: flex;
        align-items: center;
        height: calc(1.125rem * var(--mantine-scale));
        padding: 0 8px;
        font-size: calc(0.625rem * var(--mantine-scale));
    }
</style>
