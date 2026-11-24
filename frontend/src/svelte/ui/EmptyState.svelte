<script lang="ts">
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        icon,
        iconSize = 32,
        label,
        description
    }: {
        icon: IconName
        iconSize?: number
        label: string
        description?: string | undefined
    } = $props()
</script>

<div class="empty">
    <span class="empty-icon"><Icon name={icon} size={iconSize} /></span>
    <div>
        <h2>{label}</h2>
        {#if description}
            <p>{description}</p>
        {/if}
    </div>
</div>

<style>
    .empty {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--mantine-spacing-lg);
        padding: 64px var(--mantine-spacing-xl);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        text-align: center;
    }

    .empty-icon {
        display: grid;
        width: 64px;
        height: 64px;
        place-items: center;
        border: 1px solid color-mix(in srgb, var(--mantine-color-gray-6) 30%, transparent);
        border-radius: 50%;
        color: var(--mantine-color-gray-4);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--mantine-color-gray-6) 15%, transparent),
            color-mix(in srgb, var(--mantine-color-gray-7) 10%, transparent)
        );
    }

    p {
        margin: var(--mantine-spacing-xs) 0 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        white-space: pre-line;
    }
    h2 {
        margin: 0;
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-line-height-lg);
    }
</style>
