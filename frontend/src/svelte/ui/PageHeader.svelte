<script lang="ts">
    import { onDestroy, type Snippet } from 'svelte'
    import { fly } from 'svelte/transition'
    import { locale } from '../i18n.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'
    import ThemeIcon from './ThemeIcon.svelte'

    let {
        title,
        icon,
        description,
        iconSize = 24,
        compact = false,
        wrapActions = false,
        children
    }: {
        title: string
        icon: IconName | 'Logo'
        description?: string
        iconSize?: number
        compact?: boolean
        wrapActions?: boolean
        children?: Snippet
    } = $props()

    let notice = $state<{ copied: boolean; text: string }>()
    let timer: ReturnType<typeof setTimeout> | undefined
    let copyRequest = 0

    async function copy(text: string): Promise<void> {
        const request = ++copyRequest
        let copied = false
        try {
            await navigator.clipboard.writeText(text)
            copied = true
        } catch {
            // Clipboard permissions may have changed since the page opened.
        }
        if (request !== copyRequest) return
        notice = { copied, text }
        clearTimeout(timer)
        timer = setTimeout(() => (notice = undefined), 4000)
    }

    onDestroy(() => {
        copyRequest += 1
        clearTimeout(timer)
    })
</script>

<header class="page-header" class:compact>
    <div class="page-title">
        <ThemeIcon {icon} size={compact ? 34 : 44} {iconSize} />
        <div class="title-text" class:has-description={Boolean(description)}>
            <h1>{title}</h1>
            {#if description}
                <button
                    type="button"
                    class="mantine-UnstyledButton-root page-description"
                    title={locale.t('common.action.copy')}
                    onclick={() => description && void copy(description)}
                >
                    {description}
                </button>
            {/if}
        </div>
    </div>
    {#if children}
        <div class="page-actions" class:wrap-actions={wrapActions}>{@render children()}</div>
    {/if}
</header>

{#if notice}
    <div
        class="mantine-Notification-root copy-notice"
        style:--notification-color={notice.copied
            ? 'var(--mantine-color-teal-filled)'
            : 'var(--mantine-color-red-filled)'}
        transition:fly={{ x: 24, duration: 200 }}
    >
        <div class="mantine-Notification-body" role={notice.copied ? 'status' : 'alert'}>
            <div class="mantine-Notification-title">
                {locale.t(notice.copied ? 'common.message.copied' : 'common.message.error')}
            </div>
            {#if notice.copied}
                <div class="mantine-Notification-description" data-with-title>{notice.text}</div>
            {/if}
        </div>
        <button
            type="button"
            class="gw-close-button mantine-UnstyledButton-root notice-close"
            aria-label={locale.t('common.action.close')}
            onclick={() => (notice = undefined)}
        >
            <Icon name="TbX" size={16} />
        </button>
    </div>
{/if}

<style>
    .page-header {
        display: flex;
        align-items: flex-start;
        flex-wrap: wrap;
        justify-content: space-between;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        box-shadow: var(--mantine-shadow-xl);
    }

    .page-title,
    .page-actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
    }

    .page-title {
        flex: 1 1 min-content;
        min-width: 0;
    }

    .compact {
        align-items: center;
        border-radius: var(--mantine-radius-lg);
        box-shadow: none;
    }

    .page-actions {
        max-width: 100%;
        flex: 0 0 auto;
        margin-inline-start: auto;
        flex-wrap: wrap;
        justify-content: end;
        gap: var(--mantine-spacing-md);
    }

    @media (max-width: 48em) {
        .page-actions.wrap-actions {
            flex-basis: 100%;
            justify-content: center;
        }
    }

    .title-text {
        min-width: 0;
    }
    .title-text.has-description {
        display: flex;
        flex-direction: column;
        justify-content: center;
    }
    .page-description {
        align-self: flex-start;
        min-width: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height);
        cursor: copy;
    }
    .copy-notice {
        position: fixed;
        z-index: var(--mantine-z-index-overlay);
        inset: var(--mantine-spacing-md) var(--mantine-spacing-md) auto auto;
        width: min(440px, calc(100% - var(--mantine-spacing-md) * 2));
    }
    .notice-close {
        display: grid;
        flex: none;
        place-items: center;
        width: 34px;
        height: 34px;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-text);
    }
    h1 {
        min-width: 0;
        margin: 0;
        overflow: hidden;
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-h4-line-height);
        text-overflow: ellipsis;
    }
</style>
