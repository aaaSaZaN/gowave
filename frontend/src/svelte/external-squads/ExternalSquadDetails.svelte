<script lang="ts">
    import { buildOpenEntityUrl, OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, route } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import EntityStage from '../ui/EntityStage.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import ExternalSquadEditor from './ExternalSquadEditor.svelte'
    import { externalSquadQuery } from './queries'

    let id = $derived(route.params['id'] ?? '')
    let t = $derived(locale.t)
    let editIdentity = $derived(route.search['edit'] === 'identity')
    let copied = $state(false)
    let copyError = $state(false)
    let copyTimer: ReturnType<typeof setTimeout> | undefined
    const squad = createQuery(() => ({ ...externalSquadQuery(id), retry: false }))

    function copyLink(): void {
        copyError = false
        void navigator.clipboard.writeText(buildOpenEntityUrl(OPEN_ENTITY.EXTERNAL_SQUAD, id)).then(
            () => {
                copied = true
                clearTimeout(copyTimer)
                copyTimer = setTimeout(() => (copied = false), 2000)
            },
            () => (copyError = true)
        )
    }

    onDestroy(() => clearTimeout(copyTimer))
</script>

<svelte:head
    ><title>{t('external-squads.drawer.widget.edit-external-squad')} | Gowave</title></svelte:head
>

<EntityStage {id} title={t('constants.external-squads')} icon="TbWebhook" />
<Overlay
    title={t('external-squads.drawer.widget.edit-external-squad')}
    icon="TbWebhook"
    tone="teal"
    width={600}
    drawer
    class="external-squad-drawer"
    inlineActions
    close={() => void navigate(ROUTES.DASHBOARD.MANAGEMENT.EXTERNAL_SQUADS, { replace: true })}
>
    {#snippet actions()}
        <ActionIcon
            icon={copied ? 'PiCheck' : 'TbLink'}
            size={28}
            iconSize={16}
            subtle
            label={t('common.action.copy-link')}
            onclick={copyLink}
        />
    {/snippet}
    {#if copyError}
        <p role="alert">{t('common.message.error')}</p>
    {/if}
    {#if squad.isPending}
        <RouteLoading compact />
    {:else if squad.isError}
        <p role="alert">{squad.error.message}</p>
    {:else}
        {#key `${id}-${editIdentity}`}
            <ExternalSquadEditor squad={squad.data} {editIdentity} />
        {/key}
    {/if}
</Overlay>

<style>
    :global(.external-squad-drawer .drawer-icon) {
        display: none;
    }
    :global(.external-squad-drawer .overlay-header) {
        min-height: 60px;
    }
    :global(.external-squad-drawer .overlay-heading) {
        gap: var(--mantine-spacing-xs);
    }
    :global(.external-squad-drawer .overlay-title h2) {
        font-size: var(--mantine-font-size-md);
        font-weight: 400;
        line-height: var(--mantine-line-height-md);
    }
</style>
