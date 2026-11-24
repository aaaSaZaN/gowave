<script lang="ts">
    import { ROUTES } from '@shared/constants/routes'
    import { createMutation } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { navigate, returnToDashboard, route } from '../router'
    import { session } from '../session.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import { authenticateOAuth } from './api'

    const callback = createMutation(() => ({
        mutationFn: authenticateOAuth,
        onSuccess: ({ accessToken }) => {
            session.setToken(accessToken)
            returnToDashboard()
        },
        onError: (error) => {
            notifications.show({ title: 'OAuth2 Callback', message: error.message, color: 'red' })
            session.setToken('')
            void navigate(ROUTES.AUTH.LOGIN, { replace: true })
        }
    }))

    onMount(() => {
        const provider = route.params['provider']
        const search = new URLSearchParams(window.location.search)
        const code = search.get('code')
        const state = search.get('state')
        if (provider && code && state) callback.mutate({ provider, code, state })
    })
</script>

<svelte:head><title>OAuth2 Authentication | Gowave</title></svelte:head>

<main class="auth-layout">
    <section class="auth-card" aria-labelledby="oauth-heading" role="status">
        {#if callback.isPending}
            <span class="dots" aria-hidden="true">•••</span>
        {:else}
            <Icon name="TbCheck" color="var(--mantine-color-teal-6)" size={48} />
        {/if}
        <div>
            <h1 id="oauth-heading">Authenticating...</h1>
            <p>Verifying credentials...</p>
        </div>
    </section>
</main>

<style>
    .auth-layout {
        display: grid;
        min-height: 60vh;
        place-items: center;
    }

    .auth-card {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-xl);
        text-align: center;
    }

    h1 {
        margin: 0 0 var(--mantine-spacing-md);
        font-size: var(--mantine-h2-font-size);
    }

    p {
        margin: 0;
        color: var(--mantine-color-dimmed);
    }

    .dots {
        color: var(--mantine-color-cyan-6);
        font-size: var(--mantine-font-size-xl);
        letter-spacing: 0.25em;
    }
</style>
