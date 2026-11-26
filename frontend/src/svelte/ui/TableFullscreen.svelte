<script lang="ts">
    import ActionIcon from './ActionIcon.svelte'

    let { panel, active = $bindable(false) }: { panel: HTMLElement | undefined; active?: boolean } =
        $props()
    let fallback = $state(false)

    function synchronize(): void {
        active = !!panel && (fallback || document.fullscreenElement === panel)
    }

    function setFallback(next: boolean): void {
        fallback = next
        synchronize()
    }

    async function toggle(): Promise<void> {
        if (!panel) return
        if (fallback) {
            setFallback(false)
        } else if (document.fullscreenElement === panel) {
            await document.exitFullscreen()
        } else {
            try {
                if (!document.fullscreenEnabled) {
                    setFallback(true)
                    return
                }
                await panel.requestFullscreen()
            } catch {
                setFallback(true)
            }
        }
    }

    $effect(() => {
        if (!fallback || !panel) return
        const element = panel
        element.classList.add('table-fullscreen')
        return () => element.classList.remove('table-fullscreen')
    })
</script>

<svelte:document
    onfullscreenchange={synchronize}
    onkeydown={(event) => {
        if (event.key === 'Escape' && fallback && !document.querySelector('dialog:modal')) {
            setFallback(false)
        }
    }}
/>

<ActionIcon
    icon={active ? 'TbMinimize' : 'TbMaximize'}
    label={active ? 'Exit fullscreen' : 'Fullscreen'}
    size={34}
    subtle
    onclick={() => void toggle()}
/>

<style>
    :global(:is(section, div).table-fullscreen[class]) {
        position: fixed;
        inset: 0;
        z-index: var(--mantine-z-index-overlay);
        overflow: auto;
        padding: var(--mantine-spacing-md);
        background: var(--gw-surface);
    }

    :global(html:has(.table-fullscreen)) {
        overflow: hidden;
    }
</style>
