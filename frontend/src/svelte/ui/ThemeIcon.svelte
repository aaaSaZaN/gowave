<script lang="ts">
    import BrandIcon from './BrandIcon.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        icon,
        tone = 'cyan',
        size = 34,
        iconSize = 20,
        radius = 'md',
        rounded = false,
        glow = false,
        class: className
    }: {
        icon: IconName | 'Logo'
        tone?: string
        size?: 28 | 34 | 42 | 44
        iconSize?: number
        radius?: 'md' | 'lg'
        rounded?: boolean
        glow?: boolean
        class?: string
    } = $props()
</script>

<span
    class={['theme-icon', className]}
    class:rounded
    class:glow
    aria-hidden="true"
    style:--theme-icon-size={`${size}px`}
    style:--theme-icon-radius={`var(--mantine-radius-${radius})`}
    style:--theme-icon-light={`var(--mantine-color-${tone}-7)`}
    style:--theme-icon-dark={`var(--mantine-color-${tone}-4)`}
    style:--theme-icon-tint={`var(--mantine-color-${tone}-6)`}
>
    {#if icon === 'Logo'}
        <BrandIcon size={iconSize} />
    {:else}
        <Icon name={icon} size={iconSize} />
    {/if}
</span>

<style>
    .theme-icon {
        display: inline-grid;
        flex: none;
        place-items: center;
        width: var(--theme-icon-size);
        height: var(--theme-icon-size);
        border: 1px solid color-mix(in srgb, var(--theme-icon-tint) 30%, transparent);
        border-radius: var(--theme-icon-radius);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--theme-icon-tint) 15%, transparent),
            color-mix(in srgb, var(--theme-icon-light) 10%, transparent)
        );
        color: light-dark(var(--theme-icon-light), var(--theme-icon-dark));
    }

    .rounded {
        border-radius: 50%;
    }

    .glow {
        box-shadow: 0 0 24px color-mix(in srgb, var(--theme-icon-tint) 25%, transparent);
    }
</style>
