<script lang="ts">
    import { locale } from './i18n.svelte'

    let {
        height = '100dvh',
        compact = false,
        label,
        text
    }: { height?: string; compact?: boolean; label?: string; text?: string } = $props()
</script>

<div
    class="loading-state"
    class:compact
    role="status"
    aria-label={text ?? label ?? (locale.ready ? locale.t('common.message.loading') : 'Loading…')}
    style:--height={height}
>
    <span class="loading-spinner" aria-hidden="true"></span>
    {#if text}
        <span class="message">{text}</span>
    {/if}
</div>

<style>
    .loading-state {
        display: flex;
        min-block-size: max(3rem, calc(var(--height) - var(--header-height, 0px) - 20px));
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-cyan-light-color);
    }

    .compact {
        min-block-size: 3rem;
    }

    .message {
        color: var(--mantine-color-dimmed);
    }
</style>
