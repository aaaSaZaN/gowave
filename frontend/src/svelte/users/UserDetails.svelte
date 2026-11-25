<script lang="ts">
    import { ROUTES } from '@shared/constants/routes'
    import { locale } from '../i18n.svelte'
    import { navigate, route } from '../router'
    import EntityStage from '../ui/EntityStage.svelte'
    import UserDialog from './UserDialog.svelte'

    const routeId = $derived(route.params['id'] ?? '')
    const valid = $derived(/^\d+$/.test(routeId) && Number.isSafeInteger(Number(routeId)))
    const title = $derived(locale.t('constants.users'))

    function close(): void {
        void navigate(ROUTES.DASHBOARD.MANAGEMENT.USERS, { replace: true })
    }

    $effect(() => {
        if (!valid) close()
    })
</script>

<svelte:head><title>{title} | Gowave</title></svelte:head>

<EntityStage id={routeId} {title} icon="TbUser" />
{#if valid}
    {#key routeId}
        <UserDialog id={Number(routeId)} {close} />
    {/key}
{/if}
