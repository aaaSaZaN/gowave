<script lang="ts">
    import { setupHostJsonFields, setupHostMapperSchema } from '@shared/utils/monaco/host-schemas'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import Overlay from '../ui/Overlay.svelte'
    import { type jsonOptions, parseHostJson } from './host-fields'

    let {
        field,
        label,
        value,
        rawInbound,
        onchange
    }: {
        field: keyof typeof jsonOptions
        label: string
        value: string
        rawInbound: unknown
        onchange: () => void
    } = $props()
    const modelSchemes = {
        xhttpExtraParams: 'host-xhttp-extra',
        muxParams: 'host-mux',
        sockoptParams: 'host-sockopt',
        finalMask: 'host-final-mask',
        mapper: 'host-mapper'
    }
    let saved = $state<string>()
    let text = $derived(saved ?? value)
    let draft = $state('')
    let open = $state(false)
    let error = $state('')
    let ready = $state(false)
    let t = $derived(locale.t)

    $effect(() => {
        if (!ready) return
        if (field === 'mapper') setupHostMapperSchema(rawInbound)
        else if (field !== 'xhttpExtraParams') void setupHostJsonFields(locale.language)
    })

    function edit(): void {
        draft = text
        error = ''
        open = true
    }

    function close(): void {
        open = false
        ready = false
    }

    function apply(): void {
        try {
            parseHostJson(draft, field === 'mapper')
        } catch (cause) {
            error = cause instanceof Error ? cause.message : String(cause)
            return
        }
        if (draft !== text) {
            saved = draft
            onchange()
        }
        close()
    }
</script>

<input type="hidden" name={field} value={text}>
<Button
    size="compact-sm"
    color="gray"
    variant="soft"
    icon="TbJson"
    aria-haspopup="dialog"
    aria-label={`${t('common.action.edit')} ${label}`}
    onclick={edit}
>
    {text.trim() && text.trim() !== '{}'
        ? t('common.message.configured')
        : t('common.message.not-set')}
</Button>

{#if open}
    <Overlay title={label} icon="TbJson" tone="teal" width={840} {close}>
        <div class="editor">
            <CodeEditor
                language="json"
                modelUri={`${modelSchemes[field]}://editor`}
                {label}
                value={draft}
                onchange={(next) => {
                    draft = next
                    error = ''
                }}
                onready={(editor) => (ready = Boolean(editor))}
            />
        </div>
        {#if error}
            <p role="alert">{error}</p>
        {/if}
        <div class="actions">
            <Button variant="subtle" color="gray" onclick={() => (draft = '')}>
                {t('common.action.clear')}
            </Button>
            <Button onclick={dismissContainingDialog}>{t('common.action.cancel')}</Button>
            <Button color="teal" variant="soft" icon="PiFloppyDiskDuotone" onclick={apply}>
                {t('common.action.save')}
            </Button>
        </div>
    </Overlay>
{/if}

<style>
    .editor {
        display: flex;
        height: min(45vh, 28rem);
        min-height: 12rem;
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
        margin-block-start: var(--mantine-spacing-sm);
    }
</style>
