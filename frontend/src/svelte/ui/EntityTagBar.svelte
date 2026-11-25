<script lang="ts">
    import { NO_TAG } from '@shared/constants/tag-filter'
    import { locale } from '../i18n.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        items,
        active,
        ready = true,
        select
    }: {
        items: readonly { tags?: readonly string[] | null }[]
        active: string
        ready?: boolean
        select: (tag: string) => void
    } = $props()

    const t = $derived(locale.t)
    const counts = $derived.by(() => {
        const map = new Map<string, number>()
        for (const item of items) {
            for (const tag of item.tags?.length ? item.tags : [NO_TAG]) {
                map.set(tag, (map.get(tag) ?? 0) + 1)
            }
        }
        return map
    })
    const tags = $derived(
        [...counts.keys()].filter((tag) => tag !== NO_TAG).sort((a, b) => a.localeCompare(b))
    )
    $effect(() => {
        if (ready && active && !counts.has(active)) select('')
    })
</script>

{#snippet tab(value: string, text: string, count: number, icon: IconName)}
    <button
        type="button"
        class="tag-tab mantine-UnstyledButton-root"
        class:active={active === value}
        aria-pressed={active === value}
        onclick={() => select(value)}
    >
        <Icon name={icon} size={16} />
        <span>{text}</span>
        <span class="count">{count}</span>
    </button>
{/snippet}

{#if tags.length}
    <fieldset class="tag-bar" aria-label={t('common.field.tags')}>
        {@render tab('', t('common.field.all'), items.length, 'TbStack2')}
        {#each tags as tag (tag)}
            {@render tab(tag, tag, counts.get(tag) ?? 0, 'TbTag')}
        {/each}
        {#if counts.has(NO_TAG)}
            {@render tab(NO_TAG, t('common.field.untagged'), counts.get(NO_TAG) ?? 0, 'TbTagOff')}
        {/if}
    </fieldset>
{/if}

<style>
    .tag-bar {
        min-width: 0;
        margin: 0;
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-xs);
        overflow-x: auto;
        border: 1px solid rgb(255 255 255 / 8%);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
    }

    .tag-tab {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        height: 36px;
        padding-inline: 12px;
        flex: 0 0 auto;
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-md);
        background: transparent;
        color: var(--mantine-color-gray-light-color);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        cursor: pointer;
    }

    .tag-tab.active {
        color: var(--mantine-color-cyan-4);
        border-color: rgb(21 170 191 / 30%);
        background: linear-gradient(135deg, rgb(21 170 191 / 15%), rgb(16 152 173 / 10%));
    }

    .tag-tab:hover {
        background-color: var(--mantine-color-default-hover);
    }

    .count {
        min-width: 22px;
        padding-inline: 3px;
        height: 18px;
        border-radius: var(--mantine-radius-sm);
        color: inherit;
        background: var(--mantine-color-gray-light);
        font-size: 11px;
        letter-spacing: 0.25px;
        font-weight: 700;
        text-align: center;
    }

    .active .count {
        background: rgb(21 170 191 / 15%);
        box-shadow: inset 0 0 0 1px rgb(21 170 191 / 30%);
    }
</style>
