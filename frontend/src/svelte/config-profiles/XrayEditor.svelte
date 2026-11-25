<script lang="ts">
    import {
        type GetConfigProfileByUuidCommand,
        type GetSnippetsCommand,
        UpdateConfigProfileCommand
    } from '@remnawave/backend-contract'
    import { XRAY_CORE_TEMPLATE_LIST_LINK } from '@shared/constants/templates'
    import { MonacoSetupFeature } from '@shared/utils/monaco/xray-schemas'
    import { prepareXrayConfig } from '@shared/utils/xray/prepare-config'
    import {
        getXrayValidatorState,
        startXrayValidator,
        subscribeXrayValidator
    } from '@shared/utils/xray/validator'
    import { createMutation } from '@tanstack/svelte-query'
    import type { editor } from 'monaco-editor'
    import { onMount, tick, untrack } from 'svelte'
    import { submitCommand } from '../commands'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import EditorActions from '../editor/EditorActions.svelte'
    import EditorFrame from '../editor/EditorFrame.svelte'
    import KeypairGenerator from '../editor/KeypairGenerator.svelte'
    import TemplatePicker from '../editor/TemplatePicker.svelte'
    import UnsavedChanges from '../editor/UnsavedChanges.svelte'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import Button from '../ui/Button.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import ProfilePreview from './ProfilePreview.svelte'
    import { computedProfileQuery, profileQuery, profilesQuery } from './queries'

    let {
        profile,
        snippets
    }: {
        profile: GetConfigProfileByUuidCommand.Response['response']
        snippets: GetSnippetsCommand.Response['response']['snippets']
    } = $props()

    const initial = untrack(() => JSON.stringify(profile.config ?? {}, null, 2))
    let saved = $state(initial)
    let draft = $state(initial)
    let dirty = $derived(draft !== saved)
    let current = $state.raw<editor.IStandaloneCodeEditor | null>(null)
    let validator = $state(getXrayValidatorState())
    let snippetsByName = $derived(
        new Map(snippets.map((snippet) => [snippet.name, snippet.snippet]))
    )
    let validation = $state<string | null | undefined>(undefined)
    let validationTime = $state('')
    let confirmSave = $state(false)
    const configValid = $derived(validator.status === 'ready' && validation === null)
    let importOpen = $state(false)
    let keypairOpen = $state(false)
    let computedOpen = $state(false)
    let t = $derived(locale.t)

    const save = createMutation(() => ({
        mutationFn: (config: unknown) =>
            submitCommand(UpdateConfigProfileCommand, {
                uuid: profile.uuid,
                config
            }),
        onSuccess: (updated) => {
            saved = JSON.stringify(updated.config ?? {}, null, 2)
            draft = saved
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
            queryClient.setQueryData(profileQuery(updated.uuid).queryKey, updated)
            void queryClient.invalidateQueries({ queryKey: profilesQuery.queryKey })
            void queryClient.invalidateQueries({
                queryKey: computedProfileQuery(updated.uuid).queryKey
            })
        },
        onError: (error) => report(error, 'Update Config Profile')
    }))

    onMount(() => {
        const unsubscribe = subscribeXrayValidator((value) => {
            validator = value
        })
        startXrayValidator()
        return () => {
            unsubscribe()
        }
    })

    $effect(() => {
        void MonacoSetupFeature.setup(locale.language, snippets)
    })

    $effect(() => {
        if (validator.status !== 'ready') return
        const text = draft
        const map = snippetsByName
        const timeout = setTimeout(() => {
            validation = validate(text, map)
            validationTime = new Date().toTimeString().slice(0, 8)
        }, 175)
        return () => clearTimeout(timeout)
    })

    function validate(text: string, map: ReadonlyMap<string, unknown>): string | null {
        const prepared = prepareXrayConfig(text, map)
        if ('error' in prepared) return prepared.error
        try {
            if (!window.XrayParseConfig) return 'Xray validator is unavailable'
            return window.XrayParseConfig(prepared.config) || null
        } catch (reason) {
            return reason instanceof Error ? reason.message : t('common.message.error')
        }
    }

    function updateDraft(text: string): void {
        draft = text
        validation = undefined
    }

    function saveConfig(): void {
        if (save.isPending) return
        let config: unknown
        try {
            config = JSON.parse(draft)
        } catch {
            notifications.show({
                title: t('common.message.error'),
                message: t('config-editor-actions.feature.failed-to-save-invalid-json'),
                color: 'red'
            })
            return
        }
        save.mutate(config)
    }

    function report(reason: unknown, title = t('common.message.error')): void {
        notifications.show({
            title,
            message: reason instanceof Error ? reason.message : t('common.message.error'),
            color: 'red'
        })
    }

    async function formatDocument(): Promise<void> {
        await current?.getAction('editor.action.formatDocument')?.run()
    }

    async function loadTemplate(text: string): Promise<void> {
        updateDraft(text)
        await tick()
        await formatDocument()
        notifications.show({
            title: t('use-download-template.template-loaded'),
            message: '',
            color: 'teal'
        })
    }

    function copy(text: string): void {
        void navigator.clipboard.writeText(text).then(() => {
            notifications.show({
                title: t('common.message.copied'),
                message: '',
                color: 'teal'
            })
        }, report)
    }
</script>

<UnsavedChanges {dirty} />

<EditorFrame>
    {#snippet children()}
        <CodeEditor
            language="json"
            modelUri={`xray-config://${profile.uuid}/config.json`}
            label={t('config-profiles-grid.widget.xray-config')}
            value={draft}
            readOnly={save.isPending}
            onchange={updateDraft}
            onready={(value) => {
                current = value
                void value?.getAction('editor.foldLevel7')?.run().catch(report)
            }}
        />
    {/snippet}
    {#snippet status()}
        <div class="status" class:invalid={!configValid} class:wasm={validator.status !== 'ready'}>
            {#if validator.status === 'idle' || validator.status === 'loading'}
                <p class="loading" role="status">
                    <span class="m_5ae2e3c m_b34414df" aria-hidden="true"></span>
                    <code
                        >Xray Core (WASM) is loading...
                        {validator.status === 'loading' ? validator.progress : 0}%</code
                    >
                </p>
            {:else if validator.status === 'failed'}
                <p role="alert">
                    <code>Xray Core (WASM) is not running. Validation is unavailable.</code>
                </p>
                <Button size="compact-xs" color="red" variant="light" onclick={startXrayValidator}
                    >{t('restart-node-button.feature.restart')}</Button
                >
            {:else if validation === undefined}
                <p role="status">Validating Xray config…</p>
            {:else if validation}
                <p role="alert">{validationTime} | {validation}</p>
            {:else}
                <p role="status">{validationTime} | Xray config is valid.</p>
            {/if}
        </div>
    {/snippet}
    {#snippet actions()}
        <EditorActions
            {current}
            value={draft}
            saving={save.isPending}
            onsave={saveConfig}
            oncopy={() =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: '',
                    color: 'teal'
                })}
            onerror={report}
            onload={() => (importOpen = true)}
            ongenerate={() => (keypairOpen = true)}
            saveDisabled={!dirty && !configValid}
            saveAnyway={configValid ? undefined : () => (confirmSave = true)}
            saveTone={dirty ? 'teal' : 'gray'}
        >
            {#snippet extra()}
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    onclick={() => {
                        copy(profile.uuid)
                    }}
                >
                    <Icon name="PiCopy" size={14} />{t('common.action.copy-uuid')}
                </button>
                <button
                    type="button"
                    role="menuitem"
                    tabindex="-1"
                    onclick={() => {
                        computedOpen = true
                    }}
                >
                    <Icon name="TbEye" size={14} />{t('config-profile-card.widget.view-computed')}
                </button>
            {/snippet}
        </EditorActions>
    {/snippet}
</EditorFrame>

{#if confirmSave}
    <Overlay
        title={t('common.action.confirm-action')}
        icon="TbAlertTriangle"
        tone="red"
        width={440}
        fullscreenOnMobile={false}
        close={() => (confirmSave = false)}
    >
        <p>{t('config-editor-actions.feature.save-anyway-description')}</p>
        <div class="button-row">
            <Button onclick={dismissContainingDialog}>{t('common.action.cancel')}</Button>
            <Button
                color="red"
                variant="filled"
                onclick={() => {
                    confirmSave = false
                    saveConfig()
                }}
                >{t('common.action.save')}</Button
            >
        </div>
    </Overlay>
{/if}

{#if computedOpen}
    <ProfilePreview {profile} computed={true} close={() => (computedOpen = false)} />
{/if}

{#if keypairOpen}
    <KeypairGenerator close={() => (keypairOpen = false)} />
{/if}

{#if importOpen}
    <TemplatePicker
        catalog={XRAY_CORE_TEMPLATE_LIST_LINK}
        type="XRAY_JSON"
        onload={loadTemplate}
        onclose={() => {
            importOpen = false
        }}
    />
{/if}

<style>
    .status {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        flex: none;
        gap: var(--mantine-spacing-xs);
        min-height: 46px;
        max-height: 30%;
        overflow-y: auto;
        padding: 10px 16px;
        border-top: 1px solid rgb(51 171 132);
        background: rgb(51 171 132 / 10%);
        color: var(--mantine-color-text);
    }
    .status.invalid {
        border-color: rgb(241 65 65);
        background: rgb(241 65 65 / 10%);
    }
    .status p {
        margin: 0;
        color: inherit;
        font-size: 0.9rem;
        line-height: 1.55;
    }
    .status.wasm {
        min-height: 0;
        gap: var(--mantine-spacing-sm);
    }
    .wasm code {
        font-family: var(--mantine-font-family-monospace);
        font-size: 0.9rem;
    }
    .loading {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .loading > span {
        --loader-size: var(--loader-size-xs);
        --loader-color: var(--mantine-color-orange-filled);
        flex: none;
    }
    @media (prefers-reduced-motion: reduce) {
        .loading > span::after {
            animation: none;
        }
    }
</style>
