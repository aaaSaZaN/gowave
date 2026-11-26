<script lang="ts">
    import {
        CloneNodePluginCommand,
        CreateNodePluginCommand,
        DeleteNodePluginCommand,
        type GetNodePluginsCommand,
        ReorderNodePluginCommand,
        SetNodePluginTagsCommand,
        UpdateNodePluginCommand
    } from '@remnawave/backend-contract'
    import { ROUTES } from '@shared/constants/routes'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { ZodError } from 'zod'
    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { nodesQuery } from '../nodes/queries'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
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
    import PluginExecutor from './PluginExecutor.svelte'
    import { nodePluginsQuery, pluginQuery } from './queries'
    import SharedLists from './SharedLists.svelte'

    type Entry = GetNodePluginsCommand.Response['response']['nodePlugins'][number]
    type Edit = { action: 'create' } | { action: 'name' | 'tags'; uuid: string }
    const list = createQuery(() => nodePluginsQuery)
    const nodes = createQuery(() => nodesQuery)
    let shared = $state(false)
    let executor = $state(false)
    let t = $derived(locale.t)
    let tag = $state(readSectionTag('nodePlugins'))
    let activePlugin = $state<string>()
    const linkedNodes = $derived(
        nodes.data?.filter((node) => node.activePluginUuid === activePlugin) ?? []
    )
    const pluginKinds = [
        { label: 'Torrent Blocker', color: 'cyan', icon: 'TbFlame' },
        { label: 'Ingress Filter', color: 'teal', icon: 'TbLogin' },
        { label: 'Egress Filter', color: 'orange', icon: 'TbLogout' },
        { label: 'Connection Drop', color: 'grape', icon: 'TbPlugConnectedX' }
    ] as const
    let items = $derived(list.data?.nodePlugins ?? [])
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
    let editorOpen = $state(false)
    let editing = $state<Edit>({ action: 'create' })
    let value = $state('')

    const edit = createMutation(() => ({
        mutationFn: async ({ intent, value }: { intent: Edit; value: string }) => {
            if (intent.action === 'create') {
                return submitCommand(CreateNodePluginCommand, { name: value })
            }
            const { uuid } = intent
            if (intent.action === 'tags') {
                const tags = [
                    ...new Set(
                        value
                            .toUpperCase()
                            .split(/[,\s]+/)
                            .filter(Boolean)
                    )
                ]
                await submitCommand(SetNodePluginTagsCommand, { uuid, tags })
            } else {
                await submitCommand(UpdateNodePluginCommand, { uuid, name: value })
            }
            await queryClient.invalidateQueries({ queryKey: pluginQuery(uuid).queryKey })
            return undefined
        },
        onSuccess: async (created) => {
            await queryClient.invalidateQueries({ queryKey: nodePluginsQuery.queryKey })
            editorOpen = false
            if (created) {
                await navigate(ROUTES.DASHBOARD.MANAGEMENT.NODE_PLUGINS.NODE_PLUGIN_BY_UUID, {
                    params: { uuid: created.uuid }
                })
            }
        }
    }))

    const action = createMutation(() => ({
        mutationFn: async (
            change: { kind: 'clone' | 'delete'; uuid: string } | { kind: 'reorder'; order: Entry[] }
        ) => {
            switch (change.kind) {
                case 'reorder':
                    await submitCommand(ReorderNodePluginCommand, {
                        items: change.order.map(({ uuid }, viewPosition) => ({
                            uuid,
                            viewPosition
                        }))
                    })
                    return
                case 'clone':
                    await submitCommand(CloneNodePluginCommand, { cloneFromUuid: change.uuid })
                    return
                case 'delete': {
                    const command = DeleteNodePluginCommand
                    await executeCommand({ ...command, TSQ_url: command.url(change.uuid) })
                    queryClient.removeQueries({ queryKey: pluginQuery(change.uuid).queryKey })
                }
            }
        },
        onSettled: () => queryClient.invalidateQueries({ queryKey: nodePluginsQuery.queryKey })
    }))

    function open(intent: Edit, initial: string): void {
        edit.reset()
        editing = intent
        value = initial
        editorOpen = true
    }

    function changeTag(value: string): void {
        tag = value
        writeSectionTag('nodePlugins', tag)
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

<svelte:head><title>{t('constants.node-plugins')} | Gowave</title></svelte:head>
<main class="plugins-page">
    {#if list.isPending}
        <RouteLoading text="Loading node plugins..." />
    {:else}
        <PageHeader title={`${t('constants.node-plugins')} β`} icon="TbPackage" wrapActions>
            <div class="plugin-tools">
                <div class="documentation">
                    <ActionIcon
                        icon="TbBook"
                        tone="lime"
                        label={t('common.action.documentation')}
                        onclick={() =>
                            window.open(
                                'https://docs.rw/docs/learn/node-plugins',
                                '_blank',
                                'noopener,noreferrer'
                            )}
                    />
                </div>
                <div>
                    <EntitySpotlight
                        icon="TbPackage"
                        items={items.map(({ uuid, name }) => ({
                            id: uuid,
                            name,
                            href: p(ROUTES.DASHBOARD.MANAGEMENT.NODE_PLUGINS.NODE_PLUGIN_BY_UUID, {
                                params: { uuid }
                            })
                        }))}
                    />
                </div>
                <div>
                    <ActionIcon
                        icon="TbList"
                        tone="indigo"
                        label={t('common.field.shared-lists')}
                        onclick={() => (shared = true)}
                    />
                </div>
                <div>
                    <ActionIcon
                        icon="TbTerminal"
                        tone="grape"
                        label="Executor"
                        onclick={() => (executor = true)}
                    />
                </div>
                <div>
                    <ActionIcon
                        icon="TbRefresh"
                        tone="cyan"
                        label={t('common.action.refresh')}
                        pending={list.isFetching}
                        onclick={() => void list.refetch()}
                    />
                </div>
                <div>
                    <ActionIcon
                        icon="TbPlus"
                        tone="teal"
                        label={t('common.action.create')}
                        onclick={() => open({ action: 'create' }, '')}
                    />
                </div>
            </div>
        </PageHeader>
        {#if list.error}
            <p role="alert">{list.error.message}</p>
        {:else if !items.length}
            <section class="empty-plugins">
                <header class="warning">
                    <span class="warning-icon"><Icon name="TbAlertTriangle" size={20} /></span>
                    <div>
                        <h2>{t('common.message.warning')}</h2>
                        <p>
                            {t(
                                'node-plugins-grid.widget.node-plugins-are-an-advanced-feature-please-review-the-documentation-before-use'
                            )}
                        </p>
                    </div>
                </header>
                <div class="empty-body">
                    <span class="empty-icon"><Icon name="TbPackage" size={32} /></span>
                    <div>
                        <h3>{t('node-plugins-grid.widget.no-node-plugins-yet')}</h3>
                        <p>
                            {t(
                                'node-plugins-grid.widget.create-a-plugin-to-extend-node-capabilities-with'
                            )}
                        </p>
                    </div>
                    <div class="kinds">
                        {#each pluginKinds as kind (kind.label)}
                            <span
                                style:color={`var(--mantine-color-${kind.color}-light-color)`}
                                style:background={`var(--mantine-color-${kind.color}-light)`}
                                ><Icon name={kind.icon} size={16} />{kind.label}</span
                            >
                        {/each}
                    </div>
                </div>
            </section>
        {:else}
            <EntityTagBar {items} active={tag} select={changeTag} />
            {#if !visible.length}
                <EmptyState icon="TbPackage" label={t('common.message.nothing-found')} />
            {:else}
                <ul class="entity-grid">
                    {#each visible as config (config.uuid)}
                        {@const reorderable = !action.isPending && !tag}
                        <EntityCard
                            name={config.name}
                            icon="TbPackage"
                            active={true}
                            highlight={false}
                            tags={config.tags}
                            badges={[]}
                            label={t('common.action.actions')}
                            href={p(ROUTES.DASHBOARD.MANAGEMENT.NODE_PLUGINS.NODE_PLUGIN_BY_UUID, {
                                params: { uuid: config.uuid }
                            })}
                            reorder={reorderable ? drag(config.uuid) : undefined}
                        >
                            {#snippet actions()}
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => (activePlugin = config.uuid)}
                                >
                                    <Icon name="PiCpu" size={18} />
                                    {t('node-plugin-card.widget.active-on-nodes')}
                                </button>
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
                                    onclick={() =>
                                        open({ action: 'name', uuid: config.uuid }, config.name)}
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
                                    disabled={action.isPending}
                                    onclick={() => {
                                        if (
                                            window.confirm(
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
        {#if activePlugin}
            <Overlay
                width={620}
                icon="TbPackage"
                tone="teal"
                title={t('node-plugin-card.widget.active-on-nodes')}
                close={() => (activePlugin = undefined)}
            >
                <ul>
                    {#each linkedNodes as node (node.uuid)}
                        <li>
                            <a href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}
                                >{node.name}</a
                            >
                        </li>
                    {:else}
                        <li>{t('common.message.nothing-found')}</li>
                    {/each}
                </ul>
            </Overlay>
        {/if}
        {#if action.error}
            <p role="alert">{action.error.message}</p>
        {/if}

        {#if shared}
            <SharedLists close={() => (shared = false)} />
        {/if}
        {#if executor}
            <PluginExecutor nodes={nodes.data ?? []} close={() => (executor = false)} />
        {/if}
    {/if}
    {#if editorOpen}
        <Overlay
            title={editing.action === 'tags'
                ? t('common.field.tags')
                : editing.action === 'name'
                  ? t('common.action.rename')
                  : t('common.field.creation')}
            icon={editing.action === 'create' ? 'TbNewSection' : 'TbEdit'}
            tone="teal"
            width={440}
            fullscreenOnMobile={false}
            preventClose={edit.isPending}
            close={() => (editorOpen = false)}
        >
            <form
                onsubmit={(event) => {
                    event.preventDefault()
                    if (!edit.isPending) edit.mutate({ intent: editing, value: value.trim() })
                }}
            >
                <label class="edit-label"
                    ><span>
                        {editing.action === 'tags'
                            ? t('common.field.tags')
                            : t('common.field.name')}
                        {#if editing.action !== 'tags'}
                            <span class="required"> *</span>
                        {/if}
                    </span>
                    <input
                        name={editing.action === 'tags' ? 'tags' : 'name'}
                        data-autofocus
                        placeholder={editing.action === 'create' ? 'My Node Plugin' : undefined}
                        required={editing.action !== 'tags'}
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
                        onclick={() => (editorOpen = false)}
                        disabled={edit.isPending}
                        >{t('common.action.cancel')}</Button
                    >
                    <Button color="teal" variant="light" type="submit" pending={edit.isPending}
                        >{editing.action === 'create'
                            ? t('common.action.create')
                            : t('common.action.save')}</Button
                    >
                </div>
            </form>
        </Overlay>
    {/if}
</main>
<style>
    .button-row {
        justify-content: flex-end;
        gap: var(--mantine-spacing-md);
    }
    .plugins-page,
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
    .plugin-tools {
        display: flex;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-md);
    }
    .plugin-tools > div {
        flex: 1 0 42px;
    }
    .documentation :global(button) {
        width: 100%;
    }
    .empty-plugins {
        min-width: 0;
        padding: var(--mantine-spacing-xl);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .warning {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding-block-end: var(--mantine-spacing-md);
        border-block-end: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    .warning-icon {
        display: grid;
        width: 34px;
        height: 34px;
        flex: none;
        place-items: center;
        border: 1px solid rgb(250 138 20 / 30%);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-orange-light);
        color: var(--mantine-color-orange-4);
    }
    .warning h2 {
        margin: 0;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-h4-line-height);
    }
    .warning p,
    .empty-body p {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .empty-body {
        display: grid;
        place-items: center;
        text-align: center;
        gap: var(--mantine-spacing-lg);
        padding: calc(var(--mantine-spacing-xl) + var(--mantine-spacing-md)) 0
            var(--mantine-spacing-xl);
    }
    .empty-icon {
        display: grid;
        place-items: center;
        width: 64px;
        height: 64px;
        border: 1px solid rgb(134 142 150 / 30%);
        border-radius: 50%;
        background: var(--mantine-color-gray-light);
    }
    .empty-body h3 {
        margin: 0 0 var(--mantine-spacing-xs);
        font-size: var(--mantine-font-size-lg);
        font-weight: 600;
        line-height: var(--mantine-line-height-lg);
    }
    .empty-body p {
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }
    .kinds {
        display: flex;
        flex-wrap: wrap;
        justify-content: center;
        gap: var(--mantine-spacing-sm);
    }
    .kinds span {
        display: flex;
        align-items: center;
        gap: 5px;
        height: 26px;
        padding-inline: 13px;
        border-radius: var(--mantine-radius-md);
        font-size: 13px;
        font-weight: 700;
        letter-spacing: 0.25px;
        text-transform: uppercase;
    }
</style>
