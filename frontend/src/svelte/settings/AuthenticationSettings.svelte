<script lang="ts">
    import type { UpdateRemnawaveSettingsCommand } from '@remnawave/backend-contract'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import HelpDrawer, { type HelpScreen } from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import { notifications } from '../ui/notification-state.svelte'
    import Switch from '../ui/Switch.svelte'
    import { oauthField, providers, readAuthentication } from './auth-fields'
    import Passkeys from './Passkeys.svelte'
    import type { Settings } from './queries'
    import SettingsCard from './SettingsCard.svelte'

    let {
        settings,
        busy,
        save
    }: {
        settings: Settings
        busy: boolean
        save: (values: UpdateRemnawaveSettingsCommand.RequestBody) => Promise<void>
    } = $props()

    let t = $derived(locale.t)
    let manageKeys = $state(false)
    let help = $state<HelpScreen>()
    let expanded = $state<Record<string, boolean>>({})
    const logos = {
        telegram: { icon: 'BiLogoTelegram', background: '#0088cc', help: 'AUTH_METHODS_TELEGRAM' },
        github: { icon: 'BiLogoGithub', background: '#1b1f24', help: 'AUTH_METHODS_GITHUB' },
        pocketid: { icon: 'PocketIdLogo', background: '#000', help: 'AUTH_METHODS_POCKETID' },
        keycloak: { icon: 'SiKeycloak', background: '#000', help: 'AUTH_METHODS_KEYCLOAK' },
        generic: { icon: 'TbKey', background: '#000', help: 'AUTH_METHODS_GENERIC' }
    } as const
    let error = $state('')
    let labels = $derived({
        clientId: t('auth-settings.fields.clientId.label'),
        clientSecret: t('auth-settings.fields.clientSecret.label'),
        plainDomain: t('auth-settings.fields.plainDomain.label'),
        allowedEmails: t('auth-settings.fields.allowedEmails.label'),
        realm: t('auth-settings.fields.realm.label'),
        frontendDomain: t('auth-settings.passkey.rpId.label'),
        keycloakDomain: t('auth-settings.fields.keycloakDomain.label'),
        allowedIds: t('auth-settings.telegram.adminIds.label'),
        authorizationUrl: 'Authorization URL',
        tokenUrl: 'Token URL',
        withPkce: 'With PKCE'
    })

    async function submit(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (busy || !(event.currentTarget instanceof HTMLFormElement)) return
        error = ''
        try {
            await save(readAuthentication(new FormData(event.currentTarget)))
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        } catch (reason) {
            error = reason instanceof Error ? reason.message : String(reason)
        }
    }
</script>

{#snippet method(
    title: string,
    icon: IconName,
    key: string,
    background: string,
    foreground: string,
    enabled: boolean,
    article?: HelpScreen
)}
    <div class="method">
        {#if key === 'password'}
            <span class="method-summary">
                <span class="method-icon" style:background style:color={foreground}
                    ><Icon name={icon} size={24} /></span
                >
                <span class="method-label">{title}</span>
            </span>
        {:else}
            <button
                class="method-summary mantine-UnstyledButton-root"
                type="button"
                aria-expanded={Boolean(expanded[key])}
                onclick={() => (expanded[key] = !expanded[key])}
            >
                <span class="method-icon" style:background style:color={foreground}
                    ><Icon name={icon} size={24} /></span
                >
                <span class="method-label">{title}</span>
                <span class="method-chevron" class:rotated={expanded[key]}
                    ><Icon name="TbChevronDown" size={16} /></span
                >
            </button>
        {/if}
        <div class="method-controls">
            {#if article}
                <ActionIcon
                    icon="TbQuestionMark"
                    tone="lime"
                    label={t('help-action-icon.shared.help-article')}
                    size={30}
                    iconSize={20}
                    onclick={() => (help = article)}
                />
            {/if}
            <Switch
                label={title}
                name={key === 'password' || key === 'passkey' ? key : `${key}.enabled`}
                checked={enabled}
            />
        </div>
    </div>
{/snippet}

<form onsubmit={(event) => void submit(event)}>
    <SettingsCard
        title={t('auth-settings.header.title')}
        description={t('auth-settings.header.description')}
        icon="PiKey"
    >
        {#snippet footer()}
            <div class="save">
                <Button type="submit" variant="soft" color="teal" size="md" disabled={busy}
                    >{t('common.action.save')}</Button
                >
            </div>
        {/snippet}
        <fieldset disabled={busy}>
            <div class="method-panel">
                {@render method(
                    t('auth-settings.password.title'),
                    'TbPassword',
                    'password',
                    'var(--mantine-color-orange-light)',
                    'var(--mantine-color-orange-light-color)',
                    Boolean(settings.passwordSettings?.enabled),
                    'AUTH_METHODS_PASSWORD'
                )}
            </div>
            <div class="method-panel">
                {@render method(
                    t('auth-settings.passkey.title'),
                    'TbFingerprint',
                    'passkey',
                    'var(--mantine-color-cyan-light)',
                    'var(--mantine-color-cyan-light-color)',
                    Boolean(settings.passkeySettings?.enabled)
                )}
                <div class="fields" hidden={!expanded['passkey']}>
                    <label>
                        {t('auth-settings.passkey.rpId.label')}
                        <input
                            name="rpId"
                            value={settings.passkeySettings?.rpId ?? ''}
                            placeholder="example.com"
                        >
                        <small>{t('auth-settings.passkey.rpId.description')}</small>
                    </label>
                    <label>
                        {t('auth-settings.passkey.origin.label')}
                        <input
                            name="origin"
                            value={settings.passkeySettings?.origin ?? ''}
                            placeholder="https://example.com"
                        >
                        <small>{t('auth-settings.passkey.origin.description')}</small>
                    </label>
                    <button type="button" onclick={() => (manageKeys = true)}>
                        {t('auth-settings.passkey.manage-button')}
                        {t('passkeys-drawer.component.passkeys')}
                    </button>
                </div>
            </div>
            {#if settings.oauth2Settings}
                {#each providers as provider (provider.key)}
                    {@const config = settings.oauth2Settings[provider.key]}
                    {@const logo = logos[provider.key]}
                    <div class="method-panel">
                        {@render method(
                            provider.title,
                            logo.icon,
                            provider.key,
                            logo.background,
                            '#fff',
                            config.enabled,
                            logo.help
                        )}
                        <div class="fields" hidden={!expanded[provider.key]}>
                            {#each provider.fields as field (field)}
                                {@const name = `${provider.key}.${field}`}
                                {#if field === 'withPkce'}
                                    <label class="toggle">
                                        <input
                                            type="checkbox"
                                            {name}
                                            checked={'withPkce' in config && config.withPkce}
                                        >
                                        {labels[field]}
                                    </label>
                                {:else}
                                    <label>
                                        {labels[field]}
                                        <input
                                            type={field === 'clientSecret' ? 'password' : 'text'}
                                            {name}
                                            value={oauthField(config, field)}
                                            autocomplete={field === 'clientSecret'
                                                ? 'new-password'
                                                : 'off'}
                                        >
                                    </label>
                                {/if}
                            {/each}
                        </div>
                    </div>
                {/each}
            {/if}
        </fieldset>
        {#if error}
            <p role="alert">{error}</p>
        {/if}
    </SettingsCard>
</form>

{#if help}
    <HelpDrawer screen={help} close={() => (help = undefined)} />
{/if}

{#if manageKeys}
    <Passkeys close={() => (manageKeys = false)} />
{/if}

<style>
    fieldset {
        display: grid;
        gap: var(--mantine-spacing-md);
        border: 0;
        min-width: 0;
        margin: 0;
        padding: 0;
    }
    .method-panel {
        background: light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6));
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-md);
    }
    .method,
    .method-summary,
    .method-controls {
        display: flex;
        align-items: center;
    }
    .method-summary {
        flex: 1;
        min-width: 0;
        gap: var(--mantine-spacing-sm);
        padding-inline: var(--mantine-spacing-md);
        color: var(--mantine-color-bright);
        font-weight: 500;
        font-size: var(--mantine-font-size-md);
        line-height: var(--mantine-line-height);
        text-align: start;
    }
    .method-label {
        padding-block: var(--mantine-spacing-sm);
    }
    .method-icon {
        display: grid;
        place-items: center;
        width: 34px;
        height: 34px;
        border-radius: var(--mantine-radius-md);
        flex: none;
    }
    .method-chevron {
        margin-inline-start: auto;
        display: grid;
        place-items: center;
        transition: transform 150ms ease;
    }
    .method-chevron.rotated {
        transform: rotate(180deg);
    }
    .method-controls {
        padding-inline-end: var(--mantine-spacing-xs);
        gap: var(--mantine-spacing-xs);
    }
    .fields,
    .fields label {
        display: grid;
        gap: 0.25rem;
    }
    .fields {
        padding: 0 var(--mantine-spacing-md) var(--mantine-spacing-md);
        gap: var(--mantine-spacing-sm);
    }
    .fields[hidden] {
        display: none;
    }
    .fields button {
        justify-self: start;
    }
    .fields .toggle {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .save {
        display: flex;
        justify-content: end;
    }
    small {
        color: var(--mantine-color-dimmed);
    }
</style>
