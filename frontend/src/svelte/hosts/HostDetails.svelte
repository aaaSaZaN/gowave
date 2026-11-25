<script lang="ts">
    import { ROUTES } from '@shared/constants/routes'
    import { locale } from '../i18n.svelte'
    import { navigate, route } from '../router'
    import EntityStage from '../ui/EntityStage.svelte'
    import HostDialog from './HostDialog.svelte'

    const uuid = $derived(route.params['id'] ?? '')
    const title = $derived(locale.t('constants.hosts'))
    function close(): void {
        void navigate(ROUTES.DASHBOARD.MANAGEMENT.HOSTS, { replace: true })
    }
</script>

<svelte:head><title>{title} | Gowave</title></svelte:head>

<EntityStage id={uuid} {title} icon="PiListChecks" />
{#key uuid}
    <HostDialog {uuid} {close} />
{/key}
