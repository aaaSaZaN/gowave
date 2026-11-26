<script lang="ts">
    import type { Snippet } from 'svelte'
    import Icon from './Icon.svelte'

    let {
        heading,
        children,
        open = $bindable(false)
    }: { heading: Snippet; children: Snippet; open?: boolean } = $props()
</script>

<details bind:open>
    <summary>
        <span>{@render heading()}</span>
        <span class="chevron" aria-hidden="true"><Icon name="TbChevronDown" size={16} /></span>
    </summary>
    <div class="content">{@render children()}</div>
</details>

<style>
    details {
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }

    summary {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm);
        font-weight: 600;
        list-style: none;
        cursor: pointer;
    }

    summary::-webkit-details-marker {
        display: none;
    }

    .chevron {
        display: flex;
        color: var(--mantine-color-dimmed);
    }

    details[open] .chevron {
        rotate: 180deg;
    }

    .content {
        padding: var(--mantine-spacing-sm);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.08);
    }
</style>
