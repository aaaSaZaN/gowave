<script lang="ts">
    let {
        height = '100dvh',
        label = 'Loading…',
        text
    }: { height?: string; label?: string; text?: string } = $props()
</script>
<div class="loader" role="status" aria-label={text ?? label} style:--height={height}>
    {#if text}
        <p>{text}</p>
    {/if}
    <span></span>
</div>
<style>
    .loader {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);

        min-height: max(3rem, calc(var(--height) - var(--header-height, 0px) - 20px));
    }
    p {
        margin: 0;
        font-size: var(--mantine-font-size-lg);
    }
    span {
        width: min(80%, 32rem);
        height: 8px;
        border-radius: var(--mantine-radius-xs);
        background-color: var(--mantine-color-cyan-filled);
        background-image: repeating-linear-gradient(
            45deg,
            transparent 0 5px,
            rgb(255 255 255 / 15%) 5px 10px
        );
        animation: stripes 1s linear infinite;
    }
    @keyframes stripes {
        to {
            background-position: 28px 0;
        }
    }
    @media (prefers-reduced-motion: reduce) {
        span {
            animation: none;
        }
    }
</style>
