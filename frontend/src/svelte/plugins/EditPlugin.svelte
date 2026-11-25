<script lang="ts">
    import { SyncNodePluginCommand, UpdateNodePluginCommand } from '@remnawave/backend-contract'
    import { OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import { NODE_PLUGIN_TEMPLATE_LIST_LINK } from '@shared/constants/templates/template-list-links'
    import { setupNodePluginSchema } from '@shared/utils/monaco/plugin-schemas'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import type { editor } from 'monaco-editor'
    import { tick, untrack } from 'svelte'
    import { submitCommand, submitVoidCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import EditorActions from '../editor/EditorActions.svelte'
    import EditorFrame from '../editor/EditorFrame.svelte'
    import TemplatePicker from '../editor/TemplatePicker.svelte'
    import UnsavedChanges from '../editor/UnsavedChanges.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { navigate } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import CopyEntityAction from '../ui/CopyEntityAction.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { readPluginConfig } from './config'
    import { nodePluginsQuery, type Plugin, pluginQuery, sharedListsQuery } from './queries'
    import SharedLists from './SharedLists.svelte'

    let { plugin }: { plugin: Plugin } = $props()
    const initial = untrack(() => JSON.stringify(plugin.pluginConfig ?? {}, null, 2))
    const shared = createQuery(() => sharedListsQuery)
    let t = $derived(locale.t)
    let draft = $state(initial)
    let saved = $state(initial)
    let dirty = $derived(draft !== saved)
    let current = $state.raw<editor.IStandaloneCodeEditor | null>(null)
    let showApply = $state(false)
    let showCatalog = $state(false)
    let showLists = $state(false)
    let error = $state('')

    const save = createMutation(() => ({
        mutationFn: (text: string) =>
            submitCommand(UpdateNodePluginCommand, {
                uuid: plugin.uuid,
                pluginConfig: readPluginConfig(text)
            }),
        onSuccess: async (updated, text) => {
            queryClient.setQueryData(pluginQuery(updated.uuid).queryKey, updated)
            saved = text
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
            showApply = true
            await queryClient.invalidateQueries({ queryKey: nodePluginsQuery.queryKey })
        }
    }))

    const sync = createMutation(() => ({
        mutationFn: () => submitVoidCommand(SyncNodePluginCommand, { uuid: plugin.uuid }),
        onSuccess: () =>
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
    }))

    $effect(() => {
        if (current) setupNodePluginSchema(shared.data?.sharedLists)
    })

    function updateDraft(text: string): void {
        draft = text
        error = ''
    }

    function report(reason: unknown): void {
        error = reason instanceof Error ? reason.message : String(reason)
    }

    function submit(): void {
        if (!dirty || !draft.trim() || save.isPending || sync.isPending) return
        error = ''
        sync.reset()
        save.mutate(draft)
    }

    function shortcut(event: KeyboardEvent): void {
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
            event.preventDefault()
            if (!showLists && !showCatalog && !showApply) submit()
        }
    }

    function apply(): void {
        showApply = false
        error = ''
        sync.mutate()
    }
</script>

<UnsavedChanges {dirty} />

<svelte:window onkeydown={shortcut} />
<div class="plugin-editor">
    <PageHeader title={plugin.name} description={plugin.uuid} icon="TbPackage">
        <CopyEntityAction entity={OPEN_ENTITY.NODE_PLUGIN} id={plugin.uuid} />
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
        <ActionIcon
            icon="TbList"
            tone="indigo"
            label={t('common.field.shared-lists')}
            onclick={() => (showLists = true)}
        />
        <ActionIcon
            icon="TbArrowBackUp"
            tone="gray"
            label={t('create-node-modal.widget.back')}
            onclick={() => void navigate(ROUTES.DASHBOARD.MANAGEMENT.NODE_PLUGINS.ROOT)}
        />
    </PageHeader>

    <EditorFrame>
        {#snippet children()}
            <CodeEditor
                language="json"
                value={draft}
                modelUri="node-plugin://editor"
                label={t('constants.node-plugins')}
                onchange={updateDraft}
                onready={(instance) => (current = instance)}
            />
        {/snippet}
        {#snippet status()}
            {#if error || save.error || sync.error}
                <p class="editor-status" role="alert">
                    <code>{error || save.error?.message || sync.error?.message}</code>
                </p>
            {/if}
        {/snippet}
        {#snippet actions()}
            <EditorActions
                {current}
                value={draft}
                saving={save.isPending || sync.isPending}
                saveTone={dirty ? 'teal' : 'gray'}
                saveDisabled={!dirty || !draft.trim()}
                onsave={submit}
                oncopy={() =>
                    notifications.show({
                        title: t('common.message.copied'),
                        message: '',
                        color: 'teal'
                    })}
                onerror={report}
                onload={() => (showCatalog = true)}
            >
                {#snippet extra()}
                    <button
                        type="button"
                        role="menuitem"
                        tabindex="-1"
                        disabled={dirty || save.isPending || sync.isPending}
                        onclick={() => {
                            showApply = true
                        }}
                    >
                        <Icon name="TbRocket" size={14} />
                        {t('apply-to-nodes.apply')}
                    </button>
                {/snippet}
            </EditorActions>
        {/snippet}
    </EditorFrame>
</div>

{#if showApply}
    <Overlay
        title={t('apply-to-nodes.title')}
        icon="TbPackage"
        tone="grape"
        width={440}
        fullscreenOnMobile={false}
        close={() => (showApply = false)}
    >
        <div class="apply-options">
            <button type="button" class="mantine-UnstyledButton-root apply-option" onclick={apply}>
                <Icon name="TbRocket" size={22} />
                <span
                    ><strong>{t('apply-to-nodes.apply')}</strong
                    ><small>{t('apply-to-nodes.apply-description')}</small></span
                >
            </button>
            <button
                type="button"
                class="mantine-UnstyledButton-root apply-option later"
                onclick={() => (showApply = false)}
            >
                <Icon name="TbDeviceFloppy" size={22} />
                <span
                    ><strong>{t('apply-to-nodes.later')}</strong
                    ><small>{t('apply-to-nodes.later-description')}</small></span
                >
            </button>
        </div>
    </Overlay>
{/if}
{#if showLists}
    <SharedLists close={() => (showLists = false)} />
{/if}
{#if showCatalog}
    <TemplatePicker
        catalog={NODE_PLUGIN_TEMPLATE_LIST_LINK}
        type="NODE_PLUGIN"
        onclose={() => (showCatalog = false)}
        onload={async (text) => {
            updateDraft(text)
            await tick()
            await current?.getAction('editor.action.formatDocument')?.run()
        }}
    />
{/if}

<style>
    .plugin-editor,
    .apply-options {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
    .apply-options {
        gap: var(--mantine-spacing-sm);
    }
    .editor-status {
        margin: 0;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-md);
        border-top: 1px solid rgb(241 65 65);
        background: rgb(241 65 65 / 10%);
    }
    .editor-status code {
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        color: var(--mantine-color-red-4);
    }
    .apply-option {
        --option-edge: var(--mantine-color-teal-6);
        --option-color: var(--mantine-color-teal-4);
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-md);
        border: 1px solid color-mix(in srgb, var(--option-edge) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--option-edge) 12%, transparent);
        color: var(--option-color);
        cursor: pointer;
    }
    .apply-option:hover {
        background: color-mix(in srgb, var(--option-edge) 20%, transparent);
    }
    .apply-option.later {
        --option-edge: var(--mantine-color-gray-6);
        --option-color: var(--mantine-color-gray-light-color);
    }
    .apply-option strong,
    .apply-option small {
        display: block;
    }
    .apply-option small {
        color: var(--mantine-color-dimmed);
    }
</style>
