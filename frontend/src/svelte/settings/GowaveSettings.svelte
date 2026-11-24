<script lang="ts">
    import { UpdateRemnawaveSettingsCommand } from '@remnawave/backend-contract'
    import { createQuery } from '@tanstack/svelte-query'
    import { MediaQuery } from 'svelte/reactivity'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { layout } from '../layout.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import Icon from '../ui/Icon.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import ApiTokens from './ApiTokens.svelte'
    import AuthenticationSettings from './AuthenticationSettings.svelte'
    import BrandingSettings from './BrandingSettings.svelte'
    import { panelSettings } from './queries'
    import SettingsCard from './SettingsCard.svelte'

    const settings = createQuery(() => panelSettings)
    const desktop = new MediaQuery('(width > 64rem)')
    let t = $derived(locale.t)
    let busy = $state(false)

    async function save(values: UpdateRemnawaveSettingsCommand.RequestBody): Promise<void> {
        busy = true
        try {
            const updated = await submitCommand(UpdateRemnawaveSettingsCommand, values)
            await queryClient.cancelQueries({ queryKey: panelSettings.queryKey })
            queryClient.setQueryData(panelSettings.queryKey, updated)
            await queryClient.invalidateQueries({ queryKey: ['auth', 'getAuthStatus'] })
        } finally {
            busy = false
        }
    }
</script>

<svelte:head><title>{t('constants.gowave-settings')} | Gowave</title></svelte:head>
<main class="settings-page">
    {#if settings.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.gowave-settings')} icon="Logo" />

        {#if settings.error}
            <p role="alert">{settings.error.message}</p>
            <button type="button" onclick={() => void settings.refetch()}>
                {t('common.action.try-again')}
            </button>
        {/if}
        <div class="settings">
            <div class="settings-left">
                {#if settings.data}
                    <AuthenticationSettings settings={settings.data} {busy} {save} />
                {/if}
                {#if desktop.current}
                    <div class="experimental">
                        <SettingsCard
                            title={t('experimental-settings-card.widget.experimental-features')}
                            description={t('experimental-settings-card.widget.description')}
                            icon="TbFlask"
                            tone="grape"
                        >
                            <label class="layout-toggle" class:enabled={layout.legacy}>
                                <span class="layout-icon"
                                    ><Icon name="TbLayoutSidebar" size={20} /></span
                                >
                                <span
                                    >{t(
                                        'experimental-settings-card.widget.legacy-layout-style'
                                    )}</span
                                >
                                <input
                                    type="checkbox"
                                    checked={layout.legacy}
                                    onchange={(event) =>
                                        layout.setLegacy(event.currentTarget.checked)}
                                >
                            </label>
                        </SettingsCard>
                    </div>
                {/if}
            </div>
            <div class="settings-right">
                <ApiTokens />
                {#if settings.data}
                    <BrandingSettings branding={settings.data.brandingSettings} {busy} {save} />
                {/if}
            </div>
        </div>
    {/if}
</main>

<style>
    .settings-page,
    .settings,
    .settings-left,
    .settings-right {
        min-width: 0;
        display: grid;
        gap: var(--mantine-spacing-md);
        align-content: start;
    }
    .settings-left,
    .settings-right {
        display: contents;
    }
    .settings {
        padding-block-end: calc(var(--mantine-spacing-md) + 2px);
    }
    .experimental {
        order: 4;
        min-width: 0;
    }
    .layout-toggle {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: 10px 12px;
        min-height: 56px;
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-bright);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        line-height: 1.2;
        cursor: pointer;
    }
    .layout-toggle.enabled {
        border-color: var(--mantine-primary-color-filled);
        background-image: linear-gradient(135deg, rgb(34 184 207 / 8%), transparent 60%);
    }
    .layout-toggle input {
        margin-inline-start: auto;
        width: 20px;
        height: 20px;
        accent-color: var(--mantine-primary-color-filled);
    }
    .layout-icon {
        display: grid;
        place-items: center;
        width: 34px;
        height: 34px;
        flex: none;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-cyan-4);
        border: 1px solid rgb(21 170 191 / 30%);
        background: linear-gradient(135deg, rgb(21 170 191 / 15%), rgb(16 152 173 / 10%));
    }
    @media (width < 48em) {
        .experimental {
            display: none;
        }
    }
    @media (1400px <= width < 2000px) {
        .settings {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
        .settings-left,
        .settings-right {
            display: grid;
        }
    }
    @media (width >= 2000px) {
        .settings {
            grid-template-columns: repeat(3, minmax(0, 1fr));
            align-items: start;
        }
    }
    @media (width >= 3000px) {
        .settings {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
</style>
