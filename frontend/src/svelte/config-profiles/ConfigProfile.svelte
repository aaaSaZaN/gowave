<script lang="ts">
    import { OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, route } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import CopyEntityAction from '../ui/CopyEntityAction.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { profileQuery, snippetsQuery } from './queries'
    import Snippets from './Snippets.svelte'
    import XrayEditor from './XrayEditor.svelte'

    const profile = createQuery(() =>
        profileQuery(route.params['uuid'] ?? route.params['id'] ?? '')
    )
    const snippets = createQuery(() => snippetsQuery)
    // Read both query results before branching so either response can update the editor.
    let data = $derived({ profile: profile.data, snippets: snippets.data })
    let t = $derived(locale.t)
    let help = $state(false)
    let showSnippets = $state(false)

    function retry(): void {
        if (!profile.data) void profile.refetch()
        if (!snippets.data) void snippets.refetch()
    }
</script>

<svelte:head><title>{t('constants.config-profiles')} | Gowave</title></svelte:head>

<main>
    {#if data.profile && data.snippets}
        <PageHeader title={data.profile.name} description={data.profile.uuid} icon="TbFile">
            <CopyEntityAction entity={OPEN_ENTITY.CONFIG_PROFILE} id={data.profile.uuid} />
            <ActionIcon
                icon="TbQuestionMark"
                tone="lime"
                label={t('help-action-icon.shared.help-article')}
                onclick={() => (help = true)}
            />
            <ActionIcon
                icon="TbCode"
                tone="teal"
                label={t('snippets.drawer.widget.snippets')}
                onclick={() => (showSnippets = true)}
            />
            <ActionIcon
                icon="TbArrowBackUp"
                tone="gray"
                label={t('create-node-modal.widget.back')}
                onclick={() => void navigate(ROUTES.DASHBOARD.MANAGEMENT.CONFIG_PROFILES)}
            />
        </PageHeader>
        {#key data.profile.uuid}
            <XrayEditor profile={data.profile} snippets={data.snippets.snippets} />
        {/key}
    {:else if profile.error || snippets.error}
        <p role="alert">{(profile.error ?? snippets.error)?.message}</p>
        <button type="button" onclick={retry}>{t('common.action.try-again')}</button>
    {:else}
        <RouteLoading label={t('common.message.loading')} />
    {/if}
</main>
{#if help}
    <HelpDrawer screen="PAGE_CONFIG_PROFILES" close={() => (help = false)} />
{/if}
{#if showSnippets}
    <Snippets close={() => (showSnippets = false)} />
{/if}

<style>
    main {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
</style>
