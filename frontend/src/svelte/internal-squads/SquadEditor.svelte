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
    import RouteLoading from '../RouteLoading.svelte'
    import Accordion from '../ui/Accordion.svelte'
    import Button from '../ui/Button.svelte'
    import Checkbox from '../ui/Checkbox.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import Overlay from '../ui/Overlay.svelte'
    import TextInput from '../ui/TextInput.svelte'
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
        <TextInput label={t('common.field.name')} bind:value={name} name="name" required />
        <fieldset>
            <legend>{t('common.field.inbounds')}</legend>
            {#if profiles.isPending}
                <RouteLoading compact />
            {:else if profiles.isError}
                <p role="alert">{profiles.error.message}</p>
            {:else}
                {#each profiles.data.configProfiles as profile (profile.uuid)}
                    <Accordion>
                        {#snippet heading()}
                            {profile.name}
                        {/snippet}
                        <div class="inbound-list">
                            {#each profile.inbounds as inbound (inbound.uuid)}
                                <Checkbox
                                    label={inbound.tag}
                                    value={inbound.uuid}
                                    checked={selected.includes(inbound.uuid)}
                                    onchange={(event) => {
                                        selected = event.currentTarget.checked
                                            ? [...selected, inbound.uuid]
                                            : selected.filter((uuid) => uuid !== inbound.uuid)
                                    }}
                                />
                            {/each}
                        </div>
                    </Accordion>
                {/each}
            {/if}
        </fieldset>
        {#if mutation.isError}
            <p role="alert">{mutation.error.message}</p>
        {/if}
        <div class="button-row">
            <Button onclick={dismissContainingDialog}>
                {t('common.action.cancel')}
            </Button>
            <Button
                type="submit"
                variant="soft"
                color="teal"
                disabled={!name.trim() || mutation.isPending || saveTags.isPending}
            >
                {t('common.action.save')}
            </Button>
        </div>
    </form>
    <form onsubmit={updateTags}>
        <TextInput
            label={t('common.field.tags')}
            bind:value={tags}
            name="tags"
            placeholder="TAG1, TAG2"
        />
        <div class="button-row">
            <Button
                type="submit"
                variant="soft"
                color="teal"
                disabled={saveTags.isPending ||
                    mutation.isPending ||
                    tags === squad.tags.join(', ')}
            >
                {t('common.action.save')}
                · {t('common.field.tags')}
            </Button>
        </div>
        {#if saveTags.error}
            <p role="alert">{saveTags.error.message}</p>
        {/if}
    </form>
</Overlay>

<style>
    .inbound-list {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }

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
    fieldset {
        max-height: 40dvh;
        overflow: auto;
    }
</style>
