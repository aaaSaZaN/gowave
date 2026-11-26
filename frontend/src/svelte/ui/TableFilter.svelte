<script lang="ts">
    import SelectField from './SelectField.svelte'

    let {
        label,
        value,
        options,
        type = 'text',
        live = false,
        onfilter
    }: {
        label: string
        value: string
        options?: readonly { value: string; label: string }[] | undefined
        type?: 'text' | 'number'
        live?: boolean
        onfilter: (value: string) => void
    } = $props()
</script>

<div class="table-filter">
    {#if options}
        <SelectField
            {label}
            {value}
            {options}
            placeholder="Filter by..."
            hideLabel
            searchable={false}
            alwaysShowClear
            onchoose={onfilter}
        />
    {:else}
        <input
            class="mantine-Input-input"
            aria-label={`Filter by ${label}`}
            placeholder="Filter by..."
            {type}
            {value}
            oninput={(event) => {
                if (live) onfilter(event.currentTarget.value)
            }}
            onchange={(event) => {
                if (!live) onfilter(event.currentTarget.value)
            }}
            onkeydown={(event) => {
                if (event.key === 'Enter') event.currentTarget.blur()
            }}
        >
    {/if}
</div>

<style>
    .table-filter {
        flex: 1;
        min-width: 0;
    }

    .table-filter :global(.select-input) {
        --input-height: 36px;
        border: 0;
        background: transparent;
    }

    .table-filter :global(input.mantine-Input-input) {
        width: 100%;
        min-height: 36px;
        padding: 0;
        border: 0;
        border-block-end: 2px solid var(--mantine-color-default-border);
        border-radius: 0;
        background: transparent;
        color: inherit;
        font: inherit;
        font-weight: 400;
        appearance: textfield;
    }

    .table-filter :global(input.mantine-Input-input:focus) {
        border-block-end-color: var(--mantine-primary-color-filled);
    }

    .table-filter :global(input.mantine-Input-input::placeholder) {
        color: var(--mantine-color-placeholder);
    }

    input::-webkit-inner-spin-button,
    input::-webkit-outer-spin-button {
        appearance: none;
        margin: 0;
    }

    .table-filter :global(.select-input input.mantine-Input-input) {
        padding-inline-end: 38px;
    }

    .table-filter :global(.select-input .clear[data-position="right"]) {
        inset-inline-end: 19px;
        width: 18px;
    }

    .table-filter :global(.select-input .arrow[data-position="right"]) {
        width: 18px;
    }
</style>
