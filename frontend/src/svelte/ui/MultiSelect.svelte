<script lang="ts">
    import { tick } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { nextComboboxOption } from './combobox'
    import Icon from './Icon.svelte'
    import TablePopup from './TablePopup.svelte'

    let {
        name,
        label,
        placeholder = '',
        hideLabel = false,
        create = false,
        options,
        values = $bindable(),
        onchange
    }: {
        name: string
        label: string
        placeholder?: string
        hideLabel?: boolean
        create?: boolean
        options: readonly {
            value: string
            label: string
            name?: string
            flag?: string
            badge?: string
        }[]
        values: string[]
        onchange: () => void
    } = $props()

    const id = $props.id()
    let anchor = $state<HTMLDivElement>()
    let input: HTMLInputElement
    let draft = $state('')
    let open = $state(false)
    let active = $state<string>()
    const pending = $derived(draft.trim())
    const entered = $derived(create && pending !== '' && !values.includes(pending))
    const filtered = $derived.by(() => {
        const matches = options.filter(
            (option) =>
                (!create || !values.includes(option.value)) &&
                option.label
                    .toLocaleLowerCase(locale.language)
                    .includes(pending.toLocaleLowerCase(locale.language))
        )
        return entered && !matches.some(({ value }) => value === pending)
            ? [...matches, { value: pending, label: `+ ${pending}` }]
            : matches
    })

    function optionId(option: string): string {
        return `${id}-option-${encodeURIComponent(option)}`
    }

    function update(next: string[]): void {
        values = next
        onchange()
    }

    function close(): void {
        if (entered) update([...values, pending])
        draft = ''
        active = undefined
        open = false
    }

    function select(value: string): void {
        update(
            values.includes(value) ? values.filter((item) => item !== value) : [...values, value]
        )
        draft = ''
        active = undefined
        input.focus({ preventScroll: true })
    }

    function keys(event: KeyboardEvent): void {
        if (event.isComposing) return
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault()
            open = true
            active = nextComboboxOption(event.key, active, filtered)
            if (active !== undefined) {
                const option = optionId(active)
                void tick().then(() =>
                    document.getElementById(option)?.scrollIntoView({ block: 'nearest' })
                )
            }
        } else if (open && event.key === 'Enter') {
            event.preventDefault()
            if (active !== undefined || entered) select(active ?? pending)
        } else if (open && event.key === 'Escape') {
            event.preventDefault()
            event.stopPropagation()
            draft = ''
            active = undefined
            open = false
        } else if (event.key === 'Backspace' && !draft && values.length) {
            update(values.slice(0, -1))
        }
    }
</script>

<div class="multi-select">
    {#if !hideLabel}
        <label for={id}>{label}</label>
    {/if}
    <div class="control" bind:this={anchor}>
        {#each entered ? [...values, pending] : values as value (value)}
            <input type="hidden" {name} {value}>
        {/each}
        {#each values as value (value)}
            {@const caption = options.find((option) => option.value === value)?.label ?? value}
            <span class="pill">
                <button
                    type="button"
                    class="mantine-UnstyledButton-root"
                    aria-label={`${locale.t('common.action.remove')} ${caption}`}
                    onmousedown={(event) => event.preventDefault()}
                    onclick={() => select(value)}
                >
                    <Icon name="TbX" size={12} />
                </button>
                {caption}
            </span>
        {/each}
        <input
            {id}
            bind:this={input}
            aria-label={hideLabel ? label : undefined}
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={open}
            aria-controls={open ? `${id}-options` : undefined}
            aria-activedescendant={open && active !== undefined ? optionId(active) : undefined}
            autocomplete="off"
            placeholder={values.length ? '' : placeholder}
            value={draft}
            onfocus={() => (open = true)}
            onclick={() => (open = true)}
            oninput={(event) => {
                draft = event.currentTarget.value
                open = true
                active = undefined
                if (create) onchange()
            }}
            onblur={close}
            onkeydown={keys}
        >
        {#if values.length}
            <button
                type="button"
                class="clear mantine-UnstyledButton-root"
                aria-label={locale.t('common.action.clear')}
                onmousedown={(event) => event.preventDefault()}
                onclick={() => {
                    draft = ''
                    active = undefined
                    update([])
                }}
            >
                <Icon name="TbX" size={16} />
            </button>
        {:else}
            <span class="arrow" aria-hidden="true"><Icon name="TbSelector" size={14} /></span>
        {/if}
    </div>
    {#if open && anchor}
        <TablePopup
            {anchor}
            id={`${id}-options`}
            {label}
            role="listbox"
            multiple
            matchWidth
            maxHeight={230}
            keyboardNavigation={false}
            {close}
        >
            {#each filtered as { value, label: caption, name: itemName, flag: itemFlag, badge } (value)}
                <button
                    type="button"
                    id={optionId(value)}
                    class="option m_92253aa5 m_b2821a6e mantine-UnstyledButton-root"
                    class:keyboard-active={active === value}
                    role="option"
                    aria-selected={values.includes(value)}
                    tabindex="-1"
                    onmousedown={(event) => event.preventDefault()}
                    onclick={() => select(value)}
                >
                    {#if !create}
                        <span
                            class="check"
                            class:checked={values.includes(value)}
                            aria-hidden="true"
                        >
                            {#if values.includes(value)}
                                <Icon name="TbCheck" size={14} />
                            {/if}
                        </span>
                    {/if}
                    {#if itemFlag}
                        <span class="opt-flag" aria-hidden="true">{itemFlag}</span>
                    {/if}
                    <span class="opt-label">{itemName ?? caption}</span>
                    {#if badge}
                        <span class="opt-badge">{badge}</span>
                    {/if}
                </button>
            {:else}
                <div class="empty">{locale.t('common.message.nothing-found')}</div>
            {/each}
        </TablePopup>
    {/if}
</div>

<style>
    label {
        display: block;
        font-size: var(--mantine-font-size-sm);
        font-weight: var(--mantine-font-weight-medium);
        line-height: var(--mantine-line-height-sm);
    }
    .control {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: 6px;
        min-height: 36px;
        padding: 5px 8px 5px 12px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
    }
    .control:focus-within {
        border-color: var(--mantine-color-cyan-7);
    }
    input:not([type="hidden"]) {
        width: 0;
        min-width: 64px;
        flex: 1;
        min-height: 22px;
        padding: 0;
        border: 0;
        outline: none;
        background: transparent;
        font-size: var(--mantine-font-size-sm);
    }
    .pill {
        display: inline-flex;
        align-items: center;
        gap: 2px;
        min-height: 20px;
        padding-inline: 2px 8px;
        border: 1px solid rgba(21, 170, 191, 0.25);
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-cyan-light);
        color: var(--mantine-color-cyan-light-color);
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
        overflow-wrap: anywhere;
    }
    .pill button {
        color: var(--mantine-color-cyan-4);
    }
    .opt-flag {
        font-size: 1.05em;
        line-height: 1;
        flex: 0 0 auto;
    }
    .opt-label {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        text-align: start;
    }
    .opt-badge {
        flex: 0 0 auto;
        padding: 1px 6px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
        font-size: 10px;
        font-weight: 600;
        text-transform: uppercase;
    }
    .pill button,
    .clear,
    .arrow {
        display: grid;
        width: 18px;
        height: 18px;
        flex: none;
        place-items: center;
    }
    .clear,
    .arrow {
        color: var(--mantine-color-dimmed);
    }
    .pill button:hover,
    .clear:hover {
        color: var(--mantine-color-bright);
    }
    .option {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        width: 100%;
    }
    .option.keyboard-active {
        background: var(--mantine-color-default-hover);
    }
    .check {
        display: grid;
        width: 18px;
        height: 18px;
        flex: none;
        place-items: center;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-xs);
        background: var(--mantine-color-default);
    }
    .check.checked {
        border-color: var(--mantine-color-cyan-6);
        color: var(--mantine-color-bright);
    }
    .empty {
        padding: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
</style>
