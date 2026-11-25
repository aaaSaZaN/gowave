<script lang="ts">
    import { DotLottie } from '@lottiefiles/dotlottie-web'
    import wasmUrl from '@lottiefiles/dotlottie-web/dotlottie-player.wasm?url'
    import { panelUrl } from '@shared/lib/panel-path'
    import { onMount, type Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import Icon from '../ui/Icon.svelte'
    import type { ScanProgress } from './scan'

    const animations = ['duck-progress', 'globe', 'jail']

    let {
        completed = 0,
        total = 0,
        percent = 0,
        explorer,
        children
    }: {
        completed?: number
        total?: number
        percent?: number
        explorer?: ScanProgress
        children?: Snippet
    } = $props()
    let t = $derived(locale.t)
    let canvas: HTMLCanvasElement
    let ready = $state(false)
    let progress = $derived(
        explorer?.total
            ? ((explorer.completed + explorer.failed) / explorer.total) * 100
            : Math.min(100, Math.max(0, percent))
    )
    let title = $derived(
        explorer
            ? t('sessions-explorer-progress.exploring-nodes')
            : t('active-sessions-drawer.widget.fetching')
    )

    onMount(() => {
        const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
        const animationName = explorer
            ? animations[Math.floor(Math.random() * animations.length)]
            : 'globe'
        DotLottie.setWasmUrl(wasmUrl)
        const animation = new DotLottie({
            canvas,
            autoplay: !reducedMotion,
            loop: !reducedMotion,
            src: panelUrl(`lotties/${animationName ?? 'globe'}.lottie`),
            renderConfig: { autoResize: true, devicePixelRatio: window.devicePixelRatio || 2 }
        })
        animation.addEventListener('load', () => (ready = true))
        return () => animation.destroy()
    })
</script>

<section class="scan" class:explorer={explorer !== undefined} aria-label={title}>
    <div class="animation" aria-hidden="true">
        <canvas bind:this={canvas} class:ready></canvas>
        {#if !ready}
            <Icon name="TbWorld" size={80} />
        {/if}
    </div>
    <div class="description" role="status">
        <strong>{title}</strong>
        {#if explorer}
            <span>
                {t('sessions-explorer-progress.nodes-processed', {
                    completed: explorer.completed + explorer.failed,
                    total: explorer.total
                })}
                {#if explorer.failed}
                    <b class="failed">
                        ({t('sessions-explorer-progress.nodes-failed', { count: explorer.failed })})
                    </b>
                {/if}
            </span>
        {:else}
            <span>{t('active-sessions-drawer.widget.progress', { completed, total })}</span>
        {/if}
    </div>
    <div class="progress">
        <div class="track" role="progressbar" aria-label={title} aria-valuenow={progress}>
            <span style:width={`${progress}%`}></span>
        </div>
        {#if !explorer}
            <b>{Math.round(progress)}%</b>
        {/if}
    </div>
    {#if explorer?.active.length}
        <ul class="active-nodes">
            {#each explorer.active as node (node.uuid)}
                <li>
                    <span aria-hidden="true">{flag(node.countryCode)}</span>
                    {node.name}
                </li>
            {/each}
        </ul>
    {/if}
    {@render children?.()}
</section>

<style>
    .scan {
        display: grid;
        gap: var(--mantine-spacing-md);
        padding: calc(var(--mantine-spacing-xl) + var(--mantine-spacing-md))
            var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        text-align: center;
    }
    .scan.explorer {
        justify-items: center;
        gap: var(--mantine-spacing-lg);
        padding: clamp(2rem, 5vw, 4rem) clamp(1rem, 4vw, 3rem);
    }
    .active-nodes {
        display: flex;
        flex-wrap: wrap;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        margin: 0;
        padding: 0;
        list-style: none;
    }
    .active-nodes li {
        display: flex;
        align-items: center;
        gap: 0.35rem;
        padding: 0.2rem 0.65rem;
        border: 1px solid color-mix(in srgb, var(--mantine-color-teal-6) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        text-transform: uppercase;
    }
    .active-nodes li span:empty {
        display: none;
    }
    .description .failed {
        color: var(--mantine-color-red-light-color);
    }
    .animation {
        position: relative;
        display: grid;
        height: 120px;
        place-items: center;
        color: var(--mantine-color-blue-5);
    }
    .animation canvas {
        position: absolute;
        width: 96px;
        height: 115px;
        visibility: hidden;
    }
    .animation canvas.ready {
        visibility: visible;
    }
    .description {
        display: grid;
        gap: 4px;
    }
    .description strong {
        color: var(--mantine-color-bright);
        font-weight: 600;
    }
    .description span {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .progress {
        display: grid;
        gap: 6px;
        margin-block-start: 6px;
        color: var(--mantine-color-bright);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-sm);
    }
    .track {
        height: 18px;
        overflow: hidden;
        border-radius: var(--mantine-radius-md);
        background: rgba(var(--gw-fg-rgb), 0.05);
    }
    .track span {
        display: block;
        height: 100%;
        border-radius: inherit;
        background: repeating-linear-gradient(
                45deg,
                transparent 0 7px,
                rgb(255 255 255 / 0.25) 7px 14px
            )
            var(--mantine-color-teal-8);
    }
    .explorer .progress {
        width: 100%;
    }
    .explorer .track {
        height: 12px;
        border-radius: var(--mantine-radius-xl);
        background: var(--mantine-color-default-border);
    }
    .explorer .description strong {
        font-size: var(--mantine-font-size-lg);
    }
    @media (prefers-reduced-motion: no-preference) {
        .active-nodes li {
            animation: show-node 200ms ease-out;
        }
        .track span {
            animation: stripes 1s linear infinite;
            transition: width 400ms ease;
        }
    }
    @keyframes show-node {
        from {
            opacity: 0;
            transform: scale(0.9);
        }
    }
    @keyframes stripes {
        to {
            background-position: 20px 0;
        }
    }
</style>
