<script lang="ts">
    import { onMount, type Snippet } from 'svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'

    let { children, status, actions }: { children: Snippet; status?: Snippet; actions: Snippet } =
        $props()
    let frame: HTMLDivElement
    let panel: HTMLDivElement
    let footer: HTMLDivElement
    let fullscreen = $state(false)

    function fit(): void {
        if (fullscreen) return
        const top = panel.getBoundingClientRect().top + window.scrollY
        const available = window.innerHeight - top - footer.getBoundingClientRect().height - 32
        panel.style.height = `${Math.max(available, 320)}px`
    }

    onMount(() => {
        fit()
        const observer = new ResizeObserver(fit)
        observer.observe(footer)
        if (frame.previousElementSibling) observer.observe(frame.previousElementSibling)
        window.addEventListener('resize', fit)
        return () => {
            observer.disconnect()
            window.removeEventListener('resize', fit)
        }
    })

    function toggle(): void {
        fullscreen = !fullscreen
        if (!fullscreen) requestAnimationFrame(fit)
    }
</script>

<svelte:window
    onkeydown={(event) => {
        if (event.key === 'Escape' && fullscreen && !document.querySelector('dialog[open]')) {
            event.preventDefault()
            toggle()
        }
    }}
/>

<div class="editor-frame" class:fullscreen bind:this={frame}>
    <div class="editor-paper" bind:this={panel}>
        {#if fullscreen}
            <div class="exit-fullscreen">
                <ActionIcon
                    icon="PiArrowsIn"
                    label="Exit fullscreen"
                    tone="gray"
                    size={36}
                    iconSize={18}
                    onclick={toggle}
                />
            </div>
        {/if}
        {@render children()}
        {@render status?.()}
    </div>
    <div class="editor-footer" bind:this={footer} hidden={fullscreen}>
        <ActionIcon
            icon="PiArrowsOut"
            label="Fullscreen"
            tone="gray"
            size={36}
            iconSize={18}
            onclick={toggle}
        />
        {@render actions()}
    </div>
</div>

<style>
    .editor-frame {
        display: flex;
        flex-direction: column;
        min-width: 0;
    }
    .editor-paper {
        position: relative;
        display: flex;
        flex-direction: column;
        min-height: 320px;
        height: 60vh;
        overflow: hidden;
        resize: vertical;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md) var(--mantine-radius-md) 0 0;
        background: var(--mantine-color-body);
    }
    .editor-footer {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: 10px 12px;
        border: 1px solid var(--mantine-color-default-border);
        border-top: none;
        border-radius: 0 0 var(--mantine-radius-md) var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .editor-footer[hidden] {
        display: none;
    }
    .fullscreen {
        position: fixed;
        inset: 0;
        z-index: 250;
        background: var(--mantine-color-body);
    }
    .fullscreen .editor-paper {
        flex: 1;
        border: 0;
        border-radius: 0;
        resize: none;
    }
    .exit-fullscreen {
        position: absolute;
        z-index: 60;
        inset-block-start: 10px;
        inset-inline-end: 10px;
    }
    :global(html:has(.editor-frame.fullscreen)) {
        overflow: hidden;
    }
</style>
