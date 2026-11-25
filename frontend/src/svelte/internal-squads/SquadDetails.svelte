<script lang="ts">
    import { ROUTES } from '@shared/constants/routes'
    import { locale } from '../i18n.svelte'
    import { navigate, route } from '../router'
    import EntityStage from '../ui/EntityStage.svelte'
    import SquadInbounds from './SquadInbounds.svelte'

    let id = $derived(route.params['id'] ?? '')
    let t = $derived(locale.t)
    function close(): void {
        void navigate(ROUTES.DASHBOARD.MANAGEMENT.INTERNAL_SQUADS, { replace: true })
    }
</script>

<svelte:head><title>{t('constants.internal-squads')} | Gowave</title></svelte:head>
<EntityStage {id} title={t('constants.internal-squads')} icon="TbCirclesRelation" />
{#key id}
    <SquadInbounds uuid={id} {close} />
{/key}
