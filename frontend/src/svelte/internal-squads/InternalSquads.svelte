<script lang="ts">
    import {
        AddUsersToInternalSquadCommand,
        DeleteInternalSquadCommand,
        DeleteUsersFromInternalSquadCommand,
        ReorderInternalSquadCommand,
        UpdateInternalSquadCommand
    } from '@remnawave/backend-contract'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import CreateSquad from '../ui/CreateSquad.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import EmptyState from '../ui/EmptyState.svelte'
    import EntityCard from '../ui/EntityCard.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import { applyEntityOrder, entityOrder } from '../ui/entity-order'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import RenameDialog from '../ui/RenameDialog.svelte'
    import { usersQueryKey } from '../users/queries'
    import { type Squad, squadsQuery } from './queries'
    import SquadEditor from './SquadEditor.svelte'
    import SquadInbounds from './SquadInbounds.svelte'
    import SquadNodes from './SquadNodes.svelte'
    import SquadUsage from './SquadUsage.svelte'

    const confirmAction = useConfirmation()

    const squads = createQuery(() => squadsQuery)
    const commands = {
        add: AddUsersToInternalSquadCommand,
        remove: DeleteUsersFromInternalSquadCommand,
        delete: DeleteInternalSquadCommand
    }
    type SquadAction = keyof typeof commands
    type SquadChange =
        | { action: SquadAction; uuid: string }
        | { items: ReorderInternalSquadCommand.RequestBody['items'] }
    let pending = $state(false)
    let failure = $state('')
    const update = createMutation(() => ({
        mutationFn: async (change: SquadChange) => {
            if ('items' in change) {
                await submitCommand(ReorderInternalSquadCommand, change)
                return
            }
            const command = commands[change.action]
            const { uuid } = command.RequestParamSchema.parse({ uuid: change.uuid })
            await executeCommand({ ...command, TSQ_url: command.url(uuid) })
        }
    }))

    async function apply(change: SquadChange): Promise<void> {
        if (pending) return
        pending = true
        failure = ''
        try {
            await update.mutateAsync(change)
            await queryClient.invalidateQueries({ queryKey: ['internalSquads'] })
            if ('action' in change) await queryClient.invalidateQueries({ queryKey: usersQueryKey })
        } catch (error) {
            failure = error instanceof Error ? error.message : String(error)
        } finally {
            pending = false
        }
    }

    let renaming = $state<Squad>()
    let editing = $state<Squad>()
    let creating = $state(false)
    let inboundSquad = $state<string>()
    let report = $state<{ kind: 'usage' | 'nodes'; squad: Squad }>()
    let helpOpen = $state(false)
    let clipboardError = $state('')
    let tag = $state('')
    let t = $derived(locale.t)
    let ordered = $derived(squads.data?.internalSquads ?? [])
    const { drag, pendingOrder } = entityOrder(
        () => ordered,
        (items) => saveOrder(items.map(({ uuid }) => uuid))
    )
    let visible = $derived(
        applyEntityOrder(ordered, $pendingOrder).filter(
            (squad) => !tag || (tag === NO_TAG ? !squad.tags.length : squad.tags.includes(tag))
        )
    )

    async function perform(action: SquadAction, squad: Squad): Promise<void> {
        const confirmation = {
            add: t('common.message.confirm-add-all-users-to-internal-squad', {
                squadName: squad.name
            }),
            remove: `${t('common.action.remove-users')} · ${squad.name}?\n${t('common.message.confirm-action-description')}`,
            delete: `${t('common.action.delete')} ${squad.name}?`
        }
        if (await confirmAction(confirmation[action])) void apply({ action, uuid: squad.uuid })
    }

    async function saveOrder(ids: string[]): Promise<void> {
        if (pending || tag || visible.length !== ordered.length) return
        await apply({ items: ids.map((uuid, viewPosition) => ({ uuid, viewPosition })) })
    }

    function copy(uuid: string): void {
        clipboardError = ''
        void navigator.clipboard.writeText(uuid).catch((error: unknown) => {
            clipboardError = error instanceof Error ? error.message : String(error)
        })
    }
</script>

<svelte:head><title>{t('constants.internal-squads')} | Gowave</title></svelte:head>

<main class="squads-page">
    {#if squads.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.internal-squads')} icon="TbCirclesRelation">
            <ActionIcon
                icon="TbQuestionMark"
                tone="lime"
                label={t('help-action-icon.shared.help-article')}
                onclick={() => (helpOpen = true)}
            />
            {#if ordered.length}
                <EntitySpotlight
                    icon="TbCirclesRelation"
                    items={ordered.map((squad) => ({
                        id: squad.uuid,
                        name: squad.name,
                        keywords: squad.tags,
                        select: () => (inboundSquad = squad.uuid)
                    }))}
                />
            {/if}
            <ActionIcon
                icon="TbRefresh"
                tone="cyan"
                label={t('common.action.update')}
                pending={squads.isFetching}
                onclick={() => void squads.refetch()}
            />
            <ActionIcon
                icon="TbPlus"
                tone="teal"
                label={t('common.action.create')}
                onclick={() => (creating = true)}
            />
        </PageHeader>
        {#if helpOpen}
            <HelpDrawer screen="PAGE_INTERNAL_SQUADS" close={() => (helpOpen = false)} />
        {/if}

        {#if squads.isError}
            <p role="alert">{squads.error.message}</p>
        {:else}
            <EntityTagBar items={ordered} active={tag} select={(value) => (tag = value)} />
            {#if !visible.length}
                <EmptyState
                    icon="PiEmpty"
                    label={t(
                        tag
                            ? 'common.message.nothing-found'
                            : 'internal-squads-grid.widget.no-internal-squads'
                    )}
                />
            {:else}
                <ul class="entity-grid squad-list">
                    {#each visible as squad (squad.uuid)}
                        {@const active = squad.info.membersCount > 0}
                        {@const reorderable = !tag && !pending && visible.length === ordered.length}
                        <EntityCard
                            name={squad.name}
                            icon="TbCirclesRelation"
                            {active}
                            tags={squad.tags}
                            badges={[
                                {
                                    icon: 'PiTag',
                                    label: t('common.field.inbounds'),
                                    value: squad.info.inboundsCount,
                                    tone: 'blue'
                                },
                                {
                                    icon: 'PiUsers',
                                    label: t('internal-squads-grid.widget.users'),
                                    value: squad.info.membersCount,
                                    tone: active ? 'teal' : 'gray'
                                }
                            ]}
                            label="Squad actions"
                            open={() => (inboundSquad = squad.uuid)}
                            reorder={reorderable ? drag(squad.uuid) : undefined}
                        >
                            {#snippet actions()}
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="teal"
                                    disabled={pending}
                                    onclick={() => perform('add', squad)}
                                >
                                    <Icon name="TbUsersPlus" size={18} />
                                    {t('common.action.add-all-users')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="red"
                                    disabled={pending || !active}
                                    onclick={() => perform('remove', squad)}
                                >
                                    <Icon name="TbUsersMinus" size={18} />
                                    {t('common.action.remove-users')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => (report = { kind: 'nodes', squad })}
                                >
                                    <Icon name="TbServerCog" size={18} />
                                    {t('internal-squad-card.widget.available-nodes')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => (report = { kind: 'usage', squad })}
                                >
                                    <Icon name="TbChartArcs" size={18} />
                                    {t('common.field.usage-stats')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => copy(squad.uuid)}
                                >
                                    <Icon name="PiCopy" size={18} />{t('common.action.copy-uuid')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => (renaming = squad)}
                                >
                                    <Icon name="PiPencil" size={18} />{t('common.action.rename')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => (editing = squad)}
                                >
                                    <Icon name="TbTags" size={18} />{t('common.field.tags')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="red"
                                    disabled={pending}
                                    onclick={() => perform('delete', squad)}
                                >
                                    <Icon name="PiTrashDuotone" size={18} />
                                    {t('internal-squads-grid.widget.delete-squad')}
                                </button>
                            {/snippet}
                        </EntityCard>
                    {/each}
                </ul>
            {/if}
        {/if}
        {#if clipboardError}
            <p role="alert">{clipboardError}</p>
        {/if}

        {#if failure}
            <p role="alert">{failure}</p>
        {/if}
        {#if creating}
            <CreateSquad
                kind="internal"
                close={() => (creating = false)}
                oncreated={(uuid) => (inboundSquad = uuid)}
            />
        {/if}
        {#if editing}
            <SquadEditor squad={editing} close={() => (editing = undefined)} />
        {/if}
        {#if inboundSquad}
            {#key inboundSquad}
                <SquadInbounds uuid={inboundSquad} close={() => (inboundSquad = undefined)} />
            {/key}
        {/if}
        {#if report}
            {#if report.kind === 'usage'}
                <SquadUsage
                    uuid={report.squad.uuid}
                    name={report.squad.name}
                    close={() => (report = undefined)}
                />
            {:else}
                <SquadNodes
                    uuid={report.squad.uuid}
                    name={report.squad.name}
                    close={() => (report = undefined)}
                />
            {/if}
        {/if}
    {/if}
    {#if renaming}
        {@const squad = renaming}
        <RenameDialog
            currentName={squad.name}
            rename={async (name) => {
                await submitCommand(UpdateInternalSquadCommand, { uuid: squad.uuid, name })
                await queryClient.invalidateQueries({ queryKey: squadsQuery.queryKey })
            }}
            close={() => (renaming = undefined)}
        />
    {/if}
</main>

<style>
    .squads-page {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
</style>
