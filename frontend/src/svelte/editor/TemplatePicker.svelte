<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { z } from 'zod'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
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
    icon="TbFolder"
    width={500}
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
            <input
                type="search"
                aria-label={t('template-selector.search-templates')}
                placeholder={t('template-selector.search-templates')}
                bind:value={search}
            >
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
                                <span>{template.name}</span>
                                <small>{template.type}</small>
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
            <Button variant="subtle" onclick={onclose}>{t('common.action.cancel')}</Button>
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
    section,
    .catalog {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    .catalog {
        align-content: start;
        max-height: min(400px, 48dvh);
        overflow: auto;
    }
    .add-template {
        padding: 8px;
        text-align: center;
        text-decoration: none;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-cyan-light);
        color: var(--mantine-color-cyan-light-color);
    }
    h3 {
        margin: 8px 0 0;
        font-size: var(--mantine-font-size-sm);
        color: var(--mantine-color-dimmed);
    }
    .template,
    .actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .template {
        justify-content: space-between;
        text-align: start;
        background: var(--gw-raised-bg);
        border: 1px solid var(--gw-border);
    }
    .template[aria-pressed="true"] {
        border-color: var(--gw-accent);
    }
    small {
        color: var(--mantine-color-cyan-light-color);
        background: var(--mantine-color-cyan-light);
        border-radius: 999px;
        padding: 2px 8px;
    }
    .actions {
        justify-content: end;
        flex-wrap: wrap;
        margin-top: var(--mantine-spacing-sm);
    }
</style>
