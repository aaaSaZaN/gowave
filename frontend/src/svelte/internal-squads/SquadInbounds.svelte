<script lang="ts">
    import { UpdateInternalSquadCommand } from '@remnawave/backend-contract'
    import { buildOpenEntityUrl, OPEN_ENTITY } from '@shared/constants/routes'
    import { fixedVirtualWindow } from '@shared/lib/virtual-layout'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { untrack } from 'svelte'
    import { MediaQuery, SvelteSet } from 'svelte/reactivity'
    import { slide } from 'svelte/transition'
    import { submitCommand } from '../commands'
    import ProfileRelations from '../config-profiles/ProfileRelations.svelte'
    import { type Profile, profilesQuery } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import JsonTree from '../ui/JsonTree.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { internalSquadQuery, squadsQuery } from './queries'

    const reducedMotion = new MediaQuery('(prefers-reduced-motion: reduce)')

    let { uuid, close }: { uuid: string; close: () => void } = $props()
    const t = $derived(locale.t)
    const title = $derived(t('internal-squads.drawer.widget.edit-internal-squad'))
    const profiles = createQuery(() => profilesQuery)
    const squad = createQuery(() => internalSquadQuery(uuid))
    const selected = new SvelteSet<string>()
    const expanded = new SvelteSet<string>()
    const tabs = [
        ['profiles', 'PiTreeStructureDuotone', 'internal-squads.drawer.widget.config-profiles'],
        ['flat', 'PiList', 'internal-squads.drawer.widget.flat-list']
    ] as const
    const filters = [
        ['all', 'common.field.all'],
        ['selected', 'internal-squads.drawer.widget.selected'],
        ['unselected', 'internal-squads.drawer.widget.unselected']
    ] as const
    type Inbound = Profile['inbounds'][number]

    let initialized = false
    let search = $state('')
    let tab = $state<(typeof tabs)[number][0]>('profiles')
    let filter = $state<(typeof filters)[number][0]>('all')
    let copied = $state<'link' | 'uuid'>()
    let copyError = $state('')
    let nodes = $state<Profile>()
    let preview = $state<{ title: string; value: unknown }>()
    let flatList = $state<HTMLDivElement>()
    let height = $state(0)
    let offset = $state(0)
    const flatRowHeight = 60
    const matching = $derived.by(() => {
        const term = search.trim().toLowerCase()
        return (profiles.data?.configProfiles ?? []).flatMap((profile) => {
            if (!term || profile.name.toLowerCase().includes(term)) return [profile]
            const inbounds = profile.inbounds.filter(
                ({ tag, type }) =>
                    tag.toLowerCase().includes(term) || type.toLowerCase().includes(term)
            )
            return inbounds.length ? [{ ...profile, inbounds }] : []
        })
    })
    const flat = $derived(
        matching.flatMap((profile) =>
            profile.inbounds
                .filter(
                    ({ uuid }) => filter === 'all' || selected.has(uuid) === (filter === 'selected')
                )
                .map((inbound) => ({ inbound, profile: profile.name }))
        )
    )
    const visible = $derived(fixedVirtualWindow(flat.length, flatRowHeight, offset, height))

    $effect(() => {
        const data = squad.data
        if (!data || initialized) return
        initialized = true
        untrack(() => {
            for (const inbound of data.inbounds) selected.add(inbound.uuid)
        })
    })

    const save = createMutation(() => ({
        mutationFn: () =>
            submitCommand(UpdateInternalSquadCommand, { uuid, inbounds: [...selected] }),
        onSuccess: async (updated) => {
            queryClient.setQueryData(internalSquadQuery(updated.uuid).queryKey, updated)
            await Promise.all([
                queryClient.invalidateQueries({ queryKey: squadsQuery.queryKey }),
                queryClient.invalidateQueries({
                    queryKey: ['configProfiles', 'getInboundsByProfile']
                })
            ])
            close()
        }
    }))

    function resetScroll(): void {
        offset = 0
        if (flatList) flatList.scrollTop = 0
    }

    function toggle(inbound: Inbound): void {
        if (selected.has(inbound.uuid)) selected.delete(inbound.uuid)
        else selected.add(inbound.uuid)
    }

    async function copy(kind: 'link' | 'uuid'): Promise<void> {
        copyError = ''
        try {
            await navigator.clipboard.writeText(
                kind === 'link' ? buildOpenEntityUrl(OPEN_ENTITY.INTERNAL_SQUAD, uuid) : uuid
            )
            copied = kind
        } catch (reason) {
            copyError = reason instanceof Error ? reason.message : String(reason)
        }
    }
</script>

{#snippet badge(
    value: string | number,
    tone: string,
    label?: string,
    icon?: IconName,
    soft = false
)}
    <span
        class="badge"
        class:soft
        title={label}
        style:--badge-color={`var(--mantine-color-${tone}-light-color)`}
        style:--badge-tint={`var(--mantine-color-${tone}-6)`}
    >
        {#if icon}
            <Icon name={icon} size={12} />
        {/if}
        {value}
    </span>
{/snippet}

{#snippet inboundCard(inbound: Inbound, profileName?: string)}
    <div
        class="inbound-card"
        class:checked={selected.has(inbound.uuid)}
        class:flat-card={profileName !== undefined}
    >
        <label>
            <input
                type="checkbox"
                checked={selected.has(inbound.uuid)}
                onchange={() => toggle(inbound)}
            >
            <span class="inbound-name">
                <strong title={inbound.tag}>{inbound.tag}</strong>
                {#if profileName}
                    <small>{profileName}</small>
                {/if}
            </span>
        </label>
        <span class="inbound-actions">
            {@render badge(inbound.type.toUpperCase(), 'gray', undefined, undefined, true)}
            {#if inbound.port}
                {@render badge(inbound.port, 'teal', undefined, undefined, true)}
            {/if}
            <ActionIcon
                icon="TbCode"
                tone="cyan"
                size={28}
                iconSize={16}
                label={t('flat-inbound-checkbox-card.shared.inbound-config-inbound-tag', {
                    inboundTag: inbound.tag
                })}
                onclick={() =>
                    (preview = {
                        title: t('flat-inbound-checkbox-card.shared.inbound-config-inbound-tag', {
                            inboundTag: inbound.tag
                        }),
                        value: inbound.rawInbound
                    })}
            />
        </span>
    </div>
{/snippet}

<Overlay
    {title}
    icon="TbCirclesRelation"
    tone="teal"
    drawer
    width={500}
    class="squad-inbounds"
    inlineActions
    {close}
>
    {#snippet actions()}
        <ActionIcon
            icon={copied === 'link' ? 'PiCheck' : 'TbLink'}
            size={28}
            iconSize={16}
            subtle
            label={t('common.action.copy-link')}
            onclick={() => void copy('link')}
        />
    {/snippet}
    <div class="squad-panel" tabindex="-1" data-autofocus>
        {#if squad.isPending || profiles.isPending}
            <RouteLoading compact />
        {:else if squad.error || profiles.error}
            <p role="alert">{squad.error?.message || profiles.error?.message}</p>
        {:else if squad.data}
            {@const active = squad.data.info.membersCount > 0}
            <section class="summary">
                <span class="squad-icon" class:active
                    ><Icon name="TbCirclesRelation" size={28} /></span
                >
                <div class="summary-copy">
                    <h3 title={squad.data.name}>{squad.data.name}</h3>
                    <div class="counts">
                        {@render badge(
                            squad.data.info.membersCount.toLocaleString('en-US'),
                            active ? 'teal' : 'gray',
                            t('internal-squads-with-store.drawer.widget.users'),
                            'PiUsers'
                        )}
                        {@render badge(
                            selected.size.toLocaleString('en-US'),
                            'blue',
                            t('common.field.inbounds'),
                            'PiTag'
                        )}
                        <ActionIcon
                            icon={copied === 'uuid' ? 'PiCheck' : 'PiCopy'}
                            size={34}
                            iconSize={18}
                            subtle
                            label={t('common.action.copy-uuid')}
                            onclick={() => void copy('uuid')}
                        />
                    </div>
                </div>
                <ActionIcon
                    icon="TbDeviceFloppy"
                    tone="teal"
                    size={44}
                    label={t('common.action.save')}
                    disabled={!selected.size || save.isPending}
                    onclick={() => save.mutate()}
                />
            </section>
            {#if save.error || copyError}
                <p role="alert">{save.error?.message || copyError}</p>
            {/if}
            <label class="search">
                <span><Icon name="TbSearch" size={16} /></span>
                <input
                    type="search"
                    bind:value={search}
                    aria-label={t('common.message.search-profiles-or-inbounds')}
                    placeholder={t('common.message.search-profiles-or-inbounds')}
                    oninput={resetScroll}
                >
            </label>
            <fieldset class="view-tabs" aria-label={t('common.field.inbounds')}>
                {#each tabs as [id, icon, label] (id)}
                    <button
                        class="mantine-UnstyledButton-root"
                        type="button"
                        aria-pressed={tab === id}
                        onclick={() => {
                            tab = id
                            resetScroll()
                        }}
                    >
                        <Icon name={icon} size={18} />{t(label)}
                    </button>
                {/each}
            </fieldset>
            {#if tab === 'profiles'}
                <div class="profiles">
                    {#each matching as profile (profile.uuid)}
                        {@const selectedCount = profile.inbounds.filter(({ uuid }) =>
                            selected.has(uuid)
                        ).length}
                        {@const open = expanded.has(profile.uuid)}
                        <article class="profile-card">
                            <div class="profile-heading">
                                <button
                                    class="mantine-UnstyledButton-root profile-toggle"
                                    type="button"
                                    aria-expanded={open}
                                    onclick={() =>
                                        open
                                            ? expanded.delete(profile.uuid)
                                            : expanded.add(profile.uuid)}
                                >
                                    <span class="chevron"
                                        ><Icon name="TbChevronDown" size={16} /></span
                                    >
                                    <strong>{profile.name}</strong>
                                </button>
                                <div class="profile-badges">
                                    {@render badge(
                                        `${selectedCount} / ${profile.inbounds.length}`,
                                        selectedCount === profile.inbounds.length ? 'teal' : 'cyan',
                                        t('common.field.inbounds'),
                                        'PiCheck',
                                        true
                                    )}
                                    <button
                                        class="mantine-UnstyledButton-root node-count"
                                        type="button"
                                        aria-label={t('config-profile-card.shared.active-on-nodes')}
                                        onclick={() => (nodes = profile)}
                                    >
                                        {@render badge(
                                            profile.nodes.length,
                                            profile.nodes.length ? 'teal' : 'cyan',
                                            t('config-profile-card.shared.active-on-nodes'),
                                            'PiCpu',
                                            true
                                        )}
                                    </button>
                                </div>
                                <div class="profile-actions">
                                    <ActionIcon
                                        icon="PiCheck"
                                        size={34}
                                        iconSize={16}
                                        subtle
                                        label="Select all inbounds"
                                        onclick={() => {
                                            for (const { uuid } of profile.inbounds)
                                                selected.add(uuid)
                                        }}
                                    />
                                    <ActionIcon
                                        icon="PiX"
                                        size={34}
                                        iconSize={16}
                                        subtle
                                        label="Unselect all inbounds"
                                        onclick={() => {
                                            for (const { uuid } of profile.inbounds)
                                                selected.delete(uuid)
                                        }}
                                    />
                                    <ActionIcon
                                        icon="XrayLogo"
                                        size={34}
                                        iconSize={16}
                                        subtle
                                        label={t('config-profiles-grid.widget.xray-config')}
                                        onclick={() =>
                                            (preview = {
                                                title: profile.name,
                                                value: profile.config
                                            })}
                                    />
                                </div>
                            </div>
                            {#if open}
                                <div
                                    class="profile-inbounds"
                                    transition:slide={{ duration: reducedMotion.current ? 0 : 200 }}
                                >
                                    {#each profile.inbounds as inbound (inbound.uuid)}
                                        {@render inboundCard(inbound)}
                                    {/each}
                                </div>
                            {/if}
                        </article>
                    {:else}
                        <p class="empty">{t('common.message.no-profiles-or-inbounds-found')}</p>
                    {/each}
                </div>
            {:else}
                <fieldset class="filters" aria-label={t('common.field.inbounds')}>
                    {#each filters as [id, label] (id)}
                        <label
                            ><input
                                type="radio"
                                bind:group={filter}
                                value={id}
                                onchange={resetScroll}
                            ><span>{t(label)}</span></label
                        >
                    {/each}
                </fieldset>
                {#if flat.length}
                    <div
                        class="flat-list"
                        bind:this={flatList}
                        bind:clientHeight={height}
                        onscroll={(event) => (offset = event.currentTarget.scrollTop)}
                    >
                        <div class="flat-space" style:height={`${flat.length * flatRowHeight}px`}>
                            {#each flat.slice(visible.from, visible.to) as item, index (item.inbound
                                .uuid)}
                                <div
                                    class="flat-item"
                                    style:inset-block-start={`${(visible.from + index) * flatRowHeight}px`}
                                >
                                    {@render inboundCard(item.inbound, item.profile)}
                                </div>
                            {/each}
                        </div>
                    </div>
                {:else}
                    <p class="empty flat-empty">
                        {t('virtualized-flat-inbounds-list.shared.no-inbounds-found')}
                    </p>
                {/if}
            {/if}
        {/if}
    </div>
</Overlay>

{#if nodes}
    <ProfileRelations profile={nodes} mode="nodes" close={() => (nodes = undefined)} />
{/if}
{#if preview}
    <Overlay
        title={preview.title}
        icon="TbCode"
        tone="teal"
        width={780}
        fullscreenOnMobile={false}
        close={() => (preview = undefined)}
    >
        <JsonTree value={preview.value} indent={4} />
    </Overlay>
{/if}

<style>
    :global(dialog.squad-inbounds .overlay-header) {
        padding-block-start: calc(var(--mantine-spacing-sm) + 1px);
    }
    :global(dialog.squad-inbounds .drawer-content) {
        height: 100%;
        display: flex;
        flex-direction: column;
    }
    :global(dialog.squad-inbounds .drawer-body) {
        display: flex;
        flex: 1;
        min-height: 0;
    }
    .squad-panel {
        display: flex;
        flex: 1;
        flex-direction: column;
        min-width: 0;
        gap: var(--mantine-spacing-md);
        outline: none;
    }
    .summary {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-md);
        border: 1px solid rgb(255 255 255 / 8%);
        border-radius: var(--mantine-radius-md);
        background: rgb(255 255 255 / 2%);
        box-shadow: var(--mantine-shadow-sm);
    }
    .summary-copy {
        flex: 1;
        min-width: 0;
    }
    .summary h3,
    .profile-toggle strong {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-family: var(--mantine-font-family-monospace);
    }
    .summary h3 {
        margin: 0 0 6px;
        font-size: var(--mantine-font-size-lg);
        line-height: var(--mantine-line-height-lg);
    }
    .squad-icon {
        display: grid;
        place-items: center;
        flex: 0 0 44px;
        height: 44px;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-gray-light-color);
        background: var(--mantine-color-dark-6);
        box-shadow: 0 4px 6px -1px rgb(0 0 0 / 20%);
    }
    .squad-icon.active {
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
    }
    .counts,
    .profile-badges,
    .profile-actions,
    .inbound-actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .badge {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        padding-inline: 10px;
        min-height: 26px;
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--badge-tint) 12%, transparent);
        color: var(--badge-color);
        font: 700 var(--mantine-font-size-sm) / 1 var(--mantine-font-family-monospace);
        white-space: nowrap;
    }
    .badge.soft {
        min-height: 20px;
        padding-inline: 8px;
        border: 1px solid color-mix(in srgb, var(--badge-tint) 30%, transparent);
        font-size: var(--mantine-font-size-xs);
    }
    .search {
        position: relative;
    }
    .search input {
        width: 100%;
        height: 36px;
        min-height: 36px;
        padding-inline-start: 36px;
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
    }
    .search > span {
        position: absolute;
        inset-block: 0;
        inset-inline-start: 0;
        display: grid;
        place-items: center;
        width: 36px;
        color: var(--mantine-color-dimmed);
        pointer-events: none;
    }
    .view-tabs {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 8px;
        margin: 0;
        padding: 0;
        min-width: 0;
        border: 0;
    }
    .view-tabs > button {
        position: relative;
        display: flex;
        align-items: center;
        justify-content: center;
        min-height: 40px;
        padding-inline: 36px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid var(--gw-border);
        background: var(--gw-card-bg);
        color: inherit;
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }
    .view-tabs > button[aria-pressed="true"] {
        border-color: color-mix(in srgb, var(--mantine-color-cyan-6) 30%, transparent);
        background: var(--mantine-color-cyan-light);
        color: var(--mantine-color-cyan-4);
    }
    .view-tabs :global(svg) {
        position: absolute;
        inset-inline-start: 16px;
    }
    .profiles,
    .flat-list {
        flex: 1;
        min-height: 0;
        overflow: auto;
    }
    .profiles {
        margin-block-start: -4px;
    }
    .profile-card {
        margin-block-end: 8px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid rgb(255 255 255 / 8%);
        background: rgb(255 255 255 / 2%);
    }
    .profile-heading {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        padding-inline-end: var(--mantine-spacing-xs);
    }
    .profile-toggle {
        grid-area: 1 / 1 / 3 / 2;
        display: grid;
        grid-template-columns: 16px minmax(0, 1fr);
        gap: 16px;
        min-width: 0;
        padding: 12px 16px 42px;
        font-size: var(--mantine-font-size-md);
        line-height: var(--mantine-line-height-md);
        text-align: start;
    }
    .chevron {
        position: relative;
        inset-block-start: 16px;
    }
    @media (prefers-reduced-motion: no-preference) {
        .chevron {
            transition: transform 200ms ease;
        }
    }
    .profile-toggle[aria-expanded="true"] .chevron {
        transform: rotate(180deg);
    }
    .profile-badges {
        grid-area: 2 / 1;
        align-self: end;
        justify-self: start;
        margin: 0 0 12px 48px;
        z-index: 1;
    }
    .node-count {
        display: flex;
    }
    .profile-actions {
        grid-area: 1 / 2 / 3 / 3;
        align-self: center;
        gap: 0;
    }
    .profile-inbounds {
        display: grid;
        gap: 6px;
        padding: 12px 16px;
        border-top: 1px solid var(--gw-border);
    }
    .inbound-card {
        display: flex;
        align-items: center;
        min-height: 56px;
        gap: var(--mantine-spacing-xs);
        padding: 7px 12px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
    }
    .inbound-card.checked {
        border-color: var(--mantine-primary-color-filled);
        background: light-dark(
            var(--mantine-primary-color-light),
            var(--mantine-primary-color-dark-color)
        );
    }
    .flat-card {
        padding: 8px 12px;
        border-color: light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4));
        background: var(--mantine-color-body);
    }
    .flat-card.checked {
        background: light-dark(
            color-mix(in srgb, var(--mantine-primary-color-filled) 10%, var(--mantine-color-body)),
            var(--mantine-color-body)
        );
    }
    .flat-card :global(.badge.soft) {
        min-height: 16px;
        padding-inline: 7px;
        font-size: 10px;
    }
    .inbound-card label {
        display: flex;
        align-items: center;
        flex: 1;
        min-width: 0;
        gap: var(--mantine-spacing-xs);
        cursor: pointer;
    }
    .inbound-card input {
        flex: 0 0 auto;
        width: 16px;
        height: 16px;
        min-height: 0;
        accent-color: var(--mantine-primary-color-filled);
    }
    .flat-card input {
        width: 20px;
        height: 20px;
        margin: 0;
    }
    .inbound-name {
        display: grid;
        min-width: 0;
        gap: 2px;
        font-size: var(--mantine-font-size-sm);
    }
    .inbound-name strong,
    .inbound-name small {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .inbound-name strong {
        font-family: var(--mantine-font-family-monospace);
    }
    .flat-card .inbound-name strong {
        font-weight: 600;
        color: var(--mantine-color-bright);
    }
    .inbound-name small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .filters {
        display: flex;
        margin: -4px 0 0;
        padding: 4px;
        border: 0;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
    }
    .filters label {
        position: relative;
        flex: 1;
        text-align: center;
        cursor: pointer;
    }
    .filters label:nth-child(3)::before {
        position: absolute;
        inset-block: 5px;
        inset-inline-start: 0;
        width: 1px;
        background: var(--mantine-color-default-border);
        content: "";
    }
    .filters input {
        position: absolute;
        opacity: 0;
    }
    .filters span {
        display: block;
        min-height: 28px;
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-sm);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: 26px;
    }
    .filters input:checked + span {
        border-color: color-mix(in srgb, var(--mantine-primary-color-6) 30%, transparent);
        background: color-mix(in srgb, var(--mantine-primary-color-6) 12%, transparent);
        color: var(--mantine-primary-color-4);
    }
    .filters input:focus-visible + span {
        outline: 2px solid var(--mantine-primary-color-filled);
    }
    .flat-list {
        margin-block-start: -5px;
        padding: 8px;
        border: 1px solid var(--mantine-color-gray-7);
        border-radius: 8px;
    }
    .flat-space {
        position: relative;
    }
    .flat-item {
        position: absolute;
        inset-inline: 0;
    }
    .empty {
        padding: var(--mantine-spacing-xl);
        text-align: center;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .flat-empty {
        margin: auto;
    }
</style>
