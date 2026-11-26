<script lang="ts">
    import ColorHash from '@shared/lib/color'
    import type { Snippet } from 'svelte'
    import ActionMenu from './ActionMenu.svelte'
    import { reorderHandle } from './entity-order'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    interface Badge {
        icon: IconName
        label: string
        value: number
        tone: 'blue' | 'teal' | 'gray'
    }

    let {
        name,
        icon,
        active,
        highlight = active,
        tags,
        badges,
        href,
        open,
        actions,
        label,
        reorder
    }: {
        name: string
        icon: IconName
        active: boolean
        highlight?: boolean
        tags: readonly string[]
        badges: readonly Badge[]
        href?: string
        open?: () => void
        actions: Snippet
        label: string
        reorder?:
            | {
                  key: string
                  drop: (target: string) => void
              }
            | undefined
    } = $props()
    const colorHash = new ColorHash({ lightness: [0.65, 0.65, 0.65] })
</script>

<li class="entity-card" class:active class:highlighted={highlight} data-reorder-key={reorder?.key}>
    <span class="card-rail"></span>
    <span class="card-icon"><Icon name={icon} size={22} /></span>
    <div class="card-content">
        <h2 class="card-title">
            {#if href}
                <a {href}>{name}</a>
            {:else}
                <button class="mantine-UnstyledButton-root" type="button" onclick={open}>
                    {name}
                </button>
            {/if}
        </h2>
        {#if tags.length}
            <span class="card-tags">
                {#each tags as tag (tag)}
                    <span style:color={colorHash.hex(tag)}
                        ><Icon name="TbStar" size={12} />{tag}</span
                    >
                {/each}
            </span>
        {/if}
        <div class="card-badges">
            {#each badges as badge (badge.label)}
                <span
                    class="card-badge"
                    class:gray={badge.tone === 'gray'}
                    style:--badge-color={`var(--mantine-color-${badge.tone}-4)`}
                    style:--badge-base={`var(--mantine-color-${badge.tone}-6)`}
                    title={badge.label}
                >
                    <Icon name={badge.icon} size={12} />{badge.value.toLocaleString('en-US')}
                </span>
            {/each}
        </div>
    </div>
    <span class="card-actions"><ActionMenu {label}> {@render actions()} </ActionMenu></span>
    {#if reorder}
        <button
            type="button"
            class="mantine-UnstyledButton-root grip"
            aria-label={`Reorder ${name}`}
            use:reorderHandle={{ axis: 'grid', drop: reorder.drop }}
        >
            <Icon name="RiDraggable" size={16} />
        </button>
    {/if}
</li>

<style>
    .entity-card {
        position: relative;
        display: flex;
        align-items: center;
        gap: 10px;
        min-width: 0;
        min-height: 46px;
        padding: 6px 10px 6px 12px;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
        cursor: pointer;
    }
    .entity-card:hover {
        background-color: var(--mantine-color-gray-outline-hover);
    }
    .card-rail {
        position: absolute;
        inset: 8px auto 8px 0;
        width: 2px;
        border-radius: 0 2px 2px 0;
        background: var(--mantine-color-dark-4);
    }
    .active .card-rail {
        background: var(--mantine-color-cyan-5);
    }
    .card-icon {
        display: grid;
        place-items: center;
        flex: none;
        width: 32px;
        height: 32px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid rgb(134 142 150 / 30%);
        background: linear-gradient(135deg, rgb(134 142 150 / 15%), rgb(73 80 87 / 10%));
    }
    .highlighted .card-icon {
        border-color: rgb(18 184 134 / 30%);
        background: linear-gradient(135deg, rgb(18 184 134 / 15%), rgb(12 166 120 / 10%));
        color: var(--mantine-color-teal-4);
    }
    .card-content,
    .card-badges,
    .card-tags,
    .card-tags > span {
        display: flex;
        align-items: center;
    }
    .card-content {
        flex: 1;
        min-width: 0;
        gap: 10px;
    }
    .card-tags {
        flex: 0 1 auto;
        max-width: 220px;
        overflow: hidden;
    }
    .card-tags > span {
        gap: 5px;
        padding-inline: 6px;
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
    }
    .card-badges {
        flex: none;
        gap: var(--mantine-spacing-xs);
    }
    .card-badge {
        display: inline-flex;
        align-items: center;
        height: 26px;
        gap: 5px;
        padding-inline: 12px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid color-mix(in srgb, var(--badge-base) 30%, transparent);
        background: color-mix(in srgb, var(--badge-base) 15%, transparent);
        color: var(--badge-color);
        font-weight: 700;
        font-size: var(--mantine-font-size-xs);
    }
    .card-badge.gray {
        color: var(--mantine-color-text);
    }
    .card-title {
        flex: 1;
        min-width: 0;
        margin: 0;
        color: var(--mantine-color-bright);
        font: 700 var(--mantine-font-size-md) / 22px var(--mantine-font-family-monospace);
    }
    .card-title :is(a, button) {
        display: block;
        overflow: hidden;
        max-width: 100%;
        color: inherit;
        font: inherit;
        white-space: nowrap;
        text-decoration: none;
        text-overflow: ellipsis;
    }
    .card-title :is(a, button)::after {
        position: absolute;
        inset: 0;
        content: "";
    }
    .card-actions,
    .grip {
        position: relative;
        z-index: 1;
    }
    .card-actions {
        display: inline-flex;
    }
    .grip {
        flex: none;
        display: grid;
        place-items: center;
        width: 18px;
        height: 30px;
        color: var(--mantine-color-gray-light-color);
    }
    .grip:hover {
        background: var(--mantine-color-gray-light-hover);
    }
    @media (max-width: 48em) {
        .card-content {
            flex-wrap: wrap;
            row-gap: 5px;
        }
        .card-title {
            flex: 1 0 100%;
        }
        .card-tags {
            max-width: 100%;
            order: 3;
        }
    }
</style>
