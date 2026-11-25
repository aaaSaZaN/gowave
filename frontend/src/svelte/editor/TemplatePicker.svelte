<script lang="ts">
    import ColorHash from '@shared/lib/color'
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { z } from 'zod'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'

    let {
        catalog,
        type,
        onload,
        onclose
    }: {
        catalog: string
        type: string
        onload: (text: string) => Promise<void>
        onclose: () => void
    } = $props()

    const catalogSchema = z.object({
        templates: z.array(
            z.object({
                author: z.string(),
                name: z.string(),
                type: z.string(),
                url: z.url({ protocol: /^https$/ })
            })
        )
    })

    const templates = createQuery(() => ({
        queryKey: ['editor-template-catalog', catalog],
        queryFn: async ({ signal }) => {
            const response = await fetch(catalog, { signal })
            if (!response.ok)
                throw new Error(locale.t('use-download-template.failed-to-load-template'))
            return catalogSchema.parse(await response.json()).templates
        }
    }))

    const colors = new ColorHash()

    let search = $state('')
    let selected = $state('')
    let busy = $state(false)
    let error = $state('')
    let request: AbortController | undefined
    let t = $derived(locale.t)
    let available = $derived(templates.data?.filter((template) => template.type === type) ?? [])

    let groups = $derived.by(() => {
        const term = search.trim().toLowerCase()
        const authors = new Map<string, typeof available>()
        for (const template of available) {
            if (
                ![template.name, template.author, template.type].some((value) =>
                    value.toLowerCase().includes(term)
                )
            )
                continue
            const entries = authors.get(template.author) ?? []
            entries.push(template)
            authors.set(template.author, entries)
        }
        return authors
    })

    onDestroy(() => request?.abort())

    function load(event: SubmitEvent): void {
        event.preventDefault()
        if (busy || !available.some((template) => template.url === selected)) return

        busy = true
        error = ''
        const controller = new AbortController()
        request = controller
        void fetch(selected, { signal: controller.signal })
            .then(async (response) => {
                if (!response.ok)
                    throw new Error(t('use-download-template.failed-to-load-template'))
                const text = await response.text()
                if (controller.signal.aborted) return
                await onload(text)
                if (!controller.signal.aborted) onclose()
            })
            .catch(() => {
                if (!controller.signal.aborted) {
                    error = t('use-download-template.failed-to-load-template-please-try-again')
                }
            })
            .finally(() => {
                busy = false
            })
    }
</script>

<Overlay
    title={t('use-download-template.select-template-to-load')}
    icon="TbDownload"
    width={620}
    fullscreenOnMobile={false}
    close={onclose}
>
    <form onsubmit={load}>
        <a
            class="add-template"
            href="https://github.com/remnawave/templates"
            target="_blank"
            rel="noreferrer"
        >
            {t('template-selector.add-template')}
        </a>
        {#if templates.isPending}
            <p role="status">{t('common.message.loading')}</p>
        {:else if templates.isError}
            <p role="alert">
                {t('use-download-template.failed-to-load-template-please-try-again')}
            </p>
            <Button onclick={() => void templates.refetch()}>{t('common.action.try-again')}</Button>
        {:else}
            <label class="search">
                <Icon name="TbSearch" size={16} />
                <input
                    type="search"
                    aria-label={t('template-selector.search-templates')}
                    placeholder={t('template-selector.search-templates')}
                    bind:value={search}
                >
            </label>
            <div class="catalog">
                {#each groups as [author, entries] (author)}
                    <section>
                        <h3>
                            {author === 'remnawave'
                                ? t('template-selector.official-templates')
                                : t('template-selector.author-templates', { author })}
                        </h3>
                        {#each entries as template (template.url)}
                            <button
                                class="template"
                                type="button"
                                aria-pressed={selected === template.url}
                                onclick={() => (selected = template.url)}
                            >
                                <span class="template-description">
                                    <span title={template.name}>{template.name}</span>
                                    <small title={template.author}>by {template.author}</small>
                                </span>
                                <span
                                    class="template-type"
                                    style:--badge-color={colors.hex(template.type)}
                                >
                                    {template.type}
                                </span>
                            </button>
                        {/each}
                    </section>
                {:else}
                    <p>{t('common.message.nothing-found')}</p>
                {/each}
            </div>
        {/if}
        {#if error}
            <p role="alert">{error}</p>
        {/if}
        <div class="actions">
            <Button variant="subtle" onclick={dismissContainingDialog}
                >{t('common.action.cancel')}</Button
            >
            <Button
                type="submit"
                variant="default"
                disabled={busy || !selected || templates.isError}
            >
                {t('template-selector.modal.load-template')}
            </Button>
        </div>
    </form>
</Overlay>

<style>
    form,
    .catalog {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .catalog {
        align-content: start;
        gap: var(--mantine-spacing-lg);
        height: min(400px, 60dvh);
        min-height: 0;
        overflow: auto;
        scrollbar-color: rgb(255 255 255 / 0.4) transparent;
        scrollbar-width: thin;
    }
    .add-template {
        display: grid;
        min-height: 36px;
        place-items: center;
        padding-inline: var(--mantine-spacing-sm);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-cyan-light);
        color: var(--mantine-color-cyan-light-color);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        text-decoration: none;
    }
    .add-template:hover {
        background: var(--mantine-color-cyan-light-hover);
    }
    .search {
        position: relative;
        display: grid;
    }
    .search > :global(svg) {
        position: absolute;
        inset-inline-start: 10px;
        inset-block-start: 10px;
        color: var(--mantine-color-dimmed);
        pointer-events: none;
    }
    .search input {
        height: 36px;
        padding-inline-start: 35px;
        font-size: var(--mantine-font-size-sm);
    }
    .search input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    section {
        display: grid;
        gap: var(--mantine-spacing-xs);
        min-width: 0;
    }
    h3 {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }
    .template,
    .actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .template {
        justify-content: space-between;
        min-width: 0;
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-text);
        text-align: start;
        transition:
            border-color 200ms ease,
            box-shadow 200ms ease;
    }
    .template:hover {
        box-shadow: var(--mantine-shadow-sm);
    }
    .template[aria-pressed="true"] {
        border-color: var(--mantine-color-blue-5);
    }
    .template-description {
        display: grid;
        flex: 1;
        min-width: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .template-description > * {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .template-description small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 400;
    }
    .template-type {
        flex: none;
        padding-inline: 9px;
        border-radius: 999px;
        background: color-mix(in srgb, var(--badge-color) 10%, transparent);
        color: var(--badge-color);
        font-size: 10px;
        font-weight: 700;
        line-height: 18px;
        letter-spacing: 0.25px;
    }
    .actions {
        justify-content: end;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-md);
    }
</style>
