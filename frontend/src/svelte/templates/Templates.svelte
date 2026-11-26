<script lang="ts">
    import {
        CreateSubscriptionTemplateCommand,
        DeleteSubscriptionTemplateCommand,
        ReorderSubscriptionTemplateCommand,
        SetSubscriptionTemplateTagsCommand,
        UpdateSubscriptionTemplateCommand
    } from '@remnawave/backend-contract'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { ZodError } from 'zod'
    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, p, route } from '../router'
    import { subscriptionTemplatesQuery, templateQuery } from '../subscriptions/queries'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import EmptyState from '../ui/EmptyState.svelte'
    import EntityCard from '../ui/EntityCard.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import { applyEntityOrder, entityOrder } from '../ui/entity-order'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { readSectionTag, writeSectionTag } from '../view-preferences'
    import {
        type Template,
        type TemplateType,
        templateLogos,
        templateType,
        templateTypes
    } from './templates'

    const confirmAction = useConfirmation()

    const list = createQuery(() => subscriptionTemplatesQuery)
    let t = $derived(locale.t)
    let type = $derived(templateType(route.params['type']) ?? 'MIHOMO')
    let templates = $derived(
        list.data?.templates.filter((item) => item.templateType === type) ?? []
    )
    let selectedTags = $state<Partial<Record<TemplateType, string>>>({})
    let activeTag = $derived(selectedTags[type] ?? readSectionTag(`templates:${type}`))
    const { drag, pendingOrder } = entityOrder(
        () => templates,
        async (order) => {
            if (!mutate.isPending && !activeTag) await mutate.mutateAsync(order)
        }
    )
    let visible = $derived(
        applyEntityOrder(templates, $pendingOrder).filter(
            (template) =>
                !activeTag ||
                (activeTag === NO_TAG
                    ? template.tags.length === 0
                    : template.tags.includes(activeTag))
        )
    )
    let dialogOpen = $state(false)
    let editing = $state<Template | null>(null)
    let editingTags = $state(false)
    let value = $state('')

    const edit = createMutation(() => ({
        mutationFn: async (input: {
            template: Template | null
            tags: boolean
            type: TemplateType
            value: string
        }) => {
            if (!input.template) {
                return submitCommand(CreateSubscriptionTemplateCommand, {
                    name: input.value,
                    templateType: input.type
                })
            }
            const { uuid } = input.template
            if (input.tags) {
                const tags = [
                    ...new Set(
                        input.value
                            .toUpperCase()
                            .split(/[,\s]+/)
                            .filter(Boolean)
                    )
                ]
                await submitCommand(SetSubscriptionTemplateTagsCommand, { uuid, tags })
            } else {
                await submitCommand(UpdateSubscriptionTemplateCommand, { uuid, name: input.value })
            }
            await queryClient.invalidateQueries({ queryKey: templateQuery(uuid).queryKey })
            return undefined
        },
        onSuccess: async (created) => {
            dialogOpen = false
            if (created) {
                await navigate('/dashboard/templates/:type/:uuid', {
                    params: { type: created.templateType, uuid: created.uuid }
                })
            }
            await queryClient.invalidateQueries({ queryKey: subscriptionTemplatesQuery.queryKey })
        }
    }))

    const mutate = createMutation(() => ({
        mutationFn: async (input: string | Template[]) => {
            if (typeof input === 'string') {
                const command = DeleteSubscriptionTemplateCommand
                await executeCommand({ ...command, TSQ_url: command.url(input) })
                queryClient.removeQueries({ queryKey: templateQuery(input).queryKey })
            } else {
                await submitCommand(ReorderSubscriptionTemplateCommand, {
                    items: input.map(({ uuid }, viewPosition) => ({ uuid, viewPosition }))
                })
            }
        },
        onSettled: () =>
            queryClient.invalidateQueries({ queryKey: subscriptionTemplatesQuery.queryKey })
    }))

    function open(template: Template | null, tags: boolean): void {
        edit.reset()
        editing = template
        editingTags = tags
        value = template ? (tags ? template.tags.join(', ') : template.name) : ''
        dialogOpen = true
    }

    function selectTag(tag: string): void {
        selectedTags[type] = tag
        writeSectionTag(`templates:${type}`, tag)
    }

    function copyUuid(uuid: string): void {
        void navigator.clipboard.writeText(uuid).then(
            () =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: uuid,
                    color: 'teal'
                }),
            (error: unknown) =>
                notifications.show({
                    title: t('common.message.error'),
                    message: error instanceof Error ? error.message : String(error),
                    color: 'red'
                })
        )
    }
</script>

<svelte:head>
    {#if route.params['type']}
        <title>{templateTypes[type].title} | Gowave</title>
    {/if}
</svelte:head>
{#if route.params['type']}
    <main class="entity-page">
        {#if list.isPending}
            <RouteLoading text="Loading templates..." />
        {:else}
            <PageHeader title={templateTypes[type].title} icon={templateLogos[type]}>
                <EntitySpotlight
                    icon={templateLogos[type]}
                    items={templates.map(({ uuid, name }) => ({
                        id: uuid,
                        name,
                        href: p('/dashboard/templates/:type/:uuid', { params: { type, uuid } })
                    }))}
                />
                <ActionIcon
                    icon="TbRefresh"
                    tone="cyan"
                    label={t('common.action.refresh')}
                    pending={list.isFetching}
                    onclick={() => void list.refetch()}
                />
                <ActionIcon
                    icon="TbPlus"
                    tone="teal"
                    label={t('common.action.create')}
                    onclick={() => open(null, false)}
                />
            </PageHeader>
            {#if list.isError}
                <p role="alert">{list.error.message}</p>
            {:else}
                <EntityTagBar items={templates} active={activeTag} select={selectTag} />
                {#if !visible.length}
                    <EmptyState
                        icon={templateLogos[type]}
                        label={t('common.message.nothing-found')}
                    />
                {:else}
                    <ul class="entity-grid">
                        {#each visible as template (template.uuid)}
                            {@const active = template.name === 'Default'}
                            {@const reorderable = !mutate.isPending && !activeTag}
                            <EntityCard
                                name={template.name}
                                icon={templateLogos[type]}
                                {active}
                                tags={template.tags}
                                badges={[]}
                                label={t('common.action.actions')}
                                href={p('/dashboard/templates/:type/:uuid', {
                                    params: { type, uuid: template.uuid }
                                })}
                                reorder={reorderable ? drag(template.uuid) : undefined}
                            >
                                {#snippet actions()}
                                    <button
                                        type="button"
                                        role="menuitem"
                                        tabindex="-1"
                                        onclick={() => copyUuid(template.uuid)}
                                    >
                                        <Icon name="PiCopy" size={18} />
                                        {t('common.action.copy-uuid')}
                                    </button>
                                    <button
                                        type="button"
                                        role="menuitem"
                                        tabindex="-1"
                                        disabled={active}
                                        onclick={() => open(template, false)}
                                    >
                                        <Icon name="PiPencil" size={18} />
                                        {t('common.action.rename')}
                                    </button>
                                    <button
                                        type="button"
                                        role="menuitem"
                                        tabindex="-1"
                                        onclick={() => open(template, true)}
                                    >
                                        <Icon name="TbTags" size={18} />{t('common.field.tags')}
                                    </button>
                                    <button
                                        type="button"
                                        role="menuitem"
                                        tabindex="-1"
                                        data-tone="red"
                                        disabled={mutate.isPending || active}
                                        onclick={async () => {
                                            if (
                                                await confirmAction(
                                                    t('common.message.confirm-action-description')
                                                )
                                            )
                                                mutate.mutate(template.uuid)
                                        }}
                                    >
                                        <Icon name="PiTrashDuotone" size={18} />
                                        {t('common.action.delete')}
                                    </button>
                                {/snippet}
                            </EntityCard>
                        {/each}
                    </ul>
                {/if}
            {/if}
            {#if mutate.error}
                <p role="alert">{mutate.error.message}</p>
            {/if}
        {/if}
        {#if dialogOpen}
            <Overlay
                title={editing
                    ? editingTags
                        ? t('common.field.tags')
                        : t('common.action.rename')
                    : t('common.field.creation')}
                icon={editing ? (editingTags ? 'PiTag' : 'PiPencil') : 'TbNewSection'}
                tone="teal"
                width={440}
                fullscreenOnMobile={false}
                preventClose={edit.isPending}
                close={() => (dialogOpen = false)}
            >
                <form
                    onsubmit={(event) => {
                        event.preventDefault()
                        if (!edit.isPending)
                            edit.mutate({
                                template: editing,
                                tags: editingTags,
                                type,
                                value: value.trim()
                            })
                    }}
                >
                    <label class="edit-label">
                        <span
                            >{editingTags
                                ? t('common.field.tags')
                                : editing
                                  ? t('common.field.name')
                                  : t('header-action-buttons.feature.template-name')}
                            {#if !editingTags}
                                <span class="required"> *</span>
                            {/if}</span
                        >
                        <input
                            class="gw-input"
                            name={editingTags ? 'tags' : 'name'}
                            required={!editingTags}
                            data-autofocus
                            placeholder={!editing ? 'My Mihomo template' : undefined}
                            bind:value
                        >
                    </label>
                    {#if edit.error}
                        <p role="alert">
                            {edit.error instanceof ZodError
                                ? (edit.error.issues[0]?.message ?? edit.error.message)
                                : edit.error.message}
                        </p>
                    {/if}
                    <div class="button-row">
                        <Button
                            color="gray"
                            variant="light"
                            onclick={() => (dialogOpen = false)}
                            disabled={edit.isPending}
                        >
                            {t('common.action.cancel')}
                        </Button>
                        <Button type="submit" color="teal" variant="light" pending={edit.isPending}>
                            {editing ? t('common.action.save') : t('common.action.create')}
                        </Button>
                    </div>
                </form>
            </Overlay>
        {/if}
    </main>
{/if}

<style>
    .entity-page,
    form {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .edit-label {
        display: grid;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-md);
    }
    .edit-label input {
        width: 100%;
        height: 36px;
        min-height: 36px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 400;
    }
    .edit-label input:focus-visible {
        outline: none;
        border-color: var(--mantine-primary-color-filled);
    }
    .required {
        color: var(--mantine-color-error);
    }
    .button-row {
        justify-content: flex-end;
        gap: var(--mantine-spacing-md);
    }
</style>
