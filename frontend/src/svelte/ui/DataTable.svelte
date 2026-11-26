<script lang="ts">
    import type { Snippet } from 'svelte'
    import type { HTMLTableAttributes } from 'svelte/elements'

    let {
        children,
        class: className,
        ...attributes
    }: HTMLTableAttributes & {
        children: Snippet
    } = $props()
</script>

<table {...attributes} class={['data-table', className]}>
    {@render children()}
</table>

<style>
    .data-table {
        width: 100%;
        border-collapse: separate;
        border-spacing: 0;
        font-size: var(--mantine-font-size-sm);
    }

    .data-table :global(th),
    .data-table :global(td) {
        text-align: start;
    }

    .data-table :global(th:not(:last-child)) {
        border-inline-end: 1px solid rgba(var(--gw-fg-rgb), 0.08);
    }

    .data-table :global(tbody tr:hover) {
        background-color: var(--gw-raised-bg);
    }
</style>
