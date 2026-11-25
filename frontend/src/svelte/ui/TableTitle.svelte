<script lang="ts">
    import type { Snippet } from 'svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        title,
        icon,
        tone = 'cyan',
        children
    }: { title: string; icon: IconName; tone?: string; children: Snippet } = $props()
</script>

<header class="table-title">
    <div class="table-heading">
        <span
            class="heading-icon"
            style:--icon-color={`var(--mantine-color-${tone}-4)`}
            style:--icon-tint={`var(--mantine-color-${tone}-6)`}
            style:--icon-shade={`var(--mantine-color-${tone}-7)`}
        >
            <Icon name={icon} size={24} />
        </span>
        <h1>{title}</h1>
    </div>
    <div class="heading-actions">{@render children()}</div>
</header>

<style>
    .table-title,
    .table-heading,
    .heading-actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
    }

    .table-title {
        justify-content: space-between;
        padding: var(--mantine-spacing-md);
    }

    .table-heading h1 {
        margin: 0;
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-h4-line-height);
    }

    .heading-icon {
        display: grid;
        width: 42px;
        height: 42px;
        flex: none;
        place-items: center;
        border: 1px solid color-mix(in srgb, var(--icon-tint) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--icon-tint) 15%, transparent),
            color-mix(in srgb, var(--icon-shade) 10%, transparent)
        );
        color: var(--icon-color);
    }

    @media (max-width: 40em) {
        .table-title {
            flex-wrap: wrap;
        }

        .heading-actions {
            margin-inline-start: auto;
        }
    }
</style>
