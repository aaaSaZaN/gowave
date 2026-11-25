<script lang="ts">
    import { logoutEvents } from '@shared/emitters/emit-logout'
    import { QueryClientProvider } from '@tanstack/svelte-query'
    import { Router } from 'sv-router'
    import { locale } from './i18n.svelte'
    import { queryClient } from './query-client'
    import RouteLoading from './RouteLoading.svelte'
    import { signOut } from './router'
    import { theme } from './theme.svelte'
    import ConnectionStatus from './ui/ConnectionStatus.svelte'
    import Notifications from './ui/Notifications.svelte'

    $effect(() => logoutEvents.subscribe(signOut))
    $effect(() => {
        document.documentElement.lang = locale.language
        document.documentElement.dir = locale.direction
    })
    $effect(() => {
        document.documentElement.dataset['colorScheme'] = theme.scheme
        document.documentElement.dataset['mantineColorScheme'] = theme.scheme
    })
</script>

<QueryClientProvider client={queryClient}>
    <ConnectionStatus />
    <Notifications />
    {#if locale.ready}
        <svelte:boundary>
            <Router />
            {#snippet failed(error, retry)}
                <main class="dashboard-content">
                    <h1>{locale.t('common.message.error')}</h1>
                    {#if error instanceof Error}
                        <p role="alert">{error.message}</p>
                    {/if}
                    <button type="button" onclick={retry}>
                        {locale.t('common.action.try-again')}
                    </button>
                </main>
            {/snippet}
        </svelte:boundary>
    {:else}
        <RouteLoading />
    {/if}
</QueryClientProvider>

<div class="safe-area-bottom" aria-hidden="true"></div>
