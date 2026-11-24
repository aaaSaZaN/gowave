<script module lang="ts">
    export type HelpScreen =
        | 'EDITOR_TEMPLATES_XRAY_JSON'
        | 'PAGE_HOSTS'
        | 'PAGE_RESPONSE_HEADERS'
        | 'PAGE_CONFIG_PROFILES'
        | 'PAGE_INTERNAL_SQUADS'
        | 'PAGE_EXTERNAL_SQUADS'
        | `AUTH_METHODS_${'PASSWORD' | 'TELEGRAM' | 'GITHUB' | 'POCKETID' | 'KEYCLOAK' | 'GENERIC'}`
</script>

<script lang="ts">
    import DOMPurify from 'dompurify'
    import { marked } from 'marked'
    import { locale } from '../i18n.svelte'
    import Icon from './Icon.svelte'
    import Overlay from './Overlay.svelte'

    let {
        screen,
        close
    }: {
        screen: HelpScreen
        close: () => void
    } = $props()
    const t = $derived(locale.t)
    const title = $derived(t('help-action-icon.shared.help-article'))
    const languages = new Set(['en', 'fa', 'ru', 'zh'])
    let content = $state('')
    let loading = $state(true)
    let error = $state('')

    $effect(() => {
        const controller = new AbortController()
        const code = locale.language.split('-')[0] ?? 'en'
        const language = languages.has(code) ? code : 'en'
        const url = `https://raw.githubusercontent.com/remnawave/panel/refs/heads/main/_panel-docs/help-articles/${language}/${screen}.md`
        const failure = t('help-drawer.shared.failed-to-load-documentation')
        loading = true
        error = ''
        content = ''

        void (async () => {
            try {
                const response = await fetch(url, { signal: controller.signal })
                if (!response.ok) throw new Error(failure)
                const markdown = await response.text()
                if (!controller.signal.aborted) {
                    content = DOMPurify.sanitize(marked.parse(markdown, { async: false }))
                }
            } catch (reason) {
                if (!controller.signal.aborted) {
                    error = reason instanceof Error ? reason.message : String(reason)
                }
            } finally {
                if (!controller.signal.aborted) loading = false
            }
        })()

        return () => controller.abort()
    })
</script>

<Overlay drawer {title} icon="TbQuestionMark" tone="yellow" {close}>
    {#if loading}
        <p class="status" role="status">{t('common.message.loading')}</p>
    {:else if error}
        <div class="status" role="alert">
            <Icon name="TbAlertCircle" size={64} />
            <h3>{t('help-drawer.shared.failed-to-load-documentation')}</h3>
            <code>{error}</code>
        </div>
    {:else}
        <!-- DOMPurify sanitizes the HTML emitted by marked. -->
        <article class="m_b2a6d4e7 mantine-Typography-root documentation">{@html content}</article>
    {/if}
</Overlay>

<style>
    .status {
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;
        gap: var(--mantine-spacing-xs);
        min-height: 80vh;
    }

    .status :global(svg) {
        color: var(--mantine-color-red-5);
    }
    .status h3 {
        color: var(--mantine-color-dimmed);
        margin: 0;
    }
    .status code {
        color: var(--mantine-color-red-light-color);
        background: var(--mantine-color-red-light);
    }
    .documentation :global(pre code) {
        all: unset;
        font-family: inherit;
    }
</style>
