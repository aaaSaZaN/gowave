<script lang="ts" generics="Id extends string">
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        id,
        label,
        items,
        selected,
        disabled = false,
        onselect
    }: {
        id: string
        label: string
        items: readonly { id: Id; label: string; icon: IconName }[]
        selected: Id
        disabled?: boolean
        onselect: (id: Id) => void
    } = $props()

    function keydown(event: KeyboardEvent, index: number): void {
        const direction = document.documentElement.dir === 'rtl' ? -1 : 1
        const next = {
            ArrowLeft: (index - direction + items.length) % items.length,
            ArrowRight: (index + direction + items.length) % items.length,
            Home: 0,
            End: items.length - 1
        }[event.key]
        if (next === undefined || disabled) return
        const item = items[next]
        if (!item) return
        event.preventDefault()
        onselect(item.id)
        document.getElementById(`${id}-tab-${item.id}`)?.focus()
    }
</script>

<div class="tabs" role="tablist" aria-label={label}>
    {#each items as item, index (item.id)}
        <button
            type="button"
            class="mantine-UnstyledButton-root"
            role="tab"
            id={`${id}-tab-${item.id}`}
            aria-controls={`${id}-panel-${item.id}`}
            aria-selected={item.id === selected}
            tabindex={item.id === selected ? 0 : -1}
            {disabled}
            onclick={() => onselect(item.id)}
            onkeydown={(event) => keydown(event, index)}
        >
            <Icon name={item.icon} size={19} />
            {item.label}
        </button>
    {/each}
</div>

<style>
    .tabs {
        display: flex;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
        margin-block-end: var(--mantine-spacing-md);
    }

    button {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }

    button:not(:disabled):hover {
        background: var(--mantine-color-default-hover);
    }

    button[aria-selected="true"] {
        color: var(--mantine-primary-color-light-color);
        background: var(--mantine-primary-color-light);
        border-color: color-mix(in srgb, var(--mantine-primary-color-6) 30%, transparent);
    }

    button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }
</style>
