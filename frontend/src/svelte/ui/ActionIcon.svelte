<script lang="ts">
    import type { HTMLButtonAttributes } from 'svelte/elements'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    interface Props extends HTMLButtonAttributes {
        icon: IconName
        label: string
        href?: string | undefined
        target?: string | undefined
        rel?: string | undefined
        tone?: string
        size?: number
        iconSize?: number
        subtle?: boolean
        pending?: boolean
    }

    let {
        icon,
        label,
        href,
        target,
        rel,
        tone,
        size = 42,
        iconSize = 24,
        subtle = false,
        pending = false,
        disabled,
        type = 'button',
        class: className,
        ...attrs
    }: Props = $props()

    const ink = $derived(
        tone ? `var(--mantine-color-${tone}-4)` : 'var(--mantine-color-gray-light-color)'
    )
    const edge = $derived(
        tone ? `var(--mantine-color-${tone}-6)` : 'var(--mantine-color-default-border)'
    )
    const shade = $derived(tone ? `var(--mantine-color-${tone}-7)` : 'var(--mantine-color-dark-6)')
</script>

<svelte:element
    this={href ? 'a' : 'button'}
    {...attrs}
    type={href ? undefined : type}
    disabled={href ? undefined : disabled || pending}
    aria-busy={pending || attrs['aria-busy']}
    {href}
    {target}
    {rel}
    class={['mantine-UnstyledButton-root', 'action-icon', className]}
    class:subtle
    class:tinted={tone !== undefined}
    aria-label={label}
    title={label}
    style:--action-size={`${size}px`}
    style:--action-ink={ink}
    style:--action-edge={edge}
    style:--action-shade={shade}
>
    {#if pending}
        <span
            class="loading-spinner"
            aria-hidden="true"
            style:--spinner-size={`${iconSize}px`}
        ></span>
    {:else}
        <Icon name={icon} size={iconSize} />
    {/if}
</svelte:element>

<style>
    .action-icon {
        display: inline-grid;
        width: var(--action-size);
        height: var(--action-size);
        flex: 0 0 auto;
        place-items: center;
        padding: 0;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
        color: var(--action-ink);
        cursor: pointer;
        vertical-align: middle;
        text-decoration: none;
    }

    .action-icon.tinted {
        border-color: color-mix(in srgb, var(--action-edge) 30%, transparent);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--action-edge) 15%, transparent),
            color-mix(in srgb, var(--action-shade) 10%, transparent)
        );
    }

    .action-icon.subtle {
        border-color: transparent;
        background: transparent;
    }

    .action-icon:not(:disabled):hover {
        background-color: color-mix(in srgb, var(--action-ink) 10%, transparent);
    }

    .action-icon:not(:disabled):active {
        background-color: color-mix(in srgb, var(--action-ink) 20%, transparent);
    }

    @media (prefers-reduced-motion: no-preference) {
        .action-icon {
            transition: scale 100ms ease-out;
        }

        .action-icon:not(:disabled):active {
            scale: 0.94;
        }
    }

    .action-icon:disabled {
        cursor: not-allowed;
    }

    .action-icon:disabled:not([aria-busy="true"]) {
        background: var(--mantine-color-disabled);
        border-color: transparent;
        color: var(--mantine-color-disabled-color);
    }
</style>
