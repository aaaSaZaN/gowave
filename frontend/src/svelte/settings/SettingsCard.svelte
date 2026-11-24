<script lang="ts">
    import type { Snippet } from 'svelte'
    import BrandIcon from '../ui/BrandIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'

    let {
        title,
        description,
        icon,
        tone = 'cyan',
        children,
        footer
    }: {
        title: string
        description: string
        icon: IconName | 'Logo'
        tone?: string
        children: Snippet
        footer?: Snippet
    } = $props()
    const headingId = $props.id()
</script>

<section class="settings-card" aria-labelledby={headingId}>
    <header>
        <span
            class="theme-icon"
            style:--icon-ink={`var(--mantine-color-${tone}-4)`}
            style:--icon-tint={`var(--mantine-color-${tone}-6)`}
            style:--icon-shade={`var(--mantine-color-${tone}-7)`}
        >
            {#if icon === 'Logo'}
                <BrandIcon />
            {:else}
                <Icon name={icon} size={24} />
            {/if}
        </span>
        <div>
            <h2 id={headingId}>{title}</h2>
            <p>{description}</p>
        </div>
    </header>
    <div class="contents">{@render children()}</div>
    {#if footer}
        <div class="bottom">{@render footer()}</div>
    {/if}
</section>

<style>
    .settings-card {
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(
            135deg,
            var(--mantine-color-dark-6),
            var(--mantine-color-dark-7)
        );
        box-shadow: var(--mantine-shadow-xl);
    }
    header {
        display: flex;
        align-items: flex-start;
        gap: var(--mantine-spacing-md);
        padding-block-end: var(--mantine-spacing-md);
        margin-block-end: calc(var(--mantine-spacing-md) + var(--mantine-spacing-xs));
        border-block-end: 1px solid
            color-mix(in srgb, var(--mantine-color-default-border) 30%, transparent);
    }
    h2 {
        margin: 0;
        font-size: var(--mantine-h4-font-size);
        line-height: var(--mantine-h4-line-height);
        font-weight: var(--mantine-h4-font-weight);
    }
    p {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        white-space: pre-line;
    }
    .theme-icon {
        flex: none;
        display: grid;
        place-items: center;
        width: 44px;
        height: 44px;
        border: 1px solid color-mix(in srgb, var(--icon-tint) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--icon-tint) 15%, transparent),
            color-mix(in srgb, var(--icon-shade) 10%, transparent)
        );
        color: var(--icon-ink);
    }
    .bottom {
        padding-block-start: var(--mantine-spacing-xs);
        margin-block-start: calc(var(--mantine-spacing-lg) + var(--mantine-spacing-xs));
        border-block-start: 1px solid
            color-mix(in srgb, var(--mantine-color-default-border) 30%, transparent);
    }
</style>
