<script lang="ts">
    import {
        AddUsersToExternalSquadCommand,
        CreateExternalSquadCommand,
        DeleteExternalSquadCommand,
        DeleteUsersFromExternalSquadCommand,
        ReorderExternalSquadCommand,
        UpdateExternalSquadCommand
    } from '@remnawave/backend-contract'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { cloneString } from '@shared/utils/misc/clone-string'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import CreateSquad from '../ui/CreateSquad.svelte'
    import EmptyState from '../ui/EmptyState.svelte'
    import EntityCard from '../ui/EntityCard.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import { applyEntityOrder, entityOrder } from '../ui/entity-order'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import RenameDialog from '../ui/RenameDialog.svelte'
    import { usersQueryKey } from '../users/queries'
    import { type ExternalSquad, externalSquadsKey, externalSquadsQuery } from './queries'

    type Action = 'add' | 'remove' | 'delete' | 'clone'
    const squads = createQuery(() => externalSquadsQuery)

    let t = $derived(locale.t)
    let renaming = $state<ExternalSquad>()
    let helpOpen = $state(false)
    let tag = $state('')
    let creating = $state(false)
    let ordered = $derived(squads.data?.externalSquads ?? [])
    const { drag, pendingOrder } = entityOrder(
        () => ordered,
        (items) => saveOrder(items.map(({ uuid }) => uuid))
    )
    let visible = $derived(
        applyEntityOrder(ordered, $pendingOrder).filter(
            (squad) => !tag || (tag === NO_TAG ? !squad.tags.length : squad.tags.includes(tag))
        )
    )
    let filtering = $derived(Boolean(tag) || visible.length !== ordered.length)

    const update = createMutation(() => ({
        mutationFn: async (
            change:
                | { items: { uuid: string; viewPosition: number }[] }
                | { action: Action; squad: ExternalSquad }
        ) => {
            if ('items' in change) {
                await submitCommand(ReorderExternalSquadCommand, change)
                return
            }
            const { action, squad } = change
            if (action === 'clone') {
                const created = await submitCommand(CreateExternalSquadCommand, {
                    name: cloneString(squad.name, 30, 'cl_')
                })
                try {
                    await submitCommand(UpdateExternalSquadCommand, {
                        ...squad,
                        uuid: created.uuid,
                        name: created.name,
                        subscriptionSettings: squad.subscriptionSettings ?? undefined,
                        hostOverrides: squad.hostOverrides ?? undefined
                    })
                } catch (error) {
                    try {
                        await deleteSquad(created.uuid)
                    } catch {
                        throw new Error(`Could not finish or remove the clone ${created.name}.`, {
                            cause: error
                        })
                    }
                    throw error
                }
                return
            }

            if (action === 'delete') {
                await deleteSquad(squad.uuid)
                return
            }

            const command =
                action === 'add'
                    ? AddUsersToExternalSquadCommand
                    : DeleteUsersFromExternalSquadCommand
            await executeCommand({ ...command, TSQ_url: command.url(squad.uuid) })
        },
        onSettled: async (_data, _error, change) => {
            await queryClient.invalidateQueries({ queryKey: externalSquadsKey })
            if ('action' in change && change.action !== 'clone') {
                await queryClient.invalidateQueries({ queryKey: usersQueryKey })
            }
        }
    }))

    function deleteSquad(uuid: string) {
        const command = DeleteExternalSquadCommand
        return executeCommand({ ...command, TSQ_url: command.url(uuid) })
    }

    function perform(action: Action, squad: ExternalSquad): void {
        const confirmations = {
            add: t('common.message.confirm-add-all-users-to-external-squad', {
                squadName: squad.name
            }),
            remove: `${t('common.action.remove-users')} · ${squad.name}?`,
            delete: `${t('common.action.delete')} ${squad.name}?`,
            clone: `${t('common.action.clone')} ${squad.name}?`
        }
        if (window.confirm(confirmations[action])) update.mutate({ action, squad })
    }

    async function saveOrder(items: string[]): Promise<void> {
        if (filtering || update.isPending) return
        await update.mutateAsync({
            items: items.map((uuid, viewPosition) => ({ uuid, viewPosition }))
        })
    }

    function openSquad(squad: ExternalSquad, editIdentity = false): void {
        void navigate('/dashboard/open/external-squad/:id', {
            params: { id: squad.uuid },
            search: editIdentity ? { edit: 'identity' } : {}
        })
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

<svelte:head><title>{t('constants.external-squads')} | Gowave</title></svelte:head>

<main class="squads-page">
    {#if squads.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.external-squads')} icon="TbWebhook">
            <ActionIcon
                icon="TbQuestionMark"
                tone="lime"
                label={t('help-action-icon.shared.help-article')}
                onclick={() => (helpOpen = true)}
            />
            {#if ordered.length}
                <EntitySpotlight
                    icon="TbWebhook"
                    items={ordered.map((squad) => ({
                        id: squad.uuid,
                        name: squad.name,
                        keywords: squad.tags,
                        select: () => openSquad(squad)
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
            <HelpDrawer screen="PAGE_EXTERNAL_SQUADS" close={() => (helpOpen = false)} />
        {/if}

        {#if squads.isError}
            <p role="alert">{squads.error.message}</p>
        {:else}
            <EntityTagBar items={ordered} active={tag} select={(value) => (tag = value)} />
            {#if !visible.length}
                <EmptyState
                    icon={filtering ? 'PiEmpty' : 'TbWebhook'}
                    description={filtering
                        ? undefined
                        : [
                              t('external-squads-grid.widget.no-external-squads-line-1'),
                              t('external-squads-grid.widget.no-external-squads-line-2'),
                              t('external-squads-grid.widget.no-external-squads-line-3')
                          ].join('\n')}
                    label={t(
                        tag
                            ? 'common.message.nothing-found'
                            : 'external-squads-grid.widget.no-external-squads'
                    )}
                />
            {:else}
                <ul class="entity-grid squad-list">
                    {#each visible as squad (squad.uuid)}
                        {@const active = squad.info.membersCount > 0}
                        {@const reorderable = !filtering && !update.isPending}
                        <EntityCard
                            name={squad.name}
                            icon="TbWebhook"
                            {active}
                            tags={squad.tags}
                            badges={[
                                {
                                    icon: 'PiUsers',
                                    label: t('external-squad-card.widget.users'),
                                    value: squad.info.membersCount,
                                    tone: active ? 'teal' : 'gray'
                                }
                            ]}
                            label="Squad actions"
                            href={p('/dashboard/open/external-squad/:id', {
                                params: { id: squad.uuid }
                            })}
                            reorder={reorderable ? drag(squad.uuid) : undefined}
                        >
                            {#snippet actions()}
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="teal"
                                    disabled={update.isPending}
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
                                    disabled={update.isPending || !active}
                                    onclick={() => perform('remove', squad)}
                                >
                                    <Icon name="TbUsersMinus" size={18} />
                                    {t('common.action.remove-users')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => copyUuid(squad.uuid)}
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
                                    onclick={() => openSquad(squad, true)}
                                >
                                    <Icon name="TbTags" size={18} />{t('common.field.tags')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    disabled={update.isPending}
                                    onclick={() => perform('clone', squad)}
                                >
                                    <Icon name="TbCopy" size={18} />{t('common.action.clone')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="red"
                                    disabled={update.isPending}
                                    onclick={() => perform('delete', squad)}
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
        {#if update.isError}
            <p role="alert">{update.error.message}</p>
        {/if}
    {/if}
    {#if creating}
        <CreateSquad
            kind="external"
            close={() => (creating = false)}
            oncreated={(id) =>
                void navigate('/dashboard/open/external-squad/:id', { params: { id } })}
        />
    {/if}
    {#if renaming}
        {@const squad = renaming}
        <RenameDialog
            currentName={squad.name}
            rename={async (name) => {
                await submitCommand(UpdateExternalSquadCommand, { uuid: squad.uuid, name })
                await queryClient.invalidateQueries({ queryKey: externalSquadsKey })
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
