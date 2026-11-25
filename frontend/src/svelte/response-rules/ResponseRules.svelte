<script lang="ts">
    import type { TSubscriptionTemplateType } from '@remnawave/backend-contract'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import RouteLoading from '../RouteLoading.svelte'
    import { subscriptionSettingsQuery, subscriptionTemplatesQuery } from '../subscriptions/queries'
    import PageHeader from '../ui/PageHeader.svelte'
    import RulesEditor from './RulesEditor.svelte'

    const settings = createQuery(() => subscriptionSettingsQuery)
    const templates = createQuery(() => subscriptionTemplatesQuery)

    let t = $derived(locale.t)
    let data = $derived({ settings: settings.data, templates: templates.data })
    let groupedTemplates = $derived.by(() => {
        const groups: Partial<Record<TSubscriptionTemplateType, string[]>> = {}
        for (const { name, templateType } of data.templates?.templates ?? []) {
            const names = groups[templateType] ?? []
            names.push(name)
            groups[templateType] = names
        }
        return groups
    })

    function retry(): void {
        if (!settings.data) void settings.refetch()
        if (!templates.data) void templates.refetch()
    }
</script>

<svelte:head><title>{t('constants.response-rules')} | Gowave</title></svelte:head>

<main>
    <PageHeader title={t('constants.response-rules')} icon="TbRoute" />
    {#if data.settings && data.templates}
        {#key data.settings.uuid}
            <RulesEditor settings={data.settings} {groupedTemplates} />
        {/key}
    {:else if settings.error || templates.error}
        <p role="alert">{(settings.error ?? templates.error)?.message}</p>
        <button type="button" onclick={retry}>{t('common.action.try-again')}</button>
    {:else}
        <RouteLoading label={t('common.message.loading')} />
    {/if}
</main>

<style>
    main {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
</style>
