<script lang="ts">
    import { tick } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { nextComboboxOption } from '../ui/combobox'
    import Icon from '../ui/Icon.svelte'
    import TablePopup from '../ui/TablePopup.svelte'

    let {
        id,
        label,
        options,
        selected,
        multiple = false,
        searchable = false,
        change
    }: {
        id: string
        label: string
        options: { value: string; label: string; flag?: string }[]
        selected: string[]
        multiple?: boolean
        searchable?: boolean
        change: (value: string | string[]) => void
    } = $props()
    const popupId = $props.id()
    let trigger = $state<HTMLInputElement>()
    let open = $state(false)
    let search = $state('')
    let active = $state<string>()
    let selectedLabel = $derived(
        options.find(({ value }) => value === selected[0])?.label ?? selected[0] ?? ''
    )
    let shown = $derived(
        options.filter((option) =>
            option.label
                .toLocaleLowerCase(locale.language)
                .includes(search.toLocaleLowerCase(locale.language))
        )
    )

    function optionId(value: string): string {
        return `${popupId}-${encodeURIComponent(value)}`
    }

    function highlight(next: string | undefined): void {
        active = next
        if (next === undefined) return
        void tick().then(() => {
            if (open && active === next)
                document.getElementById(optionId(next))?.scrollIntoView({ block: 'nearest' })
        })
    }

    function show(): void {
        search = ''
        open = true
        highlight(selected[0])
    }

    function choose(value: string): void {
        if (multiple) {
            change(
                selected.includes(value)
                    ? selected.filter((item) => item !== value)
                    : [...selected, value]
            )
            search = ''
        } else {
            change(value)
            open = false
        }
        trigger?.focus({ preventScroll: true })
    }
</script>

<div class="filter-choice" class:has-count={multiple && selected.length > 0}>
    {#if multiple && selected.length}
        <span class="count">{selected.length}</span>
    {/if}
    <input
        {id}
        bind:this={trigger}
        class="mantine-Input-input"
        aria-label="Filter by..."
        role="combobox"
        aria-haspopup="listbox"
        aria-activedescendant={open && active !== undefined ? optionId(active) : undefined}
        aria-expanded={open}
        aria-controls={open ? popupId : undefined}
        aria-autocomplete={searchable ? 'list' : 'none'}
        readonly={!searchable}
        placeholder={multiple && selected.length ? '' : 'Filter by...'}
        value={searchable && open ? search : multiple ? '' : selectedLabel}
        onclick={() => {
            if (!open) show()
            else if (!searchable) open = false
        }}
        oninput={(event) => {
            if (!open) show()
            search = event.currentTarget.value
            active = undefined
        }}
        onblur={() => (open = false)}
        onkeydown={(event) => {
            if (event.isComposing) return
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault()
                if (open) highlight(nextComboboxOption(event.key, active, shown))
                else show()
            } else if (event.key === 'Enter' && open && active !== undefined) {
                event.preventDefault()
                choose(active)
            } else if (event.key === 'Escape' && open) {
                event.preventDefault()
                event.stopPropagation()
                open = false
            }
        }}
    >
    {#if selected.length}
        <button
            type="button"
            class="clear mantine-UnstyledButton-root"
            aria-label={`Clear ${label.toLowerCase()}`}
            onclick={() => change(multiple ? [] : '')}
        >
            <Icon name="TbX" size={14} />
        </button>
    {:else}
        <span class="caret"><Icon name="TbSelector" size={14} /></span>
    {/if}
</div>

{#if open && trigger}
    <TablePopup
        anchor={trigger}
        role="listbox"
        {label}
        id={popupId}
        matchWidth
        keyboardNavigation={false}
        {multiple}
        close={() => (open = false)}
    >
        {#each shown as option (option.value)}
            {@const chosen = selected.includes(option.value)}
            <button
                type="button"
                role="option"
                id={optionId(option.value)}
                tabindex="-1"
                aria-selected={chosen}
                data-combobox-selected={option.value === active ? '' : undefined}
                onmousedown={(event) => event.preventDefault()}
                class="option m_92253aa5 m_b2821a6e mantine-UnstyledButton-root"
                onclick={() => choose(option.value)}
            >
                {#if chosen}
                    <Icon name="TbCheck" size={16} />
                {/if}
                {#if option.flag}
                    <span aria-hidden="true">{option.flag}</span>
                {/if}
                {option.label}
            </button>
        {:else}
            <div class="empty">{locale.t('common.message.nothing-found')}</div>
        {/each}
    </TablePopup>
{/if}

<style>
    .filter-choice {
        position: relative;
        font-weight: 400;
    }
    input {
        display: block;
        width: 100%;
        min-width: 0;
        height: 35px;
        padding: 0 24px 2px 0;
        border: 0;
        border-bottom: 2px solid var(--mantine-color-default-border);
        border-radius: 0;
        outline-offset: -2px;
        background: transparent;
        color: var(--mantine-color-text);
        font: inherit;
        text-overflow: ellipsis;
    }
    input:read-only {
        cursor: pointer;
    }
    input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    .caret,
    .count,
    .clear {
        position: absolute;
        inset-block-start: 50%;
        transform: translateY(-50%);
        display: grid;
        place-items: center;
    }
    .caret,
    .clear {
        inset-inline-end: 4px;
        color: var(--mantine-color-dimmed);
    }
    .caret {
        pointer-events: none;
    }
    .clear {
        width: 22px;
        height: 22px;
    }
    .count {
        inset-inline-start: 0;
        min-width: 22px;
        border-radius: 100vw;
        color: var(--mantine-color-cyan-light-color);
        background: var(--mantine-color-cyan-light);
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
    }
    .has-count input {
        padding-inline-start: 30px;
    }
    .option {
        display: flex;
        width: 100%;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        text-align: start;
        white-space: nowrap;
    }
    .empty {
        padding: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        text-align: center;
    }
</style>
