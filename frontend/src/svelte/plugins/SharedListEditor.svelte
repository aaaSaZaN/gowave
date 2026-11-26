<script lang="ts">
    import {
        CreateSharedListCommand,
        SyncSharedListCommand,
        UpdateSharedListCommand
    } from '@remnawave/backend-contract'
    import { setupSharedListSchema } from '@shared/utils/monaco/plugin-schemas'
    import { createMutation } from '@tanstack/svelte-query'
    import { untrack } from 'svelte'
    import { submitCommand, submitVoidCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import ApplyToNodes from '../ui/ApplyToNodes.svelte'
    import Button from '../ui/Button.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import { readSharedListConfig } from './config'
    import { sharedListQuery, sharedListsQuery } from './queries'

    let {
        existingName,
        existingConfig,
        close
    }: {
        existingName: string | null
        existingConfig: unknown
        close: () => void
    } = $props()
    let t = $derived(locale.t)
    let name = $state(untrack(() => existingName ?? ''))
    let draft = $state(
        untrack(() => JSON.stringify(existingConfig ?? { type: 'ipList', items: [] }, null, 2))
    )
    let persistedName = $state(untrack(() => existingName))
    let ready = $state(false)
    let problem = $state('')
    let choosingApply = $state(false)

    const save = createMutation(() => ({
        mutationFn: (sync: boolean) =>
            submitCommand(persistedName ? UpdateSharedListCommand : CreateSharedListCommand, {
                name: name.trim(),
                config: readSharedListConfig(draft)
            }).then((result) => ({ result, sync })),
        onSuccess: async ({ result, sync }) => {
            persistedName = result.name
            queryClient.setQueryData(sharedListQuery(result.name).queryKey, result)
            await queryClient.invalidateQueries({ queryKey: sharedListsQuery.queryKey })
            if (sync) apply.mutate(result.name)
            else close()
        }
    }))
    const apply = createMutation(() => ({
        mutationFn: (listName: string) =>
            submitVoidCommand(SyncSharedListCommand, { name: listName }),
        onSuccess: close,
        onError: () => {
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        }
    }))

    function submit(sync: boolean): void {
        problem = ''
        save.reset()
        apply.reset()
        if (!name.trim()) {
            problem = t('common.field.name')
            return
        }
        save.mutate(sync)
    }
</script>

<div class="fields">
    <label
        >{t('common.field.name')}
        <input
            class="gw-input"
            name="sharedListName"
            bind:value={name}
            readonly={persistedName !== null}
        ></label
    >
    <p>{t('shared-lists.editor.name-description')}</p>
    <div class="editor">
        <CodeEditor
            language="json"
            modelUri="shared-list://editor"
            label={t('common.field.shared-lists')}
            value={draft}
            onchange={(text) => {
                draft = text
                problem = ''
            }}
            onready={(instance) => {
                ready = Boolean(instance)
                if (instance) setupSharedListSchema()
            }}
        />
    </div>
    {#if problem || save.error || apply.error}
        <p role="alert">{problem || save.error?.message || apply.error?.message}</p>
    {/if}
    <div class="button-row">
        <Button variant="subtle" disabled={save.isPending || apply.isPending} onclick={close}
            >{t('common.action.cancel')}</Button
        >
        <Button
            variant="soft"
            color="teal"
            disabled={!ready || save.isPending || apply.isPending}
            onclick={() => {
                if (persistedName) choosingApply = true
                else submit(false)
            }}
            >{t('common.action.save')}</Button
        >
    </div>
</div>

{#if choosingApply}
    <ApplyToNodes
        icon="TbList"
        tone="indigo"
        close={() => (choosingApply = false)}
        choose={(immediate) => {
            choosingApply = false
            submit(immediate)
        }}
    />
{/if}

<style>
    .fields,
    label {
        display: grid;
        gap: 0.75rem;
    }
    .button-row {
        justify-content: end;
    }
    .editor {
        display: flex;
        min-height: 20rem;
        border: 1px solid var(--gw-border);
    }
    p {
        margin: 0;
    }
</style>
