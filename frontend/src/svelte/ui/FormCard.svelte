<script lang="ts">
    import type { Snippet } from 'svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        title,
        icon,
        tone,
        children,
        description,
        actions,
        class: className
    }: {
        title: string
        icon: IconName
        tone: string
        children: Snippet
        description?: string | undefined
        actions?: Snippet
        class?: string
    } = $props()
</script>

<section class={['form-card', className]}>
    <header>
        <span
            style:--card-color={`var(--mantine-color-${tone}-4)`}
            style:--card-tint={`var(--mantine-color-${tone}-6)`}
            style:--card-shade={`var(--mantine-color-${tone}-7)`}
            ><Icon name={icon} size={20} /></span
        >
        <div class="heading">
            <h3>{title}</h3>
            {#if description}
                <p>{description}</p>
            {/if}
        </div>
        {@render actions?.()}
    </header>
    <div class="card-body">{@render children()}</div>
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
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding-block-end: var(--mantine-spacing-md);
        margin-block-end: var(--mantine-spacing-md);
        border-block-end: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }

    header span {
        display: grid;
        flex: none;
        width: 34px;
        height: 34px;
        place-items: center;
        border: 1px solid color-mix(in srgb, var(--card-tint) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--card-tint) 15%, transparent),
            color-mix(in srgb, var(--card-shade) 10%, transparent)
        );
        color: var(--card-color);
    }

    h3 {
        margin: 0;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h5-font-size);
        line-height: var(--mantine-h5-line-height);
        font-weight: var(--mantine-h5-font-weight);
    }

    .heading {
        flex: 1;
        min-width: 0;
    }

    .heading p {
        margin: 0;
        overflow-wrap: anywhere;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }

    .card-body {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
</style>
