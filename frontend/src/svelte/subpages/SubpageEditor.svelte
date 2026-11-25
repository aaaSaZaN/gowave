<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { route } from '../router'
    import { subpageQuery } from '../subscriptions/queries'
    import EditSubpage from './EditSubpage.svelte'

    let uuid = $derived(route.params['uuid'] ?? route.params['id'] ?? '')
    let t = $derived(locale.t)
    const query = createQuery(() => subpageQuery(uuid))
</script>

<svelte:head
    ><title>{query.data?.name ?? t('constants.subscription-page')} | Gowave</title></svelte:head
>
<main>
    {#if query.isPending}
        <RouteLoading text="Loading config..." label={t('common.message.loading')} />
    {:else if query.error}
        <p role="alert">{query.error.message}</p>
    {:else if query.data}
        {#key uuid}
            <EditSubpage entry={query.data} />
        {/key}
    {/if}
</main>
