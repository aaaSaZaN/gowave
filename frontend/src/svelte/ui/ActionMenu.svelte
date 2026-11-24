<script lang="ts">
    import { onDestroy, type Snippet } from 'svelte'
    import ActionIcon from './ActionIcon.svelte'
    import Button from './Button.svelte'
    import type { IconName } from './icons'
    import TablePopup from './TablePopup.svelte'

    let {
        label,
        icon = 'TbDots',
        size = 30,
        iconSize = 18,
        text = false,
        hover = true,
        subtle = true,
        alignEnd = true,
        preferAbove = false,
        width = 200,
        children
    }: {
        label: string
        icon?: IconName
        size?: number
        iconSize?: number
        text?: boolean
        subtle?: boolean
        hover?: boolean
        alignEnd?: boolean
        preferAbove?: boolean
        width?: number
        children: Snippet
    } = $props()
    const id = $props.id()
    let trigger: HTMLSpanElement
    let anchor = $state<HTMLElement>()
    let open = $state(false)
    let clickOpened = $state(false)
    let timer: ReturnType<typeof setTimeout> | undefined

    function show(focus: boolean): void {
        clearTimeout(timer)
        anchor = trigger.querySelector('button') ?? undefined
        clickOpened = focus
        open = true
    }

    function close(): void {
        clearTimeout(timer)
        open = false
    }

    function toggle(): void {
        if (hover && open) {
            clearTimeout(timer)
            clickOpened = true
            document.getElementById(id)?.focus({ preventScroll: true })
        } else if (open) {
            close()
        } else {
            show(true)
        }
    }

    function enter(event: PointerEvent): void {
        clearTimeout(timer)
        if (hover && !open && event.pointerType === 'mouse')
            timer = setTimeout(() => show(false), 150)
    }

    function leave(): void {
        clearTimeout(timer)
        if (hover) timer = setTimeout(close, 100)
    }

    function leaveAnchor(): void {
        if (clickOpened) clearTimeout(timer)
        else leave()
    }

    onDestroy(() => clearTimeout(timer))
</script>

<span class="menu-trigger" bind:this={trigger}>
    {#if text}
        <Button
            {icon}
            block
            iconSize={19.2}
            color="gray"
            variant="light"
            size="md"
            aria-haspopup="menu"
            aria-expanded={open}
            aria-controls={open ? id : undefined}
            onclick={toggle}
            onpointerenter={enter}
            onpointerleave={leaveAnchor}
            >{label}</Button
        >
    {:else}
        <ActionIcon
            {icon}
            {label}
            {size}
            {iconSize}
            {...open ? { tone: 'cyan' } : {}}
            {subtle}
            aria-haspopup="menu"
            aria-expanded={open}
            aria-controls={open ? id : undefined}
            onclick={toggle}
            onpointerenter={enter}
            onpointerleave={leaveAnchor}
        />
    {/if}
</span>
{#if open && anchor}
    <TablePopup
        {anchor}
        {id}
        {label}
        {width}
        {alignEnd}
        {preferAbove}
        class="action-menu"
        focusOnOpen={clickOpened}
        {close}
        onselect={close}
        onpointerenter={() => clearTimeout(timer)}
        onpointerleave={leave}
    >
        {@render children()}
    </TablePopup>
{/if}

<style>
    .menu-trigger {
        display: inline-flex;
        flex: none;
    }
    :global(.action-menu [role="menuitem"]) {
        display: flex;
        width: 100%;
        min-height: 32px;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: 6px 12px;
        border: 0;
        border-radius: var(--mantine-radius-sm);
        color: var(--mantine-color-text);
        background: transparent;
        font-size: var(--mantine-font-size-sm);
        font-weight: 400;
        text-align: start;
    }
    :global(.action-menu [role="menuitem"]:is(:hover, :focus-visible):not(:disabled)) {
        background: var(--mantine-color-default-hover);
    }
    :global(.action-menu [role="menuitem"][data-tone="teal"]) {
        color: var(--mantine-color-teal-4);
    }
    :global(.action-menu [role="menuitem"][data-tone="red"]) {
        color: var(--mantine-color-red-4);
    }
</style>
