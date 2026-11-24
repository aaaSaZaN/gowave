<script lang="ts">
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        label,
        icon,
        height,
        iconSize = 46
    }: { label: string; icon?: IconName; height: number; iconSize?: number } = $props()
</script>
<div class="placeholder" style:min-height={`${height}px`}>
    {#if icon}
        <Icon name={icon} size={iconSize} />
    {/if}
    <span>{label}</span>
</div>
<style>
    .placeholder {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-sm);
        font-size: var(--mantine-font-size-sm);
        color: var(--mantine-color-dimmed);
    }
</style>
