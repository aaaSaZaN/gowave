<script lang="ts">
    import { untrack } from 'svelte'
    import { SvelteSet } from 'svelte/reactivity'
    import type { Profile } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'

    type Inbound = Profile['inbounds'][number]
    let {
        profiles,
        current,
        onselect,
        currentInbounds,
        onselectMany,
        close
    }: {
        profiles: readonly Profile[]
        current: string
        currentInbounds?: string[]
        onselectMany?: (profile: Profile, ids: string[]) => void
        onselect?: (profile: Profile, inbound: Inbound) => void
        close: () => void
    } = $props()

    const initial = untrack(() => current.split('/'))
    const open = new SvelteSet(initial[0] ? [initial[0]] : [])
    let selected = $state<string[]>(
        untrack(() => currentInbounds?.slice() ?? (initial[1] ? [initial[1]] : []))
    )
    let search = $state('')
    let preview = $state<{ title: string; value: unknown }>()
    let t = $derived(locale.t)
    let selectedProfile = $derived(
        profiles.find(({ inbounds }) => inbounds.some(({ uuid }) => selected.includes(uuid)))
    )
    let selectedInbound = $derived(
        selectedProfile?.inbounds.find(({ uuid }) => selected.includes(uuid))
    )
    let visible = $derived.by(() => {
        const term = search.trim().toLowerCase()
        if (!term) return profiles
        return profiles.flatMap((profile) => {
            if (profile.name.toLowerCase().includes(term)) return [profile]
            const inbounds = profile.inbounds.filter(
                ({ tag, type }) =>
                    tag.toLowerCase().includes(term) || type.toLowerCase().includes(term)
            )
            return inbounds.length ? [{ ...profile, inbounds }] : []
        })
    })
</script>

<Overlay
    title={t('constants.config-profiles')}
    icon="XrayLogo"
    tone="teal"
    drawer
    width={480}
    {close}
>
    <div class="picker">
        <div class="selection">
            <div>
                <strong class:mono={selectedProfile !== undefined}
                    >{selectedProfile?.name ??
                        t(
                            onselectMany
                                ? 'common.message.no-profile-selected'
                                : 'common.message.no-inbound-selected'
                        )}</strong
                >
                <small
                    >{onselectMany
                        ? selected.length
                            ? t('internal-squads.drawer.widget.selected-inbounds', {
                                  count: selected.length
                              })
                            : t('config-profiles.drawer.widget.no-inbounds-selected')
                        : (selectedInbound?.tag ??
                          t(
                              'hosts-config-profiles.drawer.widget.assign-an-inbound-to-the-host'
                          ))}</small
                >
            </div>
            <div class="selection-actions">
                <ActionIcon
                    icon="TbX"
                    tone="red"
                    size={34}
                    label={t('common.action.clear')}
                    disabled={!selectedInbound}
                    onclick={() => (selected = [])}
                />
                <ActionIcon
                    icon="TbDeviceFloppy"
                    tone="teal"
                    size={34}
                    label={t('common.action.save')}
                    disabled={!selectedInbound}
                    onclick={() => {
                        if (selectedProfile && selectedInbound) {
                            if (onselectMany) onselectMany(selectedProfile, selected)
                            else onselect?.(selectedProfile, selectedInbound)
                        }
                    }}
                />
            </div>
        </div>
        <div class="search">
            <Icon name="TbSearch" size={16} />
            <input
                class="gw-input"
                type="search"
                aria-label={t('common.message.search-profiles-or-inbounds')}
                placeholder={t('common.message.search-profiles-or-inbounds')}
                bind:value={search}
                onkeydown={(event) => {
                    if (event.key === 'Enter') event.preventDefault()
                }}
            >
        </div>
        <div class="profiles">
            {#each visible as profile (profile.uuid)}
                {@const selectedCount = profile.inbounds.filter(({ uuid }) =>
                    selected.includes(uuid)
                ).length}
                <div class="profile">
                    <div class="profile-heading">
                        <button
                            type="button"
                            class="mantine-UnstyledButton-root profile-toggle"
                            aria-expanded={open.has(profile.uuid)}
                            onclick={() => {
                                if (open.has(profile.uuid)) open.delete(profile.uuid)
                                else open.add(profile.uuid)
                            }}
                        >
                            <Icon
                                name={open.has(profile.uuid) ? 'TbChevronUp' : 'TbChevronDown'}
                                size={16}
                            />
                            <span>
                                <strong>{profile.name}</strong>
                                <span class="counts">
                                    <span
                                        class="pill"
                                        class:teal={selectedCount === profile.inbounds.length}
                                    >
                                        <Icon name="TbCheck" size={13} />{selectedCount}
                                        / {profile.inbounds.length}
                                    </span>
                                    <span class="pill" class:teal={profile.nodes.length > 0}>
                                        <Icon name="PiCpu" size={13} />{profile.nodes.length}
                                    </span>
                                </span>
                            </span>
                        </button>
                        {#if onselectMany && profile.inbounds.length}
                            <ActionIcon
                                icon="TbCheck"
                                label={t('common.action.select-all')}
                                size={34}
                                subtle
                                onclick={() =>
                                    (selected = profile.inbounds.map(({ uuid }) => uuid))}
                            />
                            <ActionIcon
                                icon="TbX"
                                label={t('common.action.clear')}
                                size={34}
                                subtle
                                onclick={() => (selected = [])}
                            />
                        {/if}
                        <ActionIcon
                            icon="XrayLogo"
                            label={profile.name}
                            size={34}
                            iconSize={16}
                            subtle
                            onclick={() =>
                                (preview = { title: profile.name, value: profile.config })}
                        />
                    </div>
                    {#if open.has(profile.uuid)}
                        <ul>
                            {#each profile.inbounds as item (item.uuid)}
                                <li class:checked={selected.includes(item.uuid)}>
                                    <label>
                                        <input
                                            type="checkbox"
                                            checked={selected.includes(item.uuid)}
                                            onchange={() =>
                                                (selected = selected.includes(item.uuid)
                                                    ? selected.filter((id) => id !== item.uuid)
                                                    : onselectMany &&
                                                        selectedProfile?.uuid === profile.uuid
                                                      ? [...selected, item.uuid]
                                                      : [item.uuid])}
                                        >
                                        <span>{item.tag}</span>
                                    </label>
                                    <span class="pill gray">{item.type}</span>
                                    {#if item.port}
                                        <span class="pill teal">{item.port}</span>
                                    {/if}
                                    <ActionIcon
                                        icon="TbCode"
                                        label={t(
                                            'flat-inbound-checkbox-card.shared.inbound-config-inbound-tag',
                                            { inboundTag: item.tag }
                                        )}
                                        size={28}
                                        iconSize={16}
                                        tone="cyan"
                                        onclick={() =>
                                            (preview = { title: item.tag, value: item.rawInbound })}
                                    />
                                </li>
                            {/each}
                        </ul>
                    {/if}
                </div>
            {:else}
                <p>
                    {t(
                        search.trim()
                            ? 'common.message.no-profiles-or-inbounds-found'
                            : 'common.message.no-config-profiles-available'
                    )}
                </p>
            {/each}
        </div>
    </div>
</Overlay>

{#if preview}
    <Overlay
        title={preview.title}
        icon="TbCode"
        tone="teal"
        width={900}
        fullscreenOnMobile={false}
        close={() => (preview = undefined)}
    >
        <pre>{JSON.stringify(preview.value, null, 2)}</pre>
    </Overlay>
{/if}

<style>
    .picker,
    .profiles {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .profiles {
        align-content: start;
        min-height: 0;
        gap: var(--mantine-spacing-xs);
    }
    .selection,
    .profile {
        min-width: 0;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .selection,
    .profile-heading,
    .profile-toggle,
    .selection-actions,
    .counts,
    .pill,
    li,
    li label {
        display: flex;
        align-items: center;
    }
    .selection {
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-md);
    }
    .selection strong,
    .selection small {
        display: block;
    }
    .selection strong {
        font-size: var(--mantine-font-size-sm);
    }
    .selection small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .selection small,
    .mono,
    .profile-toggle strong,
    .pill {
        font-family: var(--mantine-font-family-monospace);
    }
    .selection-actions,
    li,
    li label {
        gap: var(--mantine-spacing-xs);
    }
    .search {
        position: relative;
        display: flex;
        align-items: center;
        color: var(--mantine-color-dimmed);
    }
    .search :global(svg) {
        position: absolute;
        inset-inline-start: 10px;
        pointer-events: none;
    }
    .search input {
        width: 100%;
        padding-inline-start: 36px;
    }
    .profile-heading {
        padding-inline-end: var(--mantine-spacing-xs);
    }
    .profile-toggle {
        flex: 1;
        min-width: 0;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md);
    }
    .profile-toggle strong {
        display: block;
        margin-block-end: var(--mantine-spacing-xs);
    }
    .counts {
        gap: var(--mantine-spacing-md);
    }
    .pill {
        gap: 4px;
        padding-inline: 6px;
        border: 1px solid rgba(21, 170, 191, 0.3);
        border-radius: var(--mantine-radius-sm);
        background: rgba(21, 170, 191, 0.15);
        color: var(--mantine-color-cyan-4);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        text-transform: uppercase;
    }
    .pill.teal {
        border-color: rgba(18, 184, 134, 0.3);
        background: rgba(18, 184, 134, 0.15);
        color: var(--mantine-color-teal-4);
    }
    .pill.gray {
        border-color: var(--mantine-color-default-border);
        background: var(--mantine-color-default);
        color: var(--mantine-color-gray-light-color);
    }
    ul {
        display: grid;
        gap: var(--mantine-spacing-xs);
        max-height: 360px;
        overflow: auto;
        list-style: none;
        padding: var(--mantine-spacing-md);
        margin: 0;
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    li {
        min-height: 52px;
        padding: var(--mantine-spacing-xs);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
    }
    li.checked {
        border-color: var(--mantine-color-cyan-6);
    }
    li label {
        flex: 1;
        min-width: 0;
        cursor: pointer;
    }
    li label span {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    li input[type="checkbox"] {
        width: 18px;
        height: 18px;
        flex: none;
        margin: 0;
    }
    p {
        text-align: center;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    pre {
        overflow: auto;
        margin: 0;
        padding: var(--mantine-spacing-md);
        background: var(--gw-json-bg);
        color: var(--gw-json-fg);
        border-radius: var(--mantine-radius-md);
    }
</style>
