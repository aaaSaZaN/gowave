<script lang="ts">
    import { onMount, type Snippet } from 'svelte'

    let {
        anchor,
        role = 'menu',
        label,
        id,
        wide = false,
        alignEnd = false,
        arrow = false,
        width: menuWidth,
        maxHeight,
        matchWidth = false,
        multiple = false,
        keyboardNavigation = true,
        focusOnOpen = false,
        preferAbove = false,
        class: className,
        onpointerenter,
        onpointerleave,
        onselect,
        close,
        children
    }: {
        anchor: HTMLElement
        role?: 'menu' | 'listbox' | 'dialog'
        label: string
        id: string
        wide?: boolean
        alignEnd?: boolean
        arrow?: boolean
        width?: number
        maxHeight?: number
        matchWidth?: boolean
        multiple?: boolean
        keyboardNavigation?: boolean
        focusOnOpen?: boolean
        preferAbove?: boolean
        class?: string
        onpointerenter?: () => void
        onpointerleave?: () => void
        onselect?: () => void
        close: () => void
        children: Snippet
    } = $props()
    let popup: HTMLDivElement

    function position(): void {
        const anchorBox = anchor.getBoundingClientRect()
        const gap = arrow ? 11.5 : 8
        const edge = 5
        const below = anchorBox.bottom + gap
        const belowSpace = window.innerHeight - edge - below
        const aboveSpace = anchorBox.top - gap - edge
        if (matchWidth)
            popup.style.minWidth = `${Math.min(anchorBox.width, window.innerWidth - 2 * edge)}px`
        const heightLimit =
            maxHeight ?? (wide ? Math.max(160, window.innerHeight * 0.8 - 100) : window.innerHeight)
        const content = popup.querySelector('.popup-content') ?? popup
        const desiredHeight = Math.min(content.scrollHeight + 10, 160)
        const flip = preferAbove
            ? aboveSpace >= desiredHeight || aboveSpace > belowSpace
            : belowSpace < desiredHeight && aboveSpace > belowSpace
        popup.style.setProperty(
            '--popup-height',
            `${Math.max(0, Math.min(heightLimit, flip ? aboveSpace : belowSpace))}px`
        )
        const { width, height } = popup.getBoundingClientRect()
        const rtl = getComputedStyle(anchor).direction === 'rtl'
        const start = wide
            ? window.innerWidth - width - edge
            : alignEnd !== rtl
              ? anchorBox.right - width
              : anchorBox.left
        const top = flip ? anchorBox.top - gap - height : below
        const left = Math.max(edge, Math.min(start, window.innerWidth - width - edge))
        popup.style.left = `${left}px`
        popup.style.top = `${Math.max(edge, Math.min(top, window.innerHeight - height - edge))}px`
        if (arrow) {
            const center = alignEnd
                ? anchorBox.right - left - 8
                : anchorBox.left + anchorBox.width / 2 - left
            popup.style.setProperty(
                '--arrow-left',
                `${Math.max(10, Math.min(center, width - 10))}px`
            )
            popup.toggleAttribute('data-flipped', flip)
        }
    }

    onMount(() => {
        function select(event: MouseEvent): void {
            if (event.target instanceof Element && event.target.closest('[role=menuitem]'))
                onselect?.()
        }
        popup.addEventListener('click', select)
        popup.showPopover()
        position()
        if (focusOnOpen) {
            const target = popup.querySelector<HTMLElement>('[data-popup-autofocus]') ?? popup
            target.focus({ preventScroll: true })
        }
        const observer = new ResizeObserver(position)
        observer.observe(popup)
        observer.observe(anchor)
        window.addEventListener('scroll', position, true)
        return () => {
            popup.removeEventListener('click', select)
            observer.disconnect()
            window.removeEventListener('scroll', position, true)
        }
    })

    function keys(event: KeyboardEvent): void {
        if (!popup?.matches(':popover-open') || !(event.target instanceof Node)) return
        if (event.target !== anchor && !popup.contains(event.target)) return
        if (
            !keyboardNavigation ||
            role === 'dialog' ||
            !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)
        )
            return
        const items = [...popup.querySelectorAll('button:not(:disabled)')]
        if (!items.length) return
        const focused = document.activeElement
        const index = focused ? items.indexOf(focused) : -1
        const previous = index < 0 ? items.length - 1 : (index + items.length - 1) % items.length
        const next = (index + 1) % items.length
        const target = { ArrowUp: previous, ArrowDown: next, Home: 0, End: items.length - 1 }[
            event.key
        ]
        if (target === undefined) return
        event.preventDefault()
        const item = items[target]
        if (item instanceof HTMLButtonElement) item.focus()
    }
</script>

<svelte:window onresize={position} onkeydown={keys} />

<div
    {id}
    {...{
        role,
        'aria-label': label,
        'aria-multiselectable': role === 'listbox' && multiple ? true : undefined
    }}
    tabindex="-1"
    class={[className, 'table-popup gw-popover']}
    class:with-arrow={arrow}
    style:width={menuWidth ? `${menuWidth}px` : undefined}
    popover="auto"
    bind:this={popup}
    {onpointerenter}
    {onpointerleave}
    ontoggle={(event) => {
        if (event.newState === 'closed') close()
    }}
>
    {#if arrow}
        <span class="popup-arrow gw-popover-arrow" aria-hidden="true"></span>
        <div class="popup-content">{@render children()}</div>
    {:else}
        {@render children()}
    {/if}
</div>

<style>
    .table-popup {
        position: fixed;
        outline: none;
        inset: 0 auto auto 0;
        width: max-content;
        max-width: calc(100vw - 10px);
        max-height: var(--popup-height, calc(100dvh - 10px));
        overflow: auto;
        margin: 0;
        padding: 4px;
        border-color: var(--popover-border-color);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-text);
        box-shadow:
            0 8px 32px rgba(0, 0, 0, 0.16),
            0 2px 8px rgba(0, 0, 0, 0.1);
    }

    .with-arrow {
        overflow: visible;
    }

    .popup-content {
        position: relative;
        z-index: 2;
        max-height: max(0px, calc(var(--popup-height, 100dvh) - 10px));
        overflow: auto;
        background: inherit;
    }

    .popup-arrow {
        position: absolute;
        top: -4px;
        left: var(--arrow-left);
        width: 7px;
        height: 7px;
        transform: translateX(-50%) rotate(45deg);
    }

    [data-flipped] .popup-arrow {
        top: auto;
        bottom: -4px;
    }
</style>
