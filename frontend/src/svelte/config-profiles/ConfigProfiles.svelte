<script lang="ts">
    import {
        CreateConfigProfileCommand,
        DeleteConfigProfileCommand,
        ReorderConfigProfileCommand,
        SetConfigProfileTagsCommand,
        UpdateConfigProfileCommand
    } from '@remnawave/backend-contract'
    import { instance } from '@shared/api/axios'
    import { handleRequestError } from '@shared/api/helpers/handler-request-error'
    import { ROUTES } from '@shared/constants/routes'
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { generateDefaultXrayConfig } from '@shared/utils/xray/default-config'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { ZodError } from 'zod'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import { navigate, p } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import ActionMenu from '../ui/ActionMenu.svelte'
    import Button from '../ui/Button.svelte'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import EmptyState from '../ui/EmptyState.svelte'
    import EntitySpotlight from '../ui/EntitySpotlight.svelte'
    import EntityTagBar from '../ui/EntityTagBar.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { downloadProfile } from './download-profile'
    import ProfilePreview from './ProfilePreview.svelte'
    import ProfileRelations from './ProfileRelations.svelte'
    import { type Profile, profileQuery, profilesQuery } from './queries'
    import Snippets from './Snippets.svelte'

    let showSnippets = $state(false)
    let relations = $state<{ uuid: string; mode: 'nodes' | 'inbounds' }>()
    let preview = $state<{ uuid: string; computed: boolean }>()
    let renameUuid = $state('')
    let renameDialog: HTMLDialogElement
    let renameName = $state('')
    let helpOpen = $state(false)
    let dragging = $state<string>()
    let clipboardError = $state('')
    let tag = $state('')
    let tagsDialog: HTMLDialogElement
    let tagDraft = $state('')
    let tagUuid = $state('')
    let tagsError = $state('')
    let savingTags = $state(false)
    let reordering = $state(false)
    let reorderError = $state('')
    let name = $state('')
    let creating = $state(false)
    let t = $derived(locale.t)

    const profiles = createQuery(() => profilesQuery)
    let ordered = $derived(profiles.data?.configProfiles ?? [])
    let relationsProfile = $derived(ordered.find((profile) => profile.uuid === relations?.uuid))
    let previewProfile = $derived(ordered.find((profile) => profile.uuid === preview?.uuid))
    let shown = $derived(
        ordered.filter(
            (profile) =>
                !tag || (tag === NO_TAG ? !profile.tags.length : profile.tags.includes(tag))
        )
    )

    async function reorderProfile(from: number, to: number): Promise<void> {
        if (
            reordering ||
            shown.length !== ordered.length ||
            !ordered[from] ||
            !ordered[to] ||
            from === to
        )
            return
        const ids = ordered.map(({ uuid }) => uuid)
        const [moved] = ids.splice(from, 1)
        if (!moved) return
        ids.splice(to, 0, moved)
        reordering = true
        reorderError = ''
        try {
            const response = await submitCommand(ReorderConfigProfileCommand, {
                items: ids.map((uuid, viewPosition) => ({ uuid, viewPosition }))
            })
            queryClient.setQueryData(profilesQuery.queryKey, response)
        } catch (error) {
            reorderError = error instanceof Error ? error.message : String(error)
        } finally {
            reordering = false
        }
    }

    function dropProfile(event: DragEvent, index: number): void {
        event.preventDefault()
        const from = ordered.findIndex((profile) => profile.uuid === dragging)
        dragging = undefined
        if (from !== -1) void reorderProfile(from, index)
    }

    function showPreview(uuid: string, computed: boolean): void {
        preview = { uuid, computed }
    }

    function copyUuid(uuid: string): void {
        clipboardError = ''
        void navigator.clipboard.writeText(uuid).catch((error: unknown) => {
            clipboardError = error instanceof Error ? error.message : t('common.message.error')
        })
    }

    function editTags(profile: (typeof ordered)[number]): void {
        tagUuid = profile.uuid
        tagDraft = profile.tags.join(', ')
        tagsError = ''
        tagsDialog.showModal()
    }

    async function saveTags(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (savingTags) return
        const tags = [
            ...new Set(
                tagDraft
                    .split(',')
                    .map((value) => value.trim().toUpperCase())
                    .filter(Boolean)
            )
        ]
        savingTags = true
        tagsError = ''
        try {
            await submitCommand(SetConfigProfileTagsCommand, { uuid: tagUuid, tags })
            await queryClient.invalidateQueries({ queryKey: ['configProfiles'] })
            if (tag && tag !== NO_TAG && !ordered.some((profile) => profile.tags.includes(tag)))
                tag = ''
            dismissDialog(tagsDialog)
        } catch (error) {
            tagsError =
                error instanceof ZodError
                    ? (error.issues[0]?.message ?? error.message)
                    : error instanceof Error
                      ? error.message
                      : String(error)
        } finally {
            savingTags = false
        }
    }

    const createProfile = createMutation(() => ({
        mutationFn: (value: string) =>
            submitCommand(CreateConfigProfileCommand, {
                name: value,
                config: generateDefaultXrayConfig()
            }),
        onSuccess: async (profile) => {
            creating = false
            await queryClient.invalidateQueries({ queryKey: profilesQuery.queryKey })
            await navigate(ROUTES.DASHBOARD.MANAGEMENT.CONFIG_PROFILE_BY_UUID, {
                params: { uuid: profile.uuid }
            })
        }
    }))

    const renameProfile = createMutation(() => ({
        mutationFn: ({ uuid, name }: { uuid: string; name: string }) =>
            submitCommand(UpdateConfigProfileCommand, { uuid, name }),
        onSuccess: async (profile) => {
            queryClient.setQueryData(profileQuery(profile.uuid).queryKey, profile)
            await queryClient.invalidateQueries({ queryKey: ['configProfiles'] })
            dismissDialog(renameDialog)
        }
    }))

    function startRename(profile: Profile): void {
        renameProfile.reset()
        renameUuid = profile.uuid
        renameName = profile.name
        renameDialog.showModal()
    }

    function rename(event: SubmitEvent): void {
        event.preventDefault()
        if (!renameProfile.isPending && renameName.trim()) {
            renameProfile.mutate({ uuid: renameUuid, name: renameName.trim() })
        }
    }

    const removeProfile = createMutation(() => ({
        mutationFn: (uuid: string) => {
            const command = DeleteConfigProfileCommand
            const params = command.RequestParamSchema.parse({ uuid })
            return instance
                .request({
                    url: command.url(params.uuid),
                    method: command.endpointDetails.REQUEST_METHOD
                })
                .catch(handleRequestError)
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: profilesQuery.queryKey })
    }))

    function startCreate(): void {
        createProfile.reset()
        name = ''
        creating = true
    }

    function submit(event: SubmitEvent): void {
        event.preventDefault()
        if (!createProfile.isPending) createProfile.mutate(name.trim())
    }
</script>

<svelte:head><title>{t('constants.config-profiles')} | Gowave</title></svelte:head>

<main class="profiles-page">
    {#if profiles.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.config-profiles')} icon="XrayLogo">
            <ActionIcon
                icon="TbQuestionMark"
                tone="lime"
                label={t('help-action-icon.shared.help-article')}
                onclick={() => (helpOpen = true)}
            />
            <EntitySpotlight
                icon="XrayLogo"
                items={ordered.map(({ uuid, name, inbounds, nodes }) => ({
                    id: uuid,
                    name,
                    badges: [
                        { icon: 'PiTag', count: inbounds.length, tone: 'blue' },
                        { icon: 'PiCpu', count: nodes.length, tone: nodes.length ? 'teal' : 'gray' }
                    ],
                    href: p(ROUTES.DASHBOARD.MANAGEMENT.CONFIG_PROFILE_BY_UUID, {
                        params: { uuid }
                    })
                }))}
            />
            <ActionIcon
                icon="TbCode"
                tone="teal"
                label="Xray snippets"
                onclick={() => (showSnippets = true)}
            />
            <ActionIcon
                icon="TbRefresh"
                tone="cyan"
                label={t('common.action.update')}
                disabled={profiles.isFetching}
                onclick={() => void profiles.refetch()}
            />
            <ActionIcon
                icon="TbPlus"
                tone="teal"
                label={t('common.action.create')}
                onclick={startCreate}
            />
        </PageHeader>
        {#if helpOpen}
            <HelpDrawer screen="PAGE_CONFIG_PROFILES" close={() => (helpOpen = false)} />
        {/if}
        <EntityTagBar
            ready={!profiles.isPending}
            items={ordered}
            active={tag}
            select={(value) => (tag = value)}
        />
        {#if profiles.isError}
            <p role="alert">{profiles.error.message}</p>
        {:else if !shown.length}
            <EmptyState icon="XrayLogo" label={t('common.message.nothing-found')} />
        {:else}
            <ul class="profile-list">
                {#each shown as profile, index (profile.uuid)}
                    {@const canReorder =
                        !tag && ordered.length > 1 && ordered.length === shown.length}
                    <li
                        class="profile-row"
                        data-active={profile.nodes.length > 0}
                        ondragover={(event) => {
                            if (dragging && canReorder) event.preventDefault()
                        }}
                        ondrop={(event) => dropProfile(event, index)}
                    >
                        <span class="profile-icon"><Icon name="XrayLogo" size={22} /></span>
                        <div class="profile-identity">
                            <h2>
                                <a
                                    href={p(ROUTES.DASHBOARD.MANAGEMENT.CONFIG_PROFILE_BY_UUID, {
                                        params: { uuid: profile.uuid }
                                    })}
                                    >{profile.name}</a
                                >
                            </h2>
                            {#if profile.tags.length}
                                <div class="profile-tags">{profile.tags.join(', ')}</div>
                            {/if}
                            <div class="profile-counts">
                                <button
                                    type="button"
                                    class="count-button blue mantine-UnstyledButton-root"
                                    aria-label={`${t('common.field.inbounds')} (${profile.inbounds.length})`}
                                    title={t('common.field.inbounds')}
                                    onclick={() =>
                                        (relations = { uuid: profile.uuid, mode: 'inbounds' })}
                                >
                                    <Icon name="PiTag" size={12} />{profile.inbounds.length}
                                </button>
                                <button
                                    type="button"
                                    class="count-button mantine-UnstyledButton-root"
                                    class:teal={profile.nodes.length > 0}
                                    aria-label={`${t('constants.nodes')} (${profile.nodes.length})`}
                                    title={t('constants.nodes')}
                                    onclick={() =>
                                        (relations = { uuid: profile.uuid, mode: 'nodes' })}
                                >
                                    <Icon name="PiCpu" size={12} />{profile.nodes.length}
                                </button>
                            </div>
                        </div>
                        <div class="profile-actions">
                            <ActionMenu label="Profile actions" size={28} width={220}>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => showPreview(profile.uuid, false)}
                                >
                                    <Icon name="TbEye" size={18} />
                                    {t('config-profiles-grid.widget.quick-view')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => showPreview(profile.uuid, true)}
                                >
                                    <Icon name="TbCpu2" size={18} />
                                    {t('config-profile-card.widget.view-computed')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() =>
                                        downloadProfile(
                                            profile.name,
                                            JSON.stringify(profile.config, null, 2)
                                        )}
                                >
                                    <Icon name="TbDownload" size={18} />
                                    {t('common.action.download')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    onclick={() => copyUuid(profile.uuid)}
                                >
                                    <Icon name="PiCopy" size={18} />{t('common.action.copy-uuid')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    disabled={reordering}
                                    onclick={() => startRename(profile)}
                                >
                                    <Icon name="PiPencil" size={18} />{t('common.action.rename')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    disabled={reordering}
                                    onclick={() => editTags(profile)}
                                >
                                    <Icon name="TbTags" size={18} />{t('common.field.tags')}
                                </button>
                                <button
                                    type="button"
                                    role="menuitem"
                                    tabindex="-1"
                                    data-tone="red"
                                    disabled={removeProfile.isPending || reordering}
                                    onclick={() => {
                                        if (
                                            window.confirm(
                                                `${t('common.action.delete')} ${profile.name}?`
                                            )
                                        ) {
                                            removeProfile.mutate(profile.uuid)
                                        }
                                    }}
                                >
                                    <Icon name="PiTrashDuotone" size={18} />
                                    {t('common.action.delete')}
                                </button>
                            </ActionMenu>
                            <button
                                type="button"
                                class="profile-grip mantine-UnstyledButton-root"
                                aria-label={`Reorder ${profile.name}`}
                                title="Drag to reorder"
                                draggable={canReorder && !reordering}
                                ondragstart={(event) => {
                                    dragging = profile.uuid
                                    event.dataTransfer?.setData('text/plain', profile.uuid)
                                }}
                                ondragend={() => (dragging = undefined)}
                            >
                                <Icon name="TbGripVertical" size={16} />
                            </button>
                        </div>
                    </li>
                {/each}
            </ul>
        {/if}
        {#if clipboardError}
            <p role="alert">{clipboardError}</p>
        {/if}
        {#if reorderError}
            <p role="alert">{reorderError}</p>
        {/if}
        {#if removeProfile.isError}
            <p role="alert">{removeProfile.error.message}</p>
        {/if}
        {#if relations && relationsProfile}
            <ProfileRelations
                profile={relationsProfile}
                mode={relations.mode}
                close={() => (relations = undefined)}
            />
        {/if}
        {#if preview && previewProfile}
            <ProfilePreview
                profile={previewProfile}
                computed={preview.computed}
                close={() => (preview = undefined)}
            />
        {/if}
        {#if showSnippets}
            <Snippets close={() => (showSnippets = false)} />
        {/if}
    {/if}
</main>

{#if creating}
    <Overlay
        title={t('common.field.creation')}
        icon="TbNewSection"
        tone="teal"
        width={440}
        fullscreenOnMobile={false}
        preventClose={createProfile.isPending}
        close={() => (creating = false)}
    >
        <form class="create-profile" onsubmit={submit}>
            <p class="create-description">
                {t(
                    'config-profiles-header-action-buttons.feature.create-a-new-config-profile-by-entering-a-name-below'
                )}<br>
                {t(
                    'config-profiles-header-action-buttons.feature.you-can-customize-xray-config-after-creation'
                )}
            </p>
            <label class="create-label">
                <span
                    >{t('config-profiles-header-action-buttons.feature.profile-name')}
                    <span class="required">*</span></span
                >
                <input
                    bind:value={name}
                    name="name"
                    required
                    data-autofocus
                    placeholder={t(
                        'config-profiles-header-action-buttons.feature.enter-profile-name'
                    )}
                >
            </label>
            {#if createProfile.isError}
                <p role="alert">
                    {createProfile.error instanceof ZodError
                        ? createProfile.error.issues[0]?.message
                        : createProfile.error.message}
                </p>
            {/if}
            <div class="button-row">
                <Button
                    color="gray"
                    variant="light"
                    onclick={() => (creating = false)}
                    disabled={createProfile.isPending}
                >
                    {t('common.action.cancel')}
                </Button>
                <Button
                    type="submit"
                    color="teal"
                    variant="light"
                    disabled={createProfile.isPending}
                >
                    {t('common.action.create')}
                </Button>
            </div>
        </form>
    </Overlay>
{/if}

<dialog
    bind:this={tagsDialog}
    aria-labelledby="profile-tags-title"
    oncancel={(event) => cancelDialog(event, savingTags)}
>
    <h2 id="profile-tags-title">{t('common.field.tags')}</h2>
    <form onsubmit={(event) => void saveTags(event)}>
        <label>
            {t('common.field.tags')}
            <input name="tags" bind:value={tagDraft} placeholder="TAG1, TAG2" disabled={savingTags}>
        </label>
        {#if tagsError}
            <p role="alert">{tagsError}</p>
        {/if}
        <div class="button-row">
            <button type="button" disabled={savingTags} onclick={() => dismissDialog(tagsDialog)}>
                {t('common.action.cancel')}
            </button>
            <button type="submit" disabled={savingTags}>{t('common.action.save')}</button>
        </div>
    </form>
</dialog>

<dialog
    bind:this={renameDialog}
    aria-labelledby="profile-rename-title"
    oncancel={(event) => cancelDialog(event, renameProfile.isPending)}
>
    <h2 id="profile-rename-title">{t('common.action.rename')}</h2>
    <form onsubmit={rename}>
        <label>
            {t('common.field.name')}
            <input name="name" bind:value={renameName} required disabled={renameProfile.isPending}>
        </label>
        {#if renameProfile.error}
            <p role="alert">{renameProfile.error.message}</p>
        {/if}
        <div class="button-row">
            <button
                type="button"
                disabled={renameProfile.isPending}
                onclick={() => dismissDialog(renameDialog)}
            >
                {t('common.action.close')}
            </button>
            <button type="submit" disabled={renameProfile.isPending || !renameName.trim()}>
                {t('common.action.save')}
            </button>
        </div>
    </form>
</dialog>

<style>
    .profiles-page {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-md);
    }

    .profile-list {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        align-items: start;
        gap: var(--mantine-spacing-xs);
        padding: 4px var(--mantine-spacing-md) 0;
        margin: 0;
        list-style: none;
    }

    @media (min-width: 48em) {
        .profile-list {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }

    .profile-row {
        position: relative;
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        min-height: 46px;
        padding: 6px 10px 6px 12px;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
        cursor: pointer;
    }

    .profile-row::before {
        position: absolute;
        top: 8px;
        bottom: 8px;
        inset-inline-start: 0;
        width: 2px;
        border-radius: 0 2px 2px 0;
        background: var(--mantine-color-dark-4);
        content: "";
    }

    .profile-row[data-active="true"]::before {
        background: var(--mantine-color-cyan-5);
    }

    .profile-icon {
        display: inline-grid;
        width: 32px;
        height: 32px;
        flex-shrink: 0;
        place-items: center;
        border: 1px solid rgb(134 142 150 / 30%);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(135deg, rgb(134 142 150 / 15%), rgb(73 80 87 / 10%));
        color: var(--mantine-color-text);
    }

    .profile-row[data-active="true"] .profile-icon {
        border-color: rgb(18 184 134 / 30%);
        background: linear-gradient(135deg, rgb(18 184 134 / 15%), rgb(12 166 120 / 10%));
        color: var(--mantine-color-teal-4);
    }

    .profile-identity {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        flex: 1 1 auto;
        min-width: 0;
    }

    @media (max-width: 48em) {
        .profile-identity {
            flex-wrap: wrap;
            row-gap: 5px;
        }

        .profile-identity h2 {
            flex: 1 0 100%;
        }
    }

    h2 {
        flex: 1;
        margin: 0;
        overflow: hidden;
        font: 700 var(--mantine-font-size-md) / 22px var(--mantine-font-family-monospace);
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    h2 a {
        color: var(--mantine-color-bright);
        text-decoration: none;
    }
    h2 a::after {
        position: absolute;
        inset: 0;
        content: "";
    }

    .profile-tags {
        overflow: hidden;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .profile-counts {
        position: relative;
        z-index: 1;
        display: flex;
        flex-shrink: 0;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }

    .profile-actions {
        position: relative;
        z-index: 1;
        display: flex;
        flex-shrink: 0;
        gap: 6px;
    }

    .count-button {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        height: 26px;
        padding-inline: 12px;
        border: 1px solid rgb(134 142 150 / 30%);
        border-radius: var(--mantine-radius-md);
        background: rgb(134 142 150 / 8%);
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
    }

    .count-button.blue {
        border-color: rgb(34 139 230 / 30%);
        background: rgb(34 139 230 / 10%);
        color: var(--mantine-color-blue-4);
    }

    .count-button.teal {
        border-color: rgb(18 184 134 / 30%);
        background: rgb(18 184 134 / 10%);
        color: var(--mantine-color-teal-4);
    }

    .profile-grip {
        display: grid;
        place-items: center;
        border: 0;
        padding: 0;
        background: transparent;
    }

    .profile-grip[draggable="true"] {
        cursor: grab;
    }

    .create-profile {
        gap: var(--mantine-spacing-md);
    }
    .create-description {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
    }
    .create-label {
        gap: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-md);
    }
    .create-label input {
        width: 100%;
        height: 36px;
        min-height: 36px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 400;
    }
    .create-label input:focus-visible {
        outline: none;
        border-color: var(--mantine-primary-color-filled);
    }
    .required {
        color: var(--mantine-color-error);
    }
    .create-profile .button-row {
        justify-content: flex-end;
        gap: var(--mantine-spacing-md);
    }

    form,
    label {
        display: grid;
        gap: 0.5rem;
    }
</style>
