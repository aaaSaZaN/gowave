<script lang="ts">
    import type { editor } from 'monaco-editor'
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionMenu from '../ui/ActionMenu.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'

    let {
        current,
        value,
        onsave,
        oncopy,
        onerror,
        onload,
        saving,
        saveDisabled = false,
        saveTone = 'teal',
        saveAnyway,
        extra
    }: {
        current: editor.IStandaloneCodeEditor | null
        value: string
        onsave: () => void
        oncopy: () => void
        onerror: (error: unknown) => void
        onload: () => void
        saving: boolean
        saveDisabled?: boolean
        saveTone?: string
        saveAnyway?: (() => void) | undefined
        extra?: Snippet
    } = $props()
    const t = $derived(locale.t)

    function copyAll(): void {
        void navigator.clipboard.writeText(value).then(oncopy, onerror)
    }

    function selectAll(): void {
        const model = current?.getModel()
        if (model) {
            current?.setSelection(model.getFullModelRange())
            current?.focus()
        }
    }

    async function cut(): Promise<void> {
        const instance = current
        const model = instance?.getModel()
        const selection = instance?.getSelection()
        if (!instance || !model || !selection || selection.isEmpty()) return
        const version = model.getVersionId()
        await navigator.clipboard.writeText(model.getValueInRange(selection))
        if (
            current === instance &&
            instance.getModel() === model &&
            model.getVersionId() === version
        ) {
            instance.executeEdits('cut', [{ range: selection, text: '' }])
            instance.focus()
        }
        oncopy()
    }

    async function paste(): Promise<void> {
        const instance = current
        const selection = instance?.getSelection()
        if (!instance || !selection) return
        const text = await navigator.clipboard.readText()
        if (current === instance) {
            instance.executeEdits('paste', [{ range: selection, text }])
            instance.focus()
        }
    }

    function format(): void {
        void current?.getAction('editor.action.formatDocument')?.run().catch(onerror)
    }
</script>

<div class="editor-actions">
    <Button
        variant="soft"
        color={saveTone}
        icon="PiFloppyDisk"
        disabled={!current || saveDisabled || saving}
        onclick={onsave}
        >{t('common.action.save')}</Button
    >
    {#if saveAnyway && !saving}
        <Button color="red" variant="light" icon="PiFloppyDisk" onclick={saveAnyway}>
            {t('config-editor-actions.feature.save-anyway')}
        </Button>
    {/if}
    <div class="format-group">
        <ActionMenu
            subtle={false}
            icon="TbMenuDeep"
            label={t('common.action.actions')}
            size={36}
            iconSize={20}
            alignEnd={false}
            preferAbove
            width={230}
        >
            {@render extra?.()}
            <button
                type="button"
                role="menuitem"
                tabindex="-1"
                disabled={!current}
                onclick={() => {
                    copyAll()
                }}
            >
                <Icon name="TbClipboardCopy" size={14} />
                {t('config-editor-actions.feature.copy-all-content')}
            </button>
            <button
                type="button"
                role="menuitem"
                tabindex="-1"
                disabled={!current}
                onclick={() => {
                    selectAll()
                }}
            >
                <Icon name="TbSelectAll" size={14} />{t('common.action.select-all')}
            </button>
            <button
                type="button"
                role="menuitem"
                tabindex="-1"
                disabled={!current || saving}
                onclick={() => {
                    void cut().catch(onerror)
                }}
            >
                <Icon name="TbCut" size={14} />
                {t('config-editor-actions.feature.cut-selection')}
            </button>
            <button
                type="button"
                role="menuitem"
                tabindex="-1"
                disabled={!current || saving}
                onclick={() => {
                    void paste().catch(onerror)
                }}
            >
                <Icon name="TbClipboardText" size={14} />
                {t('config-editor-actions.feature.paste-from-clipboard')}
            </button>
            <hr>
            <button
                type="button"
                role="menuitem"
                tabindex="-1"
                onclick={() => {
                    onload()
                }}
            >
                <Icon name="TbDownload" size={14} />{t('common.action.load-from-github')}
            </button>
        </ActionMenu>
        <Button
            color="gray"
            icon="PiCheckSquareOffset"
            variant="default"
            disabled={!current || saving}
            onclick={format}
            >{t('config-editor-actions.feature.format')}</Button
        >
    </div>
</div>

<style>
    .editor-actions,
    .format-group {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
    }
    .editor-actions {
        gap: var(--mantine-spacing-md);
    }
    .format-group {
        flex-wrap: nowrap;
    }
    .format-group :global(.action-icon) {
        border-start-end-radius: 0;
        border-end-end-radius: 0;
    }
    .format-group :global(.mantine-Button-root) {
        border-start-start-radius: 0;
        border-end-start-radius: 0;
        margin-inline-start: -1px;
    }
    hr {
        border: 0;
        border-top: 1px solid var(--mantine-color-default-border);
        margin: 4px 0;
    }
    @media (max-width: 48em) {
        .editor-actions {
            min-width: 0;
            gap: var(--mantine-spacing-md);
        }
        .editor-actions > :global(.mantine-Button-root) {
            flex-grow: 1;
        }
        .format-group {
            flex-grow: 1;
        }
        .format-group > :global(.mantine-Button-root) {
            flex: 1;
        }
    }
</style>
