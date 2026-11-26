<script lang="ts">
    import type { Snippet } from 'svelte'
    import type { IconName } from './icons'
    import ThemeIcon from './ThemeIcon.svelte'

    let {
        title,
        icon,
        tone,
        children,
        description,
        headingSize,
        actions,
        class: className
    }: {
        title: string
        icon: IconName
        tone: string
        children?: Snippet
        description?: string | undefined
        headingSize?: 'lg' | 'xl'
        actions?: Snippet
        class?: string
    } = $props()
</script>

<section class={['form-card', className]}>
    <header
        class:has-content={!!children}
        class:large={headingSize === 'lg'}
        class:extra-large={headingSize === 'xl'}
    >
        <ThemeIcon
            {icon}
            {tone}
            size={headingSize === 'xl' ? 44 : 34}
            iconSize={headingSize === 'xl' ? 24 : 20}
        />
        <div class="heading">
            <h3>{title}</h3>
            {#if description}
                <p>{description}</p>
            {/if}
        </div>
        {@render actions?.()}
    </header>
    {#if children}
        <div class="card-body">{@render children()}</div>
    {/if}
</section>

<style>
    .form-card {
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }

    header {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
    }

    header.has-content {
        padding-block-end: var(--mantine-spacing-md);
        margin-block-end: var(--mantine-spacing-md);
        border-block-end: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }

    h3 {
        margin: 0;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h5-font-size);
        line-height: var(--mantine-h5-line-height);
        font-weight: var(--mantine-h5-font-weight);
    }

    .large h3 {
        font-size: var(--mantine-font-size-lg);
    }

    .extra-large h3 {
        font-size: var(--mantine-font-size-xl);
        line-height: var(--mantine-line-height);
    }

    header.extra-large {
        gap: var(--mantine-spacing-md);
    }

    .heading {
        flex: 1 1 min-content;
        min-width: 0;
    }

    .heading p {
        margin: 0;
        overflow-wrap: break-word;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }

    .card-body {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
</style>
