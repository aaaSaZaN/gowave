<script lang="ts">
    import { locale } from '../i18n.svelte'
    import Icon from '../ui/Icon.svelte'

    let {
        values = $bindable(),
        suggestions,
        label,
        onchange
    }: {
        values: string[]
        suggestions: readonly string[]
        label: string
        onchange: () => void
    } = $props()
    const choices = $props.id()
    let input: HTMLInputElement
    let draft = $state('')
    const separator = /[,;\s]+/
    let pending = $derived(draft.toUpperCase().split(separator).filter(Boolean))
    let submitted = $derived([
        ...values,
        ...[...new Set(pending)]
            .filter((tag) => !values.includes(tag))
            .slice(0, Math.max(0, 10 - values.length))
    ])

    export function clear(): void {
        draft = ''
        values = []
        onchange()
    }

    function commit(): void {
        values = submitted
        draft = ''
    }

    function remove(tag: string): void {
        values = values.filter((value) => value !== tag)
        onchange()
        input.focus()
    }
</script>

<div class="tags-control">
    <input type="hidden" name="tags" value={submitted.join(',')}>
    {#each values as tag (tag)}
        <span class="tag-pill">
            <button
                type="button"
                class="mantine-UnstyledButton-root"
                aria-label={`${locale.t('common.action.remove')} ${tag}`}
                onclick={() => remove(tag)}
            >
                <Icon name="TbX" size={12} />
            </button>
            {tag}
        </span>
    {/each}
    <input
        bind:this={input}
        list={choices}
        aria-label={label}
        autocomplete="off"
        disabled={values.length >= 10}
        placeholder="Enter tags (comma, space, semicolon)"
        value={draft}
        oninput={(event) => {
            draft = event.currentTarget.value
            if (/[,;\s]$/.test(draft)) commit()
            onchange()
        }}
        onblur={commit}
        onkeydown={(event) => {
            if (event.key === 'Enter' && draft.trim()) {
                event.preventDefault()
                commit()
            }
        }}
    >
    <datalist id={choices}>
        {#each suggestions.filter((tag) => !values.includes(tag)) as tag (tag)}
            <option value={tag}></option>
        {/each}
    </datalist>
    {#if values.length || draft}
        <button
            type="button"
            class="mantine-UnstyledButton-root clear"
            aria-label={locale.t('common.action.clear')}
            onclick={() => {
                clear()
                input.focus()
            }}
        >
            <Icon name="TbX" size={16} />
        </button>
    {/if}
</div>

<style>
    .tags-control {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: 6px;
        min-height: 36px;
        padding: 5px 8px 5px 12px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-6);
    }
    .tags-control:focus-within {
        border-color: var(--mantine-color-cyan-6);
    }
    input:not([type="hidden"]) {
        width: 0;
        min-width: 64px;
        flex: 1;
        padding: 0;
        border: 0;
        min-height: 22px;
        font-size: var(--mantine-font-size-sm);
        outline: none;
        background: transparent;
    }
    input::-webkit-calendar-picker-indicator {
        display: none;
    }
    .tag-pill {
        display: inline-flex;
        align-items: center;
        padding-inline-end: 9px;
        height: 20px;
        border: 1px solid rgba(21, 170, 191, 0.3);
        border-radius: var(--mantine-radius-sm);
        background: linear-gradient(135deg, rgba(21, 170, 191, 0.15), rgba(16, 152, 173, 0.1));
        color: var(--mantine-color-cyan-4);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        letter-spacing: 0.25px;
    }
    button {
        display: grid;
        width: 18px;
        height: 18px;
        place-items: center;
    }
    button:hover {
        color: var(--mantine-color-bright);
    }
    .clear {
        color: var(--mantine-color-dimmed);
    }
</style>
