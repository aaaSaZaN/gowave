<script lang="ts">
    import type { Snippet } from 'svelte'
    import type { HTMLButtonAttributes } from 'svelte/elements'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    interface Props extends HTMLButtonAttributes {
        variant?: 'default' | 'filled' | 'soft' | 'light' | 'subtle'
        color?: string
        size?: 'xs' | 'sm' | 'md' | 'lg' | 'compact-xs' | 'compact-sm'
        block?: boolean
        pending?: boolean
        icon?: IconName
        iconSize?: number
        start?: Snippet
    }

    let {
        children,
        variant = 'default',
        color = 'cyan',
        size = 'sm',
        block = false,
        pending = false,
        disabled,
        icon,
        iconSize = 16,
        start,
        class: className,
        type = 'button',
        style,
        ...attrs
    }: Props = $props()

    const customColor = $derived(color.startsWith('#'))
    const variantName = $derived(variant === 'subtle' ? 'light' : variant)
    const palette = $derived(
        variant === 'default'
            ? '--mantine-color-default'
            : `--mantine-color-${color}-${variantName}`
    )
    const soft = $derived(variant === 'soft')
    const tint = $derived(`var(--mantine-color-${color}-6)`)
    const shade = $derived(`var(--mantine-color-${color}-7)`)
    const ink = $derived(`var(--mantine-color-${color}-4)`)
    const background = $derived.by(() => {
        if (soft) {
            return `linear-gradient(135deg, color-mix(in srgb, ${tint} 15%, transparent), color-mix(in srgb, ${shade} 10%, transparent))`
        }
        if (variant === 'subtle') return 'transparent'
        return customColor ? color : `var(${palette})`
    })
    const foreground = $derived.by(() => {
        if (variant !== 'filled') return `var(${palette}-color)`
        if (/^#[0-9a-f]{6}$/i.test(color)) {
            const linear = (start: number) => {
                const channel = Number.parseInt(color.slice(start, start + 2), 16) / 255
                return channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
            }
            if (0.2126 * linear(1) + 0.7152 * linear(3) + 0.0722 * linear(5) > 0.3)
                return 'var(--mantine-color-black)'
        }
        return 'var(--mantine-color-white)'
    })
</script>

<button
    {...attrs}
    {type}
    disabled={disabled || pending}
    aria-busy={pending || attrs['aria-busy']}
    class={[
        'm_77c9d27d mantine-Button-root m_87cf2631 mantine-UnstyledButton-root mantine-active',
        className
    ]}
    data-variant={variant}
    data-size={size}
    data-block={block || undefined}
    data-loading={pending || undefined}
    data-with-left-section={Boolean(icon || start) || undefined}
    {style}
    style:--button-height={`var(--button-height-${size})`}
    style:--button-padding-x={`var(--button-padding-x-${size})`}
    style:--button-fz={`var(--mantine-font-size-${size.replace('compact-', '')})`}
    style:--button-radius="var(--mantine-radius-md)"
    style:--button-bg={background}
    style:--button-hover={soft
        ? `color-mix(in srgb, ${ink} 10%, transparent)`
        : customColor
          ? `color-mix(in srgb, ${color} 85%, black)`
          : `var(${palette}-hover)`}
    style:--button-color={soft ? ink : foreground}
    style:--button-bd={soft
        ? `1px solid color-mix(in srgb, ${tint} 30%, transparent)`
        : variant === 'default'
          ? '1px solid var(--mantine-color-default-border)'
          : '1px solid transparent'}
>
    <span class="m_80f1301b mantine-Button-inner">
        {#if icon || start}
            <span class="m_a74036a mantine-Button-section" data-position="left">
                {#if start}
                    {@render start()}
                {:else if icon}
                    <Icon name={icon} size={iconSize} />
                {/if}
            </span>
        {/if}
        <span class="m_811560b9 mantine-Button-label">{@render children?.()}</span>
    </span>
    {#if pending}
        <span class="pending" aria-hidden="true">
            <span class="loading-spinner" style:--spinner-size={`${iconSize}px`}></span>
        </span>
    {/if}
</button>

<style>
    .pending {
        position: absolute;
        inset: 0;
        display: grid;
        place-items: center;
    }

    @media (prefers-reduced-motion: no-preference) {
        button {
            transition:
                background-color 120ms ease,
                border-color 120ms ease,
                color 120ms ease;
        }
    }
</style>
