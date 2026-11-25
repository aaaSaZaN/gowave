<script lang="ts">
    import isSvg from '@shared/lib/is-svg'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'

    let { library = $bindable() }: { library: Record<string, string> } = $props()
    let t = $derived(locale.t)
    let key = $state('')
    let selected = $state('')
    let code = $state('')
    let error = $state('')
    let keys = $derived(Object.keys(library).sort())
    let preview = $derived(library[selected])

    function select(value: string): void {
        selected = value
        key = value
        code = library[value] ?? ''
        error = ''
    }

    function apply(event: SubmitEvent): void {
        event.preventDefault()
        const name = key.trim()
        if (!/^[A-Za-z]+$/.test(name)) {
            error = t('svg-library-modal.component.only-latin-characters-no-spaces')
            return
        }
        if (!isSvg(code)) {
            error = 'Enter a valid SVG document'
            return
        }
        if (name !== selected && Object.hasOwn(library, name)) {
            error = 'An icon with that name already exists'
            return
        }
        library = { ...library, [name]: code }
        select(name)
    }

    function remove(): void {
        if (!selected || !window.confirm(t('common.message.confirm-action-description'))) return
        const updated = { ...library }
        delete updated[selected]
        library = updated
        select('')
    }
</script>

<form onsubmit={apply}>
    <label
        >{t('common.field.svg-icon')}
        <select value={selected} onchange={(event) => select(event.currentTarget.value)}>
            <option value="">{t('common.action.add')}</option>
            {#each keys as item (item)}
                <option value={item}>{item}</option>
            {/each}
        </select>
    </label>
    <label>{t('common.field.name')} <input required bind:value={key}></label>
    <label
        >{t('svg-library-modal.component.svg-code')}
        <textarea class="svg-code" required rows="6" bind:value={code}></textarea></label
    >
    {#if error}
        <p role="alert">{error}</p>
    {/if}
    {#if selected && preview}
        <img
            alt={selected}
            width="40"
            height="40"
            src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(preview)}`}
        >
    {/if}
    <div class="button-row">
        <Button type="submit" variant="light">
            {selected && selected === key ? t('common.action.update') : t('common.action.add')}
        </Button>
        <Button type="button" variant="default" disabled={!selected} onclick={remove}>
            {t('common.action.remove')}
        </Button>
    </div>
</form>

<style>
    form {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    label {
        display: grid;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .svg-code {
        min-height: 180px;
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        line-height: 1.5;
        resize: vertical;
    }
    .button-row {
        display: flex;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
    }
</style>
