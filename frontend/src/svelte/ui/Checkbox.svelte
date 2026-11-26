<script lang="ts">
    import type { Snippet } from 'svelte'
    import type { HTMLInputAttributes } from 'svelte/elements'

    let {
        checked = $bindable(false),
        indeterminate = $bindable(false),
        label,
        end,
        id: inputId,
        ...attributes
    }: Omit<HTMLInputAttributes, 'type' | 'children'> & { label?: string; end?: Snippet } = $props()
    const generatedId = $props.id()
    const id = $derived(inputId ?? generatedId)
</script>

{#if label}
    <label for={id}>
        <input {...attributes} {id} type="checkbox" bind:checked bind:indeterminate>
        <span>{label}</span>
        {@render end?.()}
    </label>
{:else}
    <input {...attributes} {id} type="checkbox" bind:checked bind:indeterminate>
{/if}

<style>
    label {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        cursor: pointer;
    }

    label:has(input:disabled) {
        cursor: not-allowed;
    }
</style>
