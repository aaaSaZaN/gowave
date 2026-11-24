<script lang="ts">
    import { panelUrl } from '@shared/lib/panel-path'
    import { parseColoredTextUtil } from '@shared/utils/misc/parse-colored-text'
    import { createMutation, createQuery, keepPreviousData } from '@tanstack/svelte-query'
    import { ZodError } from 'zod'
    import { locale } from '../i18n.svelte'
    import { returnToDashboard } from '../router'
    import { session } from '../session.svelte'
    import Button from '../ui/Button.svelte'
    import type { IconName } from '../ui/icons'
    import { notifications } from '../ui/notification-state.svelte'
    import { authenticatePasskey, authorizeOAuth, readAuthStatus, submitCredentials } from './api'
    import CredentialsField from './CredentialsField.svelte'

    const status = createQuery(() => ({
        queryKey: ['auth', 'getAuthStatus'],
        queryFn: ({ signal }) => readAuthStatus(signal),
        refetchOnMount: false,
        placeholderData: keepPreviousData
    }))

    const submit = createMutation(() => ({
        mutationFn: (fields: Record<string, FormDataEntryValue>) =>
            submitCredentials(fields, isRegister),
        onSuccess: (result) => {
            if (isRegister)
                notifications.show({
                    title: 'Register',
                    message: 'User registered successfully',
                    color: 'teal'
                })
            signIn(result)
        },
        onError: (error) => report(isRegister ? 'Register' : 'Login', error)
    }))

    const passkey = createMutation(() => ({
        mutationFn: authenticatePasskey,
        onSuccess: (result) => {
            notifications.show({
                title: 'Passkey Verified',
                message: 'Passkey authenticated successfully',
                color: 'teal'
            })
            signIn(result)
        },
        onError: (error) => report('Passkey', error)
    }))

    const authorize = createMutation(() => ({
        mutationFn: authorizeOAuth,
        onSuccess: ({ authorizationUrl }) => {
            if (authorizationUrl) window.location.assign(authorizationUrl)
        },
        onError: (error) => report('OAuth2 Authorize', error)
    }))

    let isRegister = $derived(!status.data?.isLoginAllowed && !!status.data?.isRegisterAllowed)
    let authentication = $derived(status.data?.authentication)
    const providerDetails: {
        key: 'telegram' | 'pocketid' | 'github' | 'keycloak' | 'generic'
        label: string
        color: string
        icon: IconName
    }[] = [
        { key: 'telegram', label: 'Telegram', color: '#0088cc', icon: 'BiLogoTelegram' },
        { key: 'pocketid', label: 'PocketID', color: 'dark', icon: 'PocketIdLogo' },
        { key: 'github', label: 'GitHub', color: '#24292e', icon: 'BiLogoGithub' },
        { key: 'keycloak', label: 'Keycloak', color: '#000000', icon: 'SiKeycloak' },
        { key: 'generic', label: 'OAuth2', color: '#000000', icon: 'TbKey' }
    ]
    let providers = $derived(
        providerDetails.filter(({ key }) => authentication?.oauth2.providers[key])
    )
    let customLogo = $derived(status.data?.branding.logoUrl)
    let logo = $derived(customLogo || panelUrl(`favicons/gowave.svg?v=${__BUILD_ID__}`))
    let titleParts = $derived(
        parseColoredTextUtil(status.data?.branding.title || '{cyan}Go{white}wave')
    )
    let busy = $derived(submit.isPending || passkey.isPending || authorize.isPending)
    let username = $state('')
    let password = $state('')
    let confirmPassword = $state('')
    let validationError = $state('')
    let t = $derived(locale.t)

    function signIn(result: { accessToken: string }): void {
        session.setToken(result.accessToken)
        returnToDashboard()
    }

    function report(title: string, error: Error): void {
        const message =
            error instanceof ZodError ? (error.issues[0]?.message ?? error.message) : error.message
        notifications.show({ title, message, color: 'red' })
    }

    function resetErrors(): void {
        validationError = ''
        submit.reset()
        passkey.reset()
        authorize.reset()
    }

    function handleSubmit(event: SubmitEvent): void {
        event.preventDefault()
        if (!(event.currentTarget instanceof HTMLFormElement) || busy) return
        resetErrors()
        // Browser password managers can fill fields without dispatching input events.
        const fields = Object.fromEntries(new FormData(event.currentTarget))
        if (isRegister && fields['password'] !== fields['confirmPassword']) {
            validationError = t('register-form.feature.passwords-do-not-match')
            return
        }
        submit.mutate(fields)
    }

    function generatePassword(): void {
        const alphabet = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
        const bytes = crypto.getRandomValues(new Uint8Array(32))
        password = Array.from(bytes, (byte) => alphabet.charAt(byte % alphabet.length)).join('')
        confirmPassword = password
        void Promise.resolve()
            .then(() => navigator.clipboard.writeText(password))
            .then(
                () => {
                    notifications.show({
                        title: t('register-form.feature.password-copied'),
                        message: t('register-form.feature.password-copied-message'),
                        color: 'teal'
                    })
                },
                () => {
                    notifications.show({
                        title: t('common.message.error'),
                        message: t('register-form.feature.password-copied-error'),
                        color: 'red'
                    })
                }
            )
    }
</script>

<svelte:head><title>Login | Gowave</title></svelte:head>

<main class="auth-layout">
    <div class="auth-width">
        <section class="auth-stack" aria-labelledby="login-heading">
            <div class="auth-brand">
                <img src={logo} class:custom-logo={customLogo} alt={customLogo ? 'logo' : 'Gowave'}>
                <h1 id="login-heading">
                    {#each titleParts as part}
                        <span
                            style:color={part.color === 'white' || !part.color
                                ? 'var(--mantine-color-bright)'
                                : `var(--mantine-color-${part.color}-text, ${part.color})`}
                            >{part.text}</span
                        >
                    {/each}
                </h1>
            </div>
            {#if status.isPending}
                <p role="status">{t('common.message.loading')}</p>
            {:else if status.isError}
                <div class="auth-error">
                    <p role="alert">Server is not responding. Check logs.</p>
                    <Button onclick={() => void status.refetch()}
                        >{t('common.action.try-again')}</Button
                    >
                </div>
            {:else}
                <div class="auth-box" class:registration={isRegister}>
                    {#if isRegister || authentication?.password.enabled}
                        <form onsubmit={handleSubmit} class="auth-form">
                            <div
                                class="m_7485cace mantine-Container-root auth-container"
                                data-strategy="block"
                            >
                                <div class="m_1b7284a3 mantine-Paper-root auth-paper">
                                    {#if isRegister}
                                        <h2>{t('register-form.feature.registration')}</h2>
                                        <p class="register-description">
                                            {t('register-form.feature.register-description')}
                                        </p>
                                    {/if}
                                    <CredentialsField
                                        name="username"
                                        autocomplete="username"
                                        bind:value={username}
                                        label={t('common.field.username')}
                                        placeholder={isRegister
                                            ? 'IamSuperAdmin'
                                            : t('common.field.username')}
                                        fieldSize={isRegister ? 'md' : 'sm'}
                                    />
                                    <div class="auth-password-fields">
                                        <CredentialsField
                                            name="password"
                                            autocomplete={isRegister
                                                ? 'new-password'
                                                : 'current-password'}
                                            bind:value={password}
                                            label={t('common.field.password')}
                                            placeholder={isRegister
                                                ? 'soy_t5Px5`Gm4j0@Hf&Dd7iU'
                                                : t('login-form.feature.your-password')}
                                            fieldSize={isRegister ? 'md' : 'sm'}
                                            minlength={isRegister ? 24 : undefined}
                                            secret
                                        />
                                        {#if isRegister}
                                            <CredentialsField
                                                name="confirmPassword"
                                                autocomplete="new-password"
                                                bind:value={confirmPassword}
                                                label={t('register-form.feature.confirm-password')}
                                                placeholder="soy_t5Px5\`Gm4j0@Hf&Dd7iU"
                                                fieldSize="md"
                                                secret
                                                error={validationError || undefined}
                                            />
                                            <Button
                                                icon="PiShuffleDuotone"
                                                block
                                                size="md"
                                                variant="filled"
                                                onclick={generatePassword}
                                                disabled={busy}
                                            >
                                                {t('register-form.feature.generate')}
                                            </Button>
                                        {/if}
                                    </div>
                                    <Button
                                        class="auth-submit"
                                        type="submit"
                                        disabled={busy}
                                        block
                                        icon={isRegister ? 'PiSignpostDuotone' : 'PiSignInDuotone'}
                                        size={isRegister ? 'md' : 'sm'}
                                    >
                                        {isRegister
                                            ? t('register-form.feature.sign-up')
                                            : t('login-form.feature.sign-in')}
                                    </Button>
                                </div>
                            </div>
                        </form>
                    {/if}
                    {#if !isRegister && (authentication?.passkey.enabled || providers.length > 0)}
                        {#if authentication?.password.enabled}
                            <div class="auth-divider">OR</div>
                        {/if}
                        <div class="auth-providers" class:wide={authentication?.password.enabled}>
                            {#if authentication?.passkey.enabled}
                                <Button
                                    variant="filled"
                                    color="dark"
                                    icon="TbFingerprint"
                                    iconSize={20}
                                    disabled={busy}
                                    onclick={() => {
                                        resetErrors()
                                        passkey.mutate()
                                    }}
                                >
                                    Passkey
                                </Button>
                            {/if}
                            {#each providers as provider (provider.key)}
                                <Button
                                    variant="filled"
                                    color={provider.color}
                                    icon={provider.icon}
                                    iconSize={20}
                                    disabled={busy}
                                    onclick={() => {
                                        resetErrors()
                                        authorize.mutate(provider.key)
                                    }}
                                >
                                    {provider.label}
                                </Button>
                            {/each}
                        </div>
                    {/if}
                </div>
            {/if}
        </section>
    </div>
</main>

<style>
    .auth-layout {
        display: flex;
        min-height: 100vh;
        align-items: center;
        justify-content: center;
        padding: var(--mantine-spacing-md);
    }

    .auth-width {
        max-width: 25rem;
    }

    .auth-stack,
    .auth-password-fields,
    .auth-providers {
        display: flex;
        flex-direction: column;
    }

    .auth-stack {
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }

    .auth-brand {
        display: flex;
        justify-content: center;
        align-items: center;
        gap: 4px;
    }

    .auth-brand img {
        display: block;
        width: 3rem;
        height: auto;
        aspect-ratio: 1;
        object-fit: contain;
    }

    .auth-brand img.custom-logo {
        width: 40px;
        height: 40px;
    }

    h1 {
        margin: 0;
        font-family: Unbounded, sans-serif;
        font-size: var(--mantine-h1-font-size);
        font-weight: var(--mantine-h1-font-weight);
        line-height: var(--mantine-h1-line-height);
    }

    .auth-box {
        width: 27.5rem;
        max-width: 50rem;
        display: grid;
        gap: var(--mantine-spacing-lg);
        padding: 30px;
    }

    .auth-box.registration {
        padding: 0;
    }

    .auth-container {
        --container-size: 100%;
    }

    .auth-paper {
        --paper-radius: var(--mantine-radius-md);
    }

    .registration .auth-paper {
        padding: 30px;
    }

    h2 {
        margin: 0 0 var(--mantine-spacing-xs);
        font-size: var(--mantine-h2-font-size);
        font-weight: var(--mantine-h2-font-weight);
        line-height: var(--mantine-h2-line-height);
        text-align: center;
    }

    .register-description {
        margin: 0 0 var(--mantine-spacing-md);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        text-align: center;
    }

    .auth-password-fields {
        margin-top: var(--mantine-spacing-md);
        gap: var(--mantine-spacing-md);
    }

    .auth-form :global(.auth-submit) {
        margin-top: var(--mantine-spacing-xl);
    }

    .auth-divider {
        display: flex;
        margin-inline: auto;
        align-items: center;
        width: min(100%, 400px);
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }

    .auth-divider::before,
    .auth-divider::after {
        content: "";
        flex-grow: 1;
        border-top: 1px solid var(--mantine-color-default-border);
    }

    .auth-providers {
        margin-inline: auto;
        width: min(100%, 150px);
        gap: var(--mantine-spacing-md);
    }

    .auth-providers.wide {
        width: min(100%, 300px);
    }

    .auth-error {
        text-align: center;
    }

    @media (min-width: 40em) {
        .auth-box {
            width: 31.25rem;
        }
    }
</style>
