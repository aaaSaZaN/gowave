<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, route } from '../router'
    import { templateQuery } from '../subscriptions/queries'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import EditTemplate from './EditTemplate.svelte'
    import { templateLogos, templateType, templateTypes } from './templates'

    let t = $derived(locale.t)
    let help = $state(false)
    let type = $derived(templateType(route.params['type']))
    let uuid = $derived(route.params['uuid'] ?? '')
    const template = createQuery(() => ({ ...templateQuery(uuid), enabled: Boolean(type && uuid) }))
</script>

<svelte:head
    ><title>
        {type ? templateTypes[type].title : t('constants.templates')}
        | Gowave
    </title></svelte:head
>
<main>
    {#if !type || (template.data && template.data.templateType !== type)}
        <p role="alert">{t('common.message.nothing-found')}</p>
    {:else if template.isError}
        <p role="alert">{template.error.message}</p>
    {:else if template.data}
        <PageHeader title={template.data.name} icon={templateLogos[type]}>
            {#if type === 'XRAY_JSON'}
                <ActionIcon
                    icon="TbQuestionMark"
                    tone="lime"
                    label={t('help-action-icon.shared.help-article')}
                    onclick={() => (help = true)}
                />
            {/if}
            <ActionIcon
                icon="TbArrowBackUp"
                tone="gray"
                label={t('create-node-modal.widget.back')}
                onclick={() => void navigate('/dashboard/templates/:type', { params: { type } })}
            />
        </PageHeader>
        {#key uuid}
            <EditTemplate template={template.data} {type} />
        {/key}
    {:else}
        <RouteLoading text="Loading template..." label={t('common.message.loading')} />
    {/if}
</main>
{#if help}
    <HelpDrawer screen="EDITOR_TEMPLATES_XRAY_JSON" close={() => (help = false)} />
{/if}

<style>
    main {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
</style>
