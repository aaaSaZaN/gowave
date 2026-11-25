<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { route } from '../router'
    import EditPlugin from './EditPlugin.svelte'
    import { pluginQuery } from './queries'

    let t = $derived(locale.t)
    let uuid = $derived(route.params['uuid'] ?? route.params['id'] ?? '')
    const plugin = createQuery(() => pluginQuery(uuid))
</script>

<svelte:head
    ><title>{plugin.data?.name ?? t('constants.node-plugins')} | Gowave</title></svelte:head
>
<main>
    {#if plugin.isError}
        <p role="alert">{plugin.error.message}</p>
    {:else if plugin.data}
        {#key uuid}
            <EditPlugin plugin={plugin.data} />
        {/key}
    {:else}
        <RouteLoading text="Loading plugin..." label={t('common.message.loading')} />
    {/if}
</main>
