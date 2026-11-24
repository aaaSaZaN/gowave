<script lang="ts">
    import { tick } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { nextComboboxOption } from './combobox'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'
    import TablePopup from './TablePopup.svelte'

    let {
        name,
        label,
        description,
        size = 'sm',
        leadingSize = 16,
        emptyText,
        disabled,
        placeholder,
        options,
        leading,
        searchable = true,
        clearable = true,
        alwaysShowClear = false,
        hideLabel = false,
        required = false,
        onchoose,
        value = $bindable('')
    }: {
        name?: string
        label: string
        description?: string | undefined
        size?: 'sm' | 'md'
        leadingSize?: number
        emptyText?: string
        disabled?: boolean
        placeholder: string
        options: readonly {
            value: string
            label: string
            flag?: string
            avatar?: { image: string | null; initials: string }
        }[]
        leading?: IconName
        searchable?: boolean
        clearable?: boolean
        alwaysShowClear?: boolean
        hideLabel?: boolean
        required?: boolean
        onchoose?: (value: string) => void
        value?: string
    } = $props()

    const id = $props.id()
    let anchor = $state<HTMLInputElement>()
    let open = $state(false)
    let search = $state<string>()
    let active = $state<string>()
    const unavailable = $derived(disabled ?? !options.length)
    const selected = $derived(options.find((option) => option.value === value))
    const filtered = $derived(
        options.filter((option) =>
            option.label
                .toLocaleLowerCase(locale.language)
                .includes((search ?? '').toLocaleLowerCase(locale.language))
        )
    )

    function highlight(next: string | undefined): void {
        active = next
        if (next === undefined) return
        void tick().then(() => {
            if (open && active === next)
                document.getElementById(optionId(next))?.scrollIntoView({ block: 'nearest' })
        })
    }

    function optionId(option: string): string {
        return `${id}-option-${encodeURIComponent(option)}`
    }

    function show(): void {
        if (open) return
        search = undefined
        open = true
        highlight(selected?.value)
    }

    function choose(next: string): void {
        const selection = next === value && clearable ? '' : next
        if (onchoose) onchoose(selection)
        else value = selection
        open = false
        search = undefined
        anchor?.focus({ preventScroll: true })
    }

    function handleKeys(event: KeyboardEvent): void {
        if (event.isComposing) return
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault()
            if (open) highlight(nextComboboxOption(event.key, active, filtered))
            else show()
        } else if (event.key === 'Enter' && open && active !== undefined) {
            event.preventDefault()
            choose(active)
        } else if (event.key === 'Escape' && open) {
            event.preventDefault()
            event.stopPropagation()
            open = false
        }
    }
</script>

<div class="m_46b77525 select-field">
    {#if !hideLabel}
        <span id={`${id}-label`} class="m_8fdc1311">
            {label}
            {#if required}
                <span class="m_78a94662"> *</span>
            {/if}
        </span>
    {/if}
    {#if description}
        <div id={`${id}-description`} class="m_fe47ce59">{description}</div>
    {/if}
    <div
        class="m_6c018570 select-input"
        class:medium={size === 'md'}
        data-variant="default"
        data-with-left-section={selected?.flag || selected?.avatar || leading ? true : undefined}
        data-with-right-section
    >
        {#if name}
            <input type="hidden" {name} {value} disabled={unavailable}>
        {/if}
        <input
            {id}
            bind:this={anchor}
            class="m_8fb7ebe7 mantine-Input-input"
            role="combobox"
            aria-labelledby={hideLabel ? undefined : `${id}-label`}
            aria-label={hideLabel ? label : undefined}
            aria-autocomplete={searchable ? 'list' : 'none'}
            aria-haspopup="listbox"
            aria-activedescendant={open && active !== undefined ? optionId(active) : undefined}
            aria-controls={open ? `${id}-options` : undefined}
            aria-describedby={description ? `${id}-description` : undefined}
            aria-expanded={open}
            aria-required={required || undefined}
            disabled={unavailable}
            readonly={!searchable}
            {placeholder}
            value={(open && search !== undefined ? search : selected?.label) ?? ''}
            onclick={() => {
                if (open && !searchable) open = false
                else show()
            }}
            oninput={(event) => {
                show()
                search = event.currentTarget.value
                active = undefined
            }}
            onblur={() => (open = false)}
            onkeydown={handleKeys}
        >
        {#if leading || selected?.flag || selected?.avatar}
            <span class="m_82577fc2" data-position="left" aria-hidden="true">
                {#if leading}
                    <Icon name={leading} size={leadingSize} />
                {:else if selected?.avatar}
                    {@render avatar(selected.avatar, 16)}
                {:else}
                    {selected?.flag}
                {/if}
            </span>
        {/if}
        {#if clearable && (selected || alwaysShowClear)}
            <button
                type="button"
                class="m_82577fc2 clear mantine-UnstyledButton-root"
                data-position="right"
                aria-label={locale.t('common.action.clear')}
                disabled={!selected}
                onclick={() => choose(value)}
            >
                <Icon name="TbX" size={14} />
            </button>
        {/if}
        {#if alwaysShowClear || !(clearable && selected)}
            <span class="m_82577fc2 arrow" data-position="right" aria-hidden="true"
                ><Icon name="TbSelector" size={14} /></span
            >
        {/if}
    </div>
    {#if open && anchor}
        <TablePopup
            {anchor}
            id={`${id}-options`}
            {label}
            role="listbox"
            matchWidth
            maxHeight={230}
            keyboardNavigation={false}
            close={() => (open = false)}
        >
            {#each filtered as item (item.value)}
                <button
                    type="button"
                    id={optionId(item.value)}
                    role="option"
                    tabindex="-1"
                    data-combobox-selected={item.value === active ? '' : undefined}
                    onmousedown={(event) => event.preventDefault()}
                    aria-selected={item.value === value}
                    class="m_92253aa5 m_b2821a6e option mantine-UnstyledButton-root"
                    onclick={() => choose(item.value)}
                >
                    {#if item.value === value}
                        <Icon name="TbCheck" size={14} />
                    {/if}
                    {#if item.flag}
                        <span aria-hidden="true">{item.flag}</span>
                    {:else if item.avatar}
                        {@render avatar(item.avatar, 20)}
                    {/if}
                    {item.label}
                </button>
            {:else}
                <div class="empty">{emptyText ?? locale.t('common.message.nothing-found')}</div>
            {/each}
        </TablePopup>
    {/if}
</div>

{#snippet avatar(data: { image: string | null; initials: string }, size: number)}
    <span class="avatar" style:--avatar-size={`${size}px`} aria-hidden="true">
        {data.initials}
        {#if data.image}
            <img
                src={data.image}
                alt=""
                loading="lazy"
                decoding="async"
                onerror={({ currentTarget }) => {
                    if (currentTarget instanceof HTMLImageElement) currentTarget.hidden = true
                }}
                onload={({ currentTarget }) => {
                    if (
                        currentTarget instanceof HTMLImageElement &&
                        currentTarget.naturalWidth <= 16 &&
                        currentTarget.naturalHeight <= 16
                    )
                        currentTarget.hidden = true
                }}
            >
        {/if}
    </span>
{/snippet}

<style>
    .select-field {
        min-width: 0;
        flex: 1;
    }
    .select-input {
        --input-fz: var(--mantine-font-size-sm);
    }
    .select-input.medium {
        --input-height: var(--input-height-md);
        --input-fz: var(--mantine-font-size-md);
    }
    .select-field > .m_8fdc1311 {
        font-weight: var(--select-label-weight, var(--mantine-font-weight-medium));
    }
    .select-field:has(.m_fe47ce59) .select-input {
        margin-block-start: 5px;
    }
    .arrow {
        pointer-events: none;
    }
    input[readonly]:not(:disabled) {
        cursor: pointer;
    }
    .select-field :global(.table-popup) {
        box-shadow: none;
    }
    .select-field :global(.table-popup:not(:hover)) {
        scrollbar-color: transparent transparent;
    }
    .clear {
        width: 35px;
    }
    .option {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        width: 100%;
    }
    .avatar {
        position: relative;
        display: inline-grid;
        width: var(--avatar-size);
        height: var(--avatar-size);
        flex: none;
        place-items: center;
        border-radius: var(--mantine-radius-sm);
        color: var(--mantine-color-blue-light-color);
        background: var(--mantine-color-blue-light);
        overflow: hidden;
        font-size: calc(var(--avatar-size) * 0.4);
        font-weight: 700;
    }
    .avatar img {
        position: absolute;
        inset: 0;
        width: 100%;
        height: 100%;
        object-fit: cover;
        background: var(--mantine-color-body);
    }
    .empty {
        padding: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
</style>
