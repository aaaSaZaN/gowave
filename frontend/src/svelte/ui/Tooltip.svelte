<script lang="ts">
    import { type Snippet, tick } from 'svelte'

    let {
        text,
        tip,
        disabled = false,
        multiline = false,
        width,
        placement = 'top',
        openDelay = 0,
        block = false,
        class: className = '',
        children
    }: {
        text?: string | undefined
        tip?: Snippet | undefined
        disabled?: boolean | undefined
        multiline?: boolean | undefined
        width?: number | undefined
        placement?: 'top' | 'bottom' | undefined
        openDelay?: number | undefined
        block?: boolean | undefined
        class?: string | undefined
        children: Snippet
    } = $props()

    const id = $props.id()
    let trigger = $state<HTMLSpanElement>()
    let popup = $state<HTMLDivElement>()
    let open = $state(false)
    let timer: ReturnType<typeof setTimeout> | undefined

    function position(): void {
        if (!trigger || !popup) return
        const anchorBox = trigger.getBoundingClientRect()
        const tipBox = popup.getBoundingClientRect()
        const edge = 8
        const gap = 8

        const spaceAbove = anchorBox.top - gap - edge
        const spaceBelow = window.innerHeight - anchorBox.bottom - gap - edge
        const showBelow =
            placement === 'bottom'
                ? spaceBelow >= tipBox.height || spaceBelow > spaceAbove
                : spaceAbove < tipBox.height && spaceBelow > spaceAbove

        const rawTop = showBelow ? anchorBox.bottom + gap : anchorBox.top - tipBox.height - gap
        const rawLeft = anchorBox.left + anchorBox.width / 2 - tipBox.width / 2
        const top = Math.max(edge, Math.min(rawTop, window.innerHeight - tipBox.height - edge))
        const left = Math.max(edge, Math.min(rawLeft, window.innerWidth - tipBox.width - edge))

        popup.style.top = `${top}px`
        popup.style.left = `${left}px`
    }

    function showNow(): void {
        if (disabled || (!text && !tip)) return
        open = true
        void tick().then(() => {
            if (!open || !popup) return
            if (typeof popup.showPopover === 'function' && !popup.matches(':popover-open')) {
                try {
                    popup.showPopover()
                } catch {
                    // Ignore environments without Popover API support.
                }
            }
            position()
        })
    }

    function scheduleOpen(): void {
        if (disabled || (!text && !tip)) return
        clearTimeout(timer)
        if (openDelay > 0) {
            timer = setTimeout(showNow, openDelay)
        } else {
            showNow()
        }
    }

    function hide(): void {
        clearTimeout(timer)
        open = false
        if (popup && typeof popup.hidePopover === 'function' && popup.matches(':popover-open')) {
            try {
                popup.hidePopover()
            } catch {
                // Ignore if already closed.
            }
        }
    }

    $effect(() => {
        const element = trigger
        if (!element) return
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') hide()
        }
        element.addEventListener('mouseenter', scheduleOpen)
        element.addEventListener('mouseleave', hide)
        element.addEventListener('focusin', scheduleOpen)
        element.addEventListener('focusout', hide)
        element.addEventListener('keydown', onKeyDown)
        return () => {
            element.removeEventListener('mouseenter', scheduleOpen)
            element.removeEventListener('mouseleave', hide)
            element.removeEventListener('focusin', scheduleOpen)
            element.removeEventListener('focusout', hide)
            element.removeEventListener('keydown', onKeyDown)
        }
    })

    $effect(() => () => {
        clearTimeout(timer)
        if (popup && typeof popup.hidePopover === 'function' && popup.matches(':popover-open')) {
            try {
                popup.hidePopover()
            } catch {
                // Ignore teardown races.
            }
        }
    })
</script>

<span
    bind:this={trigger}
    class={`tooltip-trigger ${className}`.trim()}
    class:block
    aria-describedby={open ? id : undefined}
>
    {@render children()}
</span>

{#if open && !disabled && (text || tip)}
    <div
        {id}
        bind:this={popup}
        popover="manual"
        role="tooltip"
        class="gw-tooltip tooltip-surface"
        data-fixed="true"
        data-multiline={multiline || width ? 'true' : undefined}
        style:width={width ? `${width}px` : undefined}
        style:max-width={width ? `${width}px` : 'min(280px, calc(100vw - 16px))'}
    >
        {#if tip}
            {@render tip()}
        {:else}
            {text}
        {/if}
    </div>
{/if}

<style>
    .tooltip-trigger {
        display: inline-flex;
        align-items: center;
        min-width: 0;
    }

    .tooltip-trigger.block {
        display: block;
        width: 100%;
    }

    .tooltip-surface {
        position: fixed;
        inset: auto;
        margin: 0;
        padding: 6px 10px;
        border: 1px solid #484f58;
        border-radius: var(--mantine-radius-md);
        background-color: #21262d;
        color: #f0f6fc;
        font-size: var(--mantine-font-size-xs);
        line-height: 1.45;
        box-shadow: var(--mantine-shadow-sm);
        pointer-events: none;
        z-index: 400;
    }
</style>
