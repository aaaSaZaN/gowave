<script lang="ts">
    import {
        SetInternalSquadTagsCommand,
        UpdateInternalSquadCommand
    } from '@remnawave/backend-contract'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { untrack } from 'svelte'
    import { submitCommand } from '../commands'
    import { profilesQuery } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import Overlay from '../ui/Overlay.svelte'
    import type { Squad } from './queries'

    let { squad, close }: { squad: Squad; close: () => void } = $props()
    const initialSquad = untrack(() => squad)
    let name = $state(initialSquad.name)
    let selected = $state(initialSquad.inbounds.map(({ uuid }) => uuid))
    let tags = $state(initialSquad.tags.join(', '))
    let t = $derived(locale.t)

    const profiles = createQuery(() => profilesQuery)
    const mutation = createMutation(() => ({
        mutationFn: (values: { name: string; inbounds: string[] }) =>
            submitCommand(UpdateInternalSquadCommand, { ...values, uuid: squad.uuid }),
        onSuccess: async () => {
            await queryClient.invalidateQueries({ queryKey: ['internalSquads'] })
            close()
        }
    }))

    const saveTags = createMutation(() => ({
        mutationFn: async (values: SetInternalSquadTagsCommand.RequestBody) =>
            submitCommand(SetInternalSquadTagsCommand, values),
        onSuccess: async (saved) => {
            tags = saved.tags.join(', ')
            await queryClient.invalidateQueries({ queryKey: ['internalSquads'] })
        }
    }))

    function updateTags(event: SubmitEvent): void {
        event.preventDefault()
        const normalized = [
            ...new Set(
                tags
                    .split(',')
                    .map((tag) => tag.trim().toUpperCase())
                    .filter(Boolean)
            )
        ]
        saveTags.mutate({ uuid: squad.uuid, tags: normalized })
    }

    function save(event: SubmitEvent): void {
        event.preventDefault()
        mutation.mutate({ name: name.trim(), inbounds: selected.slice() })
    }
</script>

<Overlay
    title={t('internal-squads.drawer.widget.edit-internal-squad')}
    icon="TbCirclesRelation"
    tone="teal"
    drawer
    width={500}
    {close}
>
    <section class="squad-summary">
        <h3>{squad.name}</h3>
        <span>{t('constants.users')} {squad.info.membersCount}</span>
        <span>{t('common.field.inbounds')} {squad.info.inboundsCount}</span>
        {#if squad.tags.length}
            <span>{squad.tags.join(', ')}</span>
        {/if}
    </section>
    <form onsubmit={save}>
        <label>
            {t('common.field.name')}
            <input bind:value={name} name="name" required>
        </label>
        <fieldset>
            <legend>{t('common.field.inbounds')}</legend>
            {#if profiles.isPending}
                <p role="status">{t('common.message.loading')}</p>
            {:else if profiles.isError}
                <p role="alert">{profiles.error.message}</p>
            {:else}
                {#each profiles.data.configProfiles as profile (profile.uuid)}
                    <h3>{profile.name}</h3>
                    {#each profile.inbounds as inbound (inbound.uuid)}
                        <label class="inbound">
                            <input type="checkbox" bind:group={selected} value={inbound.uuid}>
                            {inbound.tag}
                        </label>
                    {/each}
                {/each}
            {/if}
        </fieldset>
        {#if mutation.isError}
            <p role="alert">{mutation.error.message}</p>
        {/if}
        <div class="button-row">
            <button type="button" onclick={dismissContainingDialog}>
                {t('common.action.cancel')}
            </button>
            <button
                type="submit"
                disabled={!name.trim() || mutation.isPending || saveTags.isPending}
            >
                {t('common.action.save')}
            </button>
        </div>
    </form>
    <form onsubmit={updateTags}>
        <label>
            {t('common.field.tags')}
            <input name="tags" bind:value={tags} placeholder="TAG1, TAG2">
        </label>
        <div class="button-row">
            <button
                type="submit"
                disabled={saveTags.isPending ||
                    mutation.isPending ||
                    tags === squad.tags.join(', ')}
            >
                {t('common.action.save')}
                · {t('common.field.tags')}
            </button>
        </div>
        {#if saveTags.error}
            <p role="alert">{saveTags.error.message}</p>
        {/if}
    </form>
</Overlay>

<style>
    .squad-summary,
    form {
        margin: 0 0 var(--mantine-spacing-md);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        padding: var(--mantine-spacing-md);
    }
    .squad-summary h3 {
        margin: 0 0 6px;
    }
    .squad-summary span {
        display: inline-block;
        margin: 0 6px 6px 0;
        padding: 2px 8px;
        border-radius: var(--mantine-radius-sm);
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
        font-size: var(--mantine-font-size-xs);
    }
    form,
    label:not(.inbound) {
        display: grid;
        gap: 0.5rem;
    }

    fieldset {
        max-height: 40dvh;
        overflow: auto;
    }

    .inbound {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }
</style>
