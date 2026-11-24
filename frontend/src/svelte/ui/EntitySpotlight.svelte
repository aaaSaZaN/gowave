<script lang="ts">
    import { tick } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'
    import EmptyState from './EmptyState.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    type ResultBadge = { icon: IconName; count: number; tone: string }
    type Result = {
        name: string
        id: string
        description?: string
        detail?: string
        keywords?: string[]
        flag?: string
        icon?: IconName
        tone?: string
        badges?: ResultBadge[]
    } & ({ href: string } | { select: () => void })

    let {
        items,
        icon,
        searchIcon = 'TbSearch'
    }: { items: Result[]; icon: IconName; searchIcon?: IconName } = $props()

    let dialog = $state<HTMLDialogElement>()
    let input = $state<HTMLInputElement>()
    let visible = $state(false)
    let query = $state('')
    let active = $state(-1)
    const id = $props.id()
    const label = $derived(locale.t('common.action.search'))
    const term = $derived(query.trim().toLowerCase())
    const results = $derived(
        items.filter(({ name, description, detail, keywords }) =>
            [name, description ?? '', detail ?? '', ...(keywords ?? [])].some((value) =>
                value.toLowerCase().includes(term)
            )
        )
    )
    const selected = $derived(Math.min(active, results.length - 1))

    async function open(): Promise<void> {
        query = ''
        active = -1
        visible = true
        await tick()
        if (dialog && !dialog.open) dialog.showModal()
        input?.focus()
    }

    function shortcut(event: KeyboardEvent): void {
        if (
            event.defaultPrevented ||
            event.altKey ||
            event.shiftKey ||
            !(event.ctrlKey || event.metaKey) ||
            event.key.toLowerCase() !== 'f' ||
            (!visible && document.querySelector('dialog[open]'))
        )
            return

        event.preventDefault()
        if (!event.repeat) void open()
    }

    async function moveSelection(event: KeyboardEvent): Promise<void> {
        if (!results.length || event.isComposing) return
        if (event.key === 'Enter') {
            event.preventDefault()
            dialog?.querySelector<HTMLElement>('[data-selected=true]')?.click()
            return
        }
        if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
        event.preventDefault()
        const direction = event.key === 'ArrowDown' ? 1 : -1
        active = selected < 0 ? 0 : (selected + direction + results.length) % results.length
        await tick()
        dialog?.querySelector('[data-selected=true]')?.scrollIntoView({ block: 'nearest' })
    }
</script>

<svelte:window onkeydown={shortcut} />
<ActionIcon icon="TbSearch" tone="gray" {label} onclick={() => void open()} />

{#if visible}
    <dialog
        bind:this={dialog}
        onclose={() => (visible = false)}
        aria-label={label}
        closedby="any"
        class="entity-spotlight"
    >
        <div class="search">
            <Icon name={searchIcon} size={searchIcon === 'TbSearch' ? 16 : 20} />
            <input
                bind:this={input}
                bind:value={query}
                type="search"
                role="combobox"
                aria-label={label}
                aria-expanded="true"
                aria-controls={id}
                aria-activedescendant={selected < 0 ? undefined : id + results[selected]?.id}
                placeholder={`${label}...`}
                oninput={() => (active = 0)}
                onkeydown={(event) => void moveSelection(event)}
            >
        </div>
        <div {id} class="results" role="listbox" aria-label={label}>
            {#each results as item, index (item.id)}
                {@const match = item.name.toLowerCase().indexOf(term)}
                {#snippet content()}
                    {#if item.flag}
                        <span class="flag" aria-hidden="true">{item.flag}</span>
                    {:else if item.tone}
                        <span
                            class="icon-tile"
                            style:--tone={`var(--mantine-color-${item.tone}-4)`}
                        >
                            <Icon name={item.icon ?? icon} size={20} />
                        </span>
                    {:else}
                        <Icon
                            name={item.icon ?? icon}
                            size={16}
                            color="var(--mantine-color-dimmed)"
                        />
                    {/if}
                    <div class="result-text">
                        {#if term && match >= 0}
                            <span
                                >{item.name.slice(0, match)}<mark
                                    >{item.name.slice(match, match + term.length)}</mark
                                >{item.name.slice(match + term.length)}</span
                            >
                        {:else}
                            <span>{item.name}</span>
                        {/if}
                        {#if item.description}
                            <small>{item.description}</small>
                        {/if}
                        {#if item.detail}
                            <small>{item.detail}</small>
                        {/if}
                    </div>
                    {#if item.badges?.length}
                        <div class="result-badges">
                            {#each item.badges as badge (badge.icon)}
                                <span
                                    style:color={`var(--mantine-color-${badge.tone}-light-color)`}
                                    style:background={`var(--mantine-color-${badge.tone}-light)`}
                                >
                                    <Icon name={badge.icon} size={12} />
                                    {badge.count.toLocaleString('en-US')}
                                </span>
                            {/each}
                        </div>
                    {/if}
                {/snippet}
                {#if 'href' in item}
                    <a
                        id={id + item.id}
                        href={item.href}
                        role="option"
                        aria-selected={index === selected}
                        data-selected={index === selected}
                        tabindex="-1"
                        onmousedown={(event) => event.preventDefault()}
                        onclick={() => dialog?.close()}
                        >{@render content()}</a
                    >
                {:else}
                    <button
                        class="mantine-UnstyledButton-root"
                        id={id + item.id}
                        type="button"
                        role="option"
                        aria-selected={index === selected}
                        data-selected={index === selected}
                        tabindex="-1"
                        onmousedown={(event) => event.preventDefault()}
                        onclick={() => {
                            dialog?.close()
                            item.select()
                        }}
                    >
                        {@render content()}
                    </button>
                {/if}
            {/each}
        </div>
        {#if results.length === 0}
            <EmptyState icon="TbSearch" label={locale.t('common.message.nothing-found')} />
        {/if}
    </dialog>
{/if}

<style>
    .result-text {
        display: grid;
        flex: 1;
        min-width: 0;
    }

    .result-text small {
        color: var(--mantine-color-dimmed);
        overflow-wrap: anywhere;
    }

    .flag {
        font-size: 18px;
    }

    .icon-tile {
        display: grid;
        place-items: center;
        flex: none;
        width: 34px;
        height: 34px;
        border: 1px solid color-mix(in srgb, var(--tone) 28%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--tone) 12%, transparent);
        color: var(--tone);
    }

    .result-badges,
    .result-badges > span {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }

    .result-badges > span {
        gap: 5px;
        min-height: 26px;
        padding-inline: 12px;
        border-radius: 100vw;
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
    }

    .entity-spotlight {
        width: min(600px, calc(100% - 32px));
        margin: auto;
        padding: 0;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-body);
        color: var(--mantine-color-text);
        box-shadow: var(--mantine-shadow-xl);
    }
    .entity-spotlight::backdrop {
        background: rgb(0 0 0 / 60%);
    }
    .search,
    a,
    button {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: 7px var(--mantine-spacing-md);
    }
    .search {
        min-height: 50px;
        color: var(--mantine-color-dimmed);
    }
    .search input {
        flex: 1;
        padding: 0;
        min-width: 0;
        border: 0;
        outline: none;
        background: transparent;
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-lg);
    }
    .results {
        margin: 0;
        padding: 0;
        max-height: min(350px, 55dvh);
        overflow: auto;
    }
    .results:not(:empty) {
        border-block-start: 1px solid var(--mantine-color-default-border);
        padding: 4px;
    }
    a,
    button {
        border-radius: var(--mantine-radius-sm);
        text-decoration: none;
        color: inherit;
    }
    button {
        width: 100%;
        text-align: start;
    }
    :is(a, button):hover:not([data-selected="true"]) {
        background: var(--mantine-color-default-hover);
    }
    [data-selected="true"] {
        background: var(--mantine-color-dark-5);
        color: var(--mantine-primary-color-contrast, var(--mantine-color-white));
    }
    [data-selected="true"] small {
        color: inherit;
        opacity: 0.7;
    }
    mark {
        color: inherit;
        background: var(--mantine-color-yellow-light);
    }
</style>
