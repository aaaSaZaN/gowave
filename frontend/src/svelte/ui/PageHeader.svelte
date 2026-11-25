<script lang="ts">
    import { onDestroy, type Snippet } from 'svelte'
    import { fly } from 'svelte/transition'
    import { locale } from '../i18n.svelte'
    import BrandIcon from './BrandIcon.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

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
        <span class="page-icon">
            {#if icon === 'Logo'}
                <BrandIcon />
            {:else}
                <Icon name={icon} size={iconSize} />
            {/if}
        </span>
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
        class="m_a513464 copy-notice"
        style:--notification-color={notice.copied
            ? 'var(--mantine-color-teal-filled)'
            : 'var(--mantine-color-red-filled)'}
        transition:fly={{ x: 24, duration: 200 }}
    >
        <div class="m_a49ed24" role={notice.copied ? 'status' : 'alert'}>
            <div class="m_3feedf16">
                {locale.t(notice.copied ? 'common.message.copied' : 'common.message.error')}
            </div>
            {#if notice.copied}
                <div class="m_3d733a3a" data-with-title>{notice.text}</div>
            {/if}
        </div>
        <button
            type="button"
            class="m_919a4d88 mantine-UnstyledButton-root notice-close"
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

    @media (prefers-reduced-motion: no-preference) {
        .page-header {
            animation: show-card 200ms linear;
        }
        .page-title,
        .page-actions {
            animation: show-heading 500ms cubic-bezier(0, 0.71, 0.2, 1.01);
        }
    }

    @keyframes show-card {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    @keyframes show-heading {
        from {
            opacity: 0;
            transform: translateY(-10px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }

    .page-title {
        flex: 1 1 auto;
        min-width: 0;
    }

    .compact {
        --page-icon-size: 34px;
        align-items: center;
        border-radius: var(--mantine-radius-lg);
        box-shadow: none;
    }

    .page-actions {
        flex: 0 1 auto;
        margin-inline-start: auto;
        flex-wrap: wrap;
        justify-content: end;
        gap: var(--mantine-spacing-md);
    }

    @media (max-width: 48em) {
        .page-actions.wrap-actions {
            flex-basis: 100%;
            justify-content: start;
        }
    }

    .page-icon {
        display: inline-grid;
        width: var(--page-icon-size, 44px);
        height: var(--page-icon-size, 44px);
        flex: 0 0 auto;
        place-items: center;
        color: var(--mantine-color-cyan-4);
        border: 1px solid rgb(21 170 191 / 30%);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(135deg, rgb(21 170 191 / 15%), rgb(16 152 173 / 10%));
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
