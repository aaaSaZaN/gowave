<script lang="ts">
    import { locale } from '../i18n.svelte'
    import { nextComboboxOption } from '../ui/combobox'
    import Icon from '../ui/Icon.svelte'
    import TablePopup from '../ui/TablePopup.svelte'

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
    let anchor = $state<HTMLDivElement>()
    let input: HTMLInputElement
    let open = $state(false)
    let active = $state<string>()
    const options = $derived(
        [...new Set(suggestions)]
            .filter((tag) => !values.includes(tag) && tag.includes(draft.trim().toUpperCase()))
            .map((value) => ({ value }))
    )
    const expanded = $derived(open && options.length > 0 && values.length < 10)

    function optionId(value: string): string {
        return `${choices}-${encodeURIComponent(value)}`
    }

    let draft = $state('')
    const separator = /[,;\s]+/
    let pending = $derived(draft.toUpperCase().split(separator).filter(Boolean))
    let submitted = $derived([
        ...values,
        ...[...new Set(pending)]
            .filter((tag) => !values.includes(tag))
            .slice(0, Math.max(0, 10 - values.length))
    ])

    function clear(): void {
        draft = ''
        values = []
        onchange()
    }

    function commit(): void {
        values = submitted
        draft = ''
        active = undefined
    }

    function choose(tag: string): void {
        draft = tag
        commit()
        open = false
        onchange()
    }

    function keys(event: KeyboardEvent): void {
        if (event.isComposing) return
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            if (options.length === 0) return
            event.preventDefault()
            open = true
            active = nextComboboxOption(event.key, active, options)
            if (active)
                document.getElementById(optionId(active))?.scrollIntoView({ block: 'nearest' })
        } else if (event.key === 'Enter' && (draft.trim() || active)) {
            event.preventDefault()
            if (active) choose(active)
            else commit()
        } else if (event.key === 'Escape' && expanded) {
            event.preventDefault()
            event.stopPropagation()
            open = false
            active = undefined
        }
    }

    function remove(tag: string): void {
        values = values.filter((value) => value !== tag)
        onchange()
        input.focus()
    }
</script>

<div class="tags-control" bind:this={anchor}>
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
        aria-label={label}
        role="combobox"
        aria-expanded={expanded}
        aria-autocomplete="list"
        aria-controls={expanded ? choices : undefined}
        aria-activedescendant={expanded && active ? optionId(active) : undefined}
        autocomplete="off"
        disabled={values.length >= 10}
        placeholder="Enter tags (comma, space, semicolon)"
        value={draft}
        onfocus={() => (open = true)}
        onclick={() => (open = true)}
        oninput={(event) => {
            draft = event.currentTarget.value
            if (/[,;\s]$/.test(draft)) commit()
            open = true
            active = undefined
            onchange()
        }}
        onblur={() => {
            commit()
            open = false
        }}
        onkeydown={keys}
    >
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

{#if expanded && anchor}
    <TablePopup
        {anchor}
        id={choices}
        {label}
        role="listbox"
        matchWidth
        maxHeight={230}
        keyboardNavigation={false}
        close={() => {
            open = false
            active = undefined
        }}
    >
        {#each options as { value } (value)}
            <button
                type="button"
                id={optionId(value)}
                role="option"
                class="option m_92253aa5 m_b2821a6e mantine-UnstyledButton-root"
                class:keyboard-active={active === value}
                aria-selected={active === value}
                tabindex="-1"
                onmousedown={(event) => event.preventDefault()}
                onclick={() => choose(value)}
            >
                {value}
            </button>
        {/each}
    </TablePopup>
{/if}

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
    .tags-control button {
        display: grid;
        width: 18px;
        height: 18px;
        place-items: center;
    }
    .tags-control button:hover {
        color: var(--mantine-color-bright);
    }
    .option {
        width: 100%;
        text-align: start;
    }
    .option.keyboard-active {
        background: var(--mantine-color-default-hover);
    }
    .clear {
        color: var(--mantine-color-dimmed);
    }
</style>
