<script lang="ts">
    import { formatInt } from '@shared/utils/misc/number'
    import Icon from './Icon.svelte'

    let {
        items,
        emptyText,
        maxHeight,
        format = formatInt
    }: {
        items: { name: string; total: number; color: string; href?: string; flag?: string }[]
        emptyText: string
        maxHeight: number
        format?: (value: number) => string
    } = $props()
    const largest = $derived(items.reduce((max, item) => Math.max(max, item.total), 1))
</script>

{#if items.length}
    <ol style:max-height={`${maxHeight}px`}>
        {#each items as item (item)}
            <li style:--ink={item.color} style:--share={`${(item.total / largest) * 100}%`}>
                {#snippet content()}
                    {#if item.flag}
                        <span class="flag" aria-hidden="true">{item.flag}</span>
                    {/if}
                    <span class="name" title={item.name}>{item.name}</span>
                    <strong>{format(item.total)}</strong>
                {/snippet}
                {#if item.href}
                    <a class="rank" href={item.href}>{@render content()}</a>
                {:else}
                    <div class="rank">{@render content()}</div>
                {/if}
            </li>
        {/each}
    </ol>
{:else}
    <div class="empty">
        <span class="empty-symbol"><Icon name="PiEmpty" size={32} /></span>
        <span>{emptyText}</span>
    </div>
{/if}

<style>
    ol {
        display: grid;
        align-content: start;
        min-height: 224px;
        gap: 6px;
        margin: 0;
        padding: 0;
        overflow: auto;
        list-style: none;
    }
    .rank {
        position: relative;
        display: flex;
        align-items: center;
        gap: 10px;
        padding: var(--mantine-spacing-xs);
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-default-hover);
        color: inherit;
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        font-weight: 600;
        text-decoration: none;
    }
    .rank::before {
        position: absolute;
        inset-block: 0;
        inset-inline-start: 0;
        width: var(--share);
        border-radius: inherit;
        background: var(--ink);
        opacity: 0.15;
        transition: width 300ms ease;
        content: "";
    }
    .rank::after {
        order: -1;
        flex: none;
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--ink);
        content: "";
    }
    a.rank:hover,
    a.rank:focus-visible {
        outline: 1px solid var(--mantine-color-cyan-outline);
        outline-offset: -1px;
    }
    .name,
    strong,
    .flag {
        position: relative;
    }
    .name {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }
    strong {
        font-weight: inherit;
    }
    .flag {
        flex: none;
        font-size: 1em;
        line-height: 1;
    }
    .empty {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        min-height: 224px;
        gap: 8px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .empty-symbol {
        display: grid;
        color: var(--mantine-color-text);
        opacity: 0.5;
    }
</style>
