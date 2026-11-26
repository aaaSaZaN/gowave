<script lang="ts">
    import type { Snippet } from 'svelte'
    import type { HTMLButtonAttributes } from 'svelte/elements'
    import ActionIcon from './ActionIcon.svelte'
    import type { IconName } from './icons'
    import TableFullscreen from './TableFullscreen.svelte'

    type Control = {
        label: string
        onclick: NonNullable<HTMLButtonAttributes['onclick']>
        controls?: string | undefined
        expanded?: boolean
    }
    let {
        panel,
        density,
        columns,
        fullscreen = $bindable(false),
        actions,
        layered = false
    }: {
        panel: HTMLElement | undefined
        density: Control & { icon: IconName; pressed?: boolean | undefined }
        columns: Control
        fullscreen?: boolean
        actions?: Snippet
        layered?: boolean
    } = $props()
</script>

<div class="table-toolbar" class:layered>
    {@render actions?.()}
    <ActionIcon
        icon={density.icon}
        label={density.label}
        size={34}
        subtle
        aria-pressed={density.pressed}
        onclick={density.onclick}
    />
    <TableFullscreen {panel} bind:active={fullscreen} />
    <ActionIcon
        icon="TbColumns"
        label={columns.label}
        size={34}
        subtle
        aria-haspopup={columns.controls ? 'menu' : undefined}
        aria-expanded={columns.expanded}
        aria-controls={columns.expanded ? columns.controls : undefined}
        onclick={columns.onclick}
    />
</div>

<style>
    .table-toolbar {
        display: flex;
        align-items: flex-start;
        justify-content: flex-end;
        gap: 2px;
        min-height: 56px;
        padding: var(--mantine-spacing-xs);
    }

    .layered {
        pointer-events: none;
    }

    .layered :global(button) {
        position: relative;
        pointer-events: auto;
    }
</style>
