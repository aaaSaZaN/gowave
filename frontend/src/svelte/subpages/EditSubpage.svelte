<script lang="ts">
    import {
        type GetSubpageConfigCommand,
        UpdateSubpageConfigCommand
    } from '@remnawave/backend-contract'
    import { OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import { SUBPAGE_CONFIG_TEMPLATE_LIST_LINK } from '@shared/constants/templates/template-list-links'
    import { createMutation } from '@tanstack/svelte-query'
    import { onDestroy, untrack } from 'svelte'
    import { ZodError } from 'zod'
    import { submitCommand } from '../commands'
    import TemplatePicker from '../editor/TemplatePicker.svelte'
    import UnsavedChanges from '../editor/UnsavedChanges.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { navigate } from '../router'
    import { subpageConfigsQuery, subpageQuery } from '../subscriptions/queries'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import CopyEntityAction from '../ui/CopyEntityAction.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { type ImportMode, mergePageConfig, type PageConfig, readPageConfig } from './config'
    import PageConfigFields from './PageConfigFields.svelte'

    let { entry }: { entry: GetSubpageConfigCommand.Response['response'] } = $props()
    let t = $derived(locale.t)
    const original = untrack(() => JSON.stringify(entry.config, null, 2) ?? '{}')
    const initial = untrack(() => {
        try {
            return { value: readPageConfig(JSON.parse(original)), errors: [] }
        } catch (error) {
            return { value: undefined, errors: problems(error) }
        }
    })
    let saved = $state(initial.value ? JSON.stringify(initial.value, null, 2) : original)
    let issues = $state<string[]>(initial.errors)
    let config = $state<PageConfig | undefined>(initial.value)
    let raw = $state(original)
    let jsonMode = $state(!initial.value)
    let importing = $state<PageConfig>()
    let fileInput: HTMLInputElement
    let catalog = $state(false)
    let importingFile = $state(false)
    let copied = $state(false)
    let copyTimer: ReturnType<typeof setTimeout> | undefined
    let dirty = $derived((jsonMode ? raw : JSON.stringify(config, null, 2)) !== saved)

    onDestroy(() => clearTimeout(copyTimer))

    const save = createMutation(() => ({
        mutationFn: (draft: PageConfig) =>
            submitCommand(UpdateSubpageConfigCommand, { uuid: entry.uuid, config: draft }),
        onSuccess: (updated, draft) => {
            queryClient.setQueryData(subpageQuery(updated.uuid).queryKey, updated)
            void queryClient.invalidateQueries({ queryKey: subpageConfigsQuery.queryKey })
            saved = JSON.stringify(draft, null, 2)
            notifications.show({
                title: t('common.message.success'),
                message: t('subpage-config-visual-editor.widget.configuration-saved-successfully'),
                color: 'teal'
            })
        }
    }))

    function problems(error: unknown): string[] {
        if (error instanceof ZodError) {
            return error.issues.map(({ path, message }) =>
                path.length ? `${path.join('.')}: ${message}` : message
            )
        }
        return [error instanceof Error ? error.message : String(error)]
    }

    function report(error: unknown): void {
        issues = problems(error)
    }

    function submit(): void {
        if (save.isPending) return
        save.reset()
        issues = []
        try {
            const draft = readPageConfig(jsonMode ? JSON.parse(raw) : $state.snapshot(config))
            config = draft
            raw = JSON.stringify(draft, null, 2)
            jsonMode = false
            save.mutate(draft)
        } catch (error) {
            report(error)
        }
    }

    function stageImport(text: string): void {
        const draft = readPageConfig(JSON.parse(text))
        importing = draft
    }

    function importFile(event: Event): void {
        if (!(event.currentTarget instanceof HTMLInputElement)) return
        const input = event.currentTarget
        const file = input.files?.[0]
        if (!file) return
        input.value = ''
        importingFile = true
        issues = []
        void file
            .text()
            .then(stageImport)
            .catch(report)
            .finally(() => (importingFile = false))
    }

    function applyImport(mode: ImportMode): void {
        if (!importing || (mode !== 'full' && !config)) return
        try {
            if (mode === 'full') {
                config = importing
            } else {
                const current = jsonMode ? readPageConfig(JSON.parse(raw)) : $state.snapshot(config)
                if (current) config = mergePageConfig(current, importing, mode)
            }
            raw = JSON.stringify(config, null, 2) ?? '{}'
            jsonMode = false
            issues = []
            notifications.show({
                title: t('common.message.success'),
                message: t('subpage-config-editor-page.component.config-imported-successfully'),
                color: 'teal'
            })
            importing = undefined
        } catch (error) {
            report(error)
        }
    }

    function download(): void {
        const data = jsonMode ? raw : JSON.stringify(config, null, 2)
        const href = URL.createObjectURL(new Blob([data ?? '{}'], { type: 'application/json' }))
        const link = document.createElement('a')
        link.href = href
        link.download = `subpage-${entry.uuid}.json`
        link.click()
        window.setTimeout(() => URL.revokeObjectURL(href), 1000)
    }

    function copyUuid(): void {
        issues = []
        void navigator.clipboard.writeText(entry.uuid).then(() => {
            copied = true
            clearTimeout(copyTimer)
            copyTimer = setTimeout(() => (copied = false), 2000)
        }, report)
    }
</script>

<UnsavedChanges {dirty} />

<div class="editor">
    <PageHeader title={entry.name} description={entry.uuid} icon="TbFile">
        <CopyEntityAction entity={OPEN_ENTITY.SUBPAGE_CONFIG} id={entry.uuid} />
        <ActionIcon
            icon="TbUpload"
            class="file-action"
            label={t('subpage-config-editor-page.component.import-config')}
            disabled={importingFile || save.isPending}
            onclick={() => fileInput.click()}
        />
        <ActionIcon
            icon="TbDownload"
            class="file-action"
            label={t('subpage-config-editor-page.component.download-config')}
            onclick={download}
        />
        <ActionIcon
            icon={copied ? 'PiCheck' : 'PiCopy'}
            class="file-action"
            data-copied={copied || undefined}
            label={t('common.action.copy-uuid')}
            onclick={copyUuid}
        />
        <ActionIcon
            icon="TbArrowBackUp"
            tone="gray"
            label={t('create-node-modal.widget.back')}
            onclick={() => void navigate(ROUTES.DASHBOARD.SUBPAGE_CONFIGS.ROOT)}
        />
    </PageHeader>
    <input
        bind:this={fileInput}
        type="file"
        accept=".json,application/json"
        hidden
        onchange={importFile}
    >
    <PageHeader
        title={t('subpage-config-visual-editor.widget.subpage-editor')}
        description={t(
            'subpage-config-visual-editor.widget.edit-your-subscription-page-configuration'
        )}
        icon="TbPalette"
        compact
        wrapActions
    >
        <div class="editor-actions">
            <Button
                variant="light"
                size="md"
                icon="TbCloudDownload"
                iconSize={24}
                onclick={() => (catalog = true)}
            >
                {t('common.action.load-from-github')}
            </Button>
            <Button
                variant="soft"
                size="md"
                icon="TbDeviceFloppy"
                iconSize={24}
                onclick={submit}
                disabled={save.isPending}
            >
                {t('common.action.save')}
            </Button>
        </div>
    </PageHeader>
    {#if issues.length}
        <ul role="alert">
            {#each issues as issue, index (index)}
                <li>{issue}</li>
            {/each}
        </ul>
    {/if}
    {#if save.error}
        <p role="alert">{save.error.message}</p>
    {/if}
    {#if jsonMode}
        <textarea
            class="gw-input"
            aria-label="Configuration JSON"
            name="configJson"
            rows="22"
            spellcheck="false"
            bind:value={raw}
        ></textarea>
    {:else if config}
        <PageConfigFields bind:config={() => config as PageConfig, (value) => (config = value)} />
    {/if}
</div>

{#if importing}
    <Overlay
        title={t('subpage-config-editor-page.component.import-config')}
        icon="TbFileImport"
        fullscreenOnMobile={false}
        close={() => (importing = undefined)}
    >
        <p>{t('import-config-sections.modal.import-options-description')}</p>
        {#if issues.length}
            <p role="alert">{issues[0]}</p>
        {/if}
        <div class="button-row">
            <Button variant="soft" onclick={() => applyImport('full')}>
                {t('import-config-sections.modal.full-import')}
            </Button>
            <Button disabled={!config} onclick={() => applyImport('platforms')}>
                {t('import-config-sections.modal.import-platforms')}
            </Button>
            <Button disabled={!config} onclick={() => applyImport('baseTranslations')}>
                {t('import-config-sections.modal.import-base-translations')}
            </Button>
            <Button disabled={!config} onclick={() => applyImport('svgLibrary')}>
                {t('import-config-sections.modal.import-svg-library')}
            </Button>
            <Button onclick={() => (importing = undefined)}>
                {t('common.action.cancel')}
            </Button>
        </div>
    </Overlay>
{/if}

{#if catalog}
    <TemplatePicker
        catalog={SUBPAGE_CONFIG_TEMPLATE_LIST_LINK}
        type="SUBPAGE_CONFIG"
        onload={(text) => {
            stageImport(text)
            return Promise.resolve()
        }}
        onclose={() => (catalog = false)}
    />
{/if}

<style>
    .editor {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
    .editor > :global(.page-header.compact) {
        box-shadow: var(--mantine-shadow-xl);
    }
    .editor :global(.file-action:not(:disabled)) {
        border-color: transparent;
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
    }
    .editor :global(.file-action:not(:disabled):hover) {
        background: var(--mantine-color-gray-light-hover);
    }
    .editor :global(.file-action[data-copied]:not(:disabled)) {
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
    }
    .editor-actions {
        display: flex;
        align-items: start;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-md);
    }
    .editor-actions :global(button) {
        border-color: rgba(34, 211, 238, 0.4);
        background: linear-gradient(135deg, rgba(34, 211, 238, 0.2), rgba(6, 182, 212, 0.15));
    }
    .editor-actions :global(button:hover:not(:disabled)) {
        border-color: rgba(34, 211, 238, 0.6);
        background: linear-gradient(135deg, rgba(34, 211, 238, 0.3), rgba(6, 182, 212, 0.25));
        transform: translateY(-1px);
    }
    @media (max-width: 30em) {
        .editor-actions {
            flex-direction: column;
        }
    }
    textarea {
        font-family: monospace;
        width: 100%;
    }
</style>
