<script lang="ts">
    import { setupHostJsonFields, setupHostMapperSchema } from '@shared/utils/monaco/host-schemas'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import { locale } from '../i18n.svelte'
    import type { jsonOptions } from './host-fields'

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
    let draft = $state<string>()
    let text = $derived(draft ?? value)
    let open = $state(false)
    let ready = $state(false)
    let t = $derived(locale.t)

    $effect(() => {
        if (!ready) return
        if (field === 'mapper') setupHostMapperSchema(rawInbound)
        else if (field !== 'xhttpExtraParams') void setupHostJsonFields(locale.language)
    })

    function change(next: string): void {
        draft = next
        onchange()
    }
</script>

<div class="json-field">
    {#if open}
        <div class="editor">
            <CodeEditor
                language="json"
                modelUri={`${modelSchemes[field]}://editor`}
                {label}
                value={text}
                onchange={change}
                onready={(editor) => (ready = Boolean(editor))}
            />
        </div>
        <input type="hidden" name={field} value={text}>
    {:else}
        <label>
            {label}
            <textarea
                name={field}
                value={text}
                oninput={(event) => change(event.currentTarget.value)}
                spellcheck="false"
                rows="3"
            ></textarea>
        </label>
    {/if}
    <button type="button" onclick={() => (open = !open)}>
        {open ? t('common.action.close') : t('common.action.edit')}
        · {label}
    </button>
</div>

<style>
    .json-field,
    label {
        display: grid;
        gap: 0.5rem;
    }
    button {
        justify-self: start;
    }
    .editor {
        display: flex;
        min-height: 18rem;
    }
</style>
