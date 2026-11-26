<script lang="ts">
    import {
        CloneSubpageConfigCommand,
        CreateSubpageConfigCommand,
        DeleteSubpageConfigCommand,
        type GetSubpageConfigsCommand,
        ReorderSubpageConfigsCommand,
        SetSubpageConfigTagsCommand,
        UpdateSubpageConfigCommand
    } from '@remnawave/backend-contract'
    import { SUBPAGE_DEFAULT_CONFIG_UUID } from '@remnawave/subscription-page-types'
    import { ROUTES } from '@shared/constants/routes'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, p } from '../router'
    import { subpageConfigsQuery, subpageQuery } from '../subscriptions/queries'
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
    import RenameDialog from '../ui/RenameDialog.svelte'
    import { readSectionTag, writeSectionTag } from '../view-preferences'

    const confirmAction = useConfirmation()

    type Entry = GetSubpageConfigsCommand.Response['response']['configs'][number]
    type Edit = { action: 'create' } | { action: 'tags'; uuid: string }
    const list = createQuery(() => subpageConfigsQuery)
    let t = $derived(locale.t)
    let tag = $state(readSectionTag('subpageConfigs'))
    let items = $derived(list.data?.configs ?? [])
    const { drag, pendingOrder } = entityOrder(
        () => items,
        async (order) => {
            if (!action.isPending && !tag) await action.mutateAsync({ kind: 'reorder', order })
        }
    )
    let visible = $derived(
        applyEntityOrder(items, $pendingOrder).filter(
            (item) => !tag || (tag === NO_TAG ? item.tags.length === 0 : item.tags.includes(tag))
        )
    )
    let editing = $state<Edit>()
    let value = $state('')
    let renaming = $state<Entry>()

    const edit = createMutation(() => ({
        mutationFn: async ({ intent, value }: { intent: Edit; value: string }) => {
            if (intent.action === 'create') {
                return submitCommand(CreateSubpageConfigCommand, { name: value })
            }
            const { uuid } = intent
            const tags = [
                ...new Set(
                    value
                        .toUpperCase()
                        .split(/[,\s]+/)
                        .filter(Boolean)
                )
            ]
            await submitCommand(SetSubpageConfigTagsCommand, { uuid, tags })
            await queryClient.invalidateQueries({ queryKey: subpageQuery(uuid).queryKey })
            return undefined
        },
        onSuccess: async (created) => {
            editing = undefined
            const refresh = queryClient.invalidateQueries({
                queryKey: subpageConfigsQuery.queryKey
            })
            if (created) {
                await navigate(ROUTES.DASHBOARD.SUBPAGE_CONFIGS.SUBPAGE_CONFIG_BY_UUID, {
                    params: { uuid: created.uuid }
                })
            }
            await refresh
        }
    }))

    const action = createMutation(() => ({
        mutationFn: async (
            change: { kind: 'clone' | 'delete'; uuid: string } | { kind: 'reorder'; order: Entry[] }
        ) => {
            switch (change.kind) {
                case 'reorder':
                    await submitCommand(ReorderSubpageConfigsCommand, {
                        items: change.order.map(({ uuid }, viewPosition) => ({
                            uuid,
                            viewPosition
                        }))
                    })
                    return
                case 'clone':
                    await submitCommand(CloneSubpageConfigCommand, { cloneFromUuid: change.uuid })
                    return
                case 'delete': {
                    const command = DeleteSubpageConfigCommand
                    await executeCommand({ ...command, TSQ_url: command.url(change.uuid) })
                    queryClient.removeQueries({ queryKey: subpageQuery(change.uuid).queryKey })
                }
            }
        },
        onSettled: () => queryClient.invalidateQueries({ queryKey: subpageConfigsQuery.queryKey })
    }))

    function open(intent: Edit, initial: string): void {
        edit.reset()
        editing = intent
        value = initial
    }

    function changeTag(value: string): void {
        tag = value
        writeSectionTag('subpageConfigs', tag)
    }

    function copy(uuid: string): void {
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

<svelte:head><title>{t('constants.subscription-page')} | Gowave</title></svelte:head>
<main class="entity-page">
    {#if list.isPending}
        <RouteLoading text="Loading subpage configs..." />
    {:else}
        <PageHeader title={t('constants.subscription-page')} icon="TbFile">
            <EntitySpotlight
                icon="TbFile"
                items={items.map(({ uuid, name }) => ({
                    id: uuid,
                    name,
                    href: p(ROUTES.DASHBOARD.SUBPAGE_CONFIGS.SUBPAGE_CONFIG_BY_UUID, {
                        params: { uuid }
                    })
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
                onclick={() => open({ action: 'create' }, '')}
            />
        </PageHeader>
        {#if list.error}
            <p role="alert">{list.error.message}</p>
        {:else}
            <EntityTagBar {items} active={tag} select={changeTag} />
            {#if !visible.length}
                <EmptyState icon="TbFile" label={t('common.message.nothing-found')} />
            {:else}
                <ul class="entity-grid">
                    {#each visible as config (config.uuid)}
                        {@const reserved = config.uuid === SUBPAGE_DEFAULT_CONFIG_UUID}
                        {@const reorderable = !action.isPending && !tag}
                        <EntityCard
                            name={config.name}
                            icon="TbFile"
                            active={reserved}
                            tags={config.tags}
                            badges={[]}
                            label={t('common.action.actions')}
                            href={p(ROUTES.DASHBOARD.SUBPAGE_CONFIGS.SUBPAGE_CONFIG_BY_UUID, {
                                params: { uuid: config.uuid }
                            })}
                            reorder={reorderable ? drag(config.uuid) : undefined}
                        >
                            {#snippet actions()}
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => copy(config.uuid)}
                                >
                                    <Icon name="PiCopy" size={18} />{t('common.action.copy-uuid')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => (renaming = config)}
                                >
                                    <Icon name="PiPencil" size={18} />{t('common.action.rename')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() =>
                                        open(
                                            { action: 'tags', uuid: config.uuid },
                                            config.tags.join(', ')
                                        )}
                                >
                                    <Icon name="TbTags" size={18} />{t('common.field.tags')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    disabled={action.isPending}
                                    onclick={() =>
                                        action.mutate({ kind: 'clone', uuid: config.uuid })}
                                >
                                    <Icon name="TbCopy" size={18} />{t('common.action.clone')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="red"
                                    disabled={reserved || action.isPending}
                                    onclick={async () => {
                                        if (
                                            await confirmAction(
                                                t('common.message.confirm-action-description')
                                            )
                                        )
                                            action.mutate({ kind: 'delete', uuid: config.uuid })
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
        {#if action.error}
            <p role="alert">{action.error.message}</p>
        {/if}
    {/if}
    {#if editing}
        {@const intent = editing}
        <Overlay
            title={intent.action === 'tags' ? t('common.field.tags') : t('common.field.creation')}
            icon={intent.action === 'tags' ? 'PiTag' : 'TbNewSection'}
            tone="teal"
            width={440}
            fullscreenOnMobile={false}
            preventClose={edit.isPending}
            close={() => (editing = undefined)}
        >
            <form
                onsubmit={(event) => {
                    event.preventDefault()
                    if (!edit.isPending) edit.mutate({ intent, value: value.trim() })
                }}
            >
                <label class="edit-label"
                    ><span>
                        {intent.action === 'tags' ? t('common.field.tags') : t('common.field.name')}
                        {#if intent.action !== 'tags'}
                            <span class="required"> *</span>
                        {/if}
                    </span>
                    <input
                        class="gw-input"
                        name={intent.action === 'tags' ? 'tags' : 'name'}
                        required={intent.action !== 'tags'}
                        data-autofocus
                        placeholder={intent.action === 'create'
                            ? 'My Subscription Page Config'
                            : undefined}
                        bind:value
                    >
                </label>
                {#if edit.error}
                    <p role="alert">{edit.error.message}</p>
                {/if}
                <div class="button-row">
                    <Button
                        color="gray"
                        variant="light"
                        onclick={() => (editing = undefined)}
                        disabled={edit.isPending}
                    >
                        {t('common.action.cancel')}
                    </Button>
                    <Button type="submit" color="teal" variant="light" pending={edit.isPending}>
                        {intent.action === 'create'
                            ? t('common.action.create')
                            : t('common.action.save')}
                    </Button>
                </div>
            </form>
        </Overlay>
    {/if}
    {#if renaming}
        {@const config = renaming}
        <RenameDialog
            currentName={config.name}
            rename={async (name) => {
                await submitCommand(UpdateSubpageConfigCommand, { uuid: config.uuid, name })
                await Promise.all([
                    queryClient.invalidateQueries({ queryKey: subpageQuery(config.uuid).queryKey }),
                    queryClient.invalidateQueries({ queryKey: subpageConfigsQuery.queryKey })
                ])
            }}
            close={() => (renaming = undefined)}
        />
    {/if}
</main>
<style>
    .button-row {
        justify-content: flex-end;
        gap: var(--mantine-spacing-md);
    }
    .entity-page,
    form {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .edit-label {
        display: grid;
        gap: 0;
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
</style>
