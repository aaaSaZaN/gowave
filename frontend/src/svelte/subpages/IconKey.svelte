<script lang="ts">
    import isSvg from '@shared/lib/is-svg'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import TablePopup from '../ui/TablePopup.svelte'

    let {
        library,
        optional = false,
        label,
        value = $bindable()
    }: {
        library: Record<string, string>
        optional?: boolean
        label?: string
        value: string | undefined
    } = $props()

    const id = $props.id()
    let anchor = $state<HTMLButtonElement>()
    let open = $state(false)
    const caption = $derived(label ?? locale.t('common.field.svg-icon'))
    const previews = $derived(
        Object.entries(library).map(([key, svg]) => ({
            key,
            image: isSvg(svg)
                ? `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
                : undefined
        }))
    )
    const selected = $derived(previews.find((item) => item.key === value))
</script>

<div class="field">
    <span id={`${id}-label`}
        >{caption}
        {#if !optional}
            <b> *</b>
        {/if}</span
    >
    <div class="trigger" class:optional={optional && !!value}>
        <button
            class="choose mantine-UnstyledButton-root"
            class:has-value={!!value}
            type="button"
            bind:this={anchor}
            aria-labelledby={`${id}-label ${id}-value`}
            aria-haspopup="dialog"
            aria-expanded={open}
            onclick={() => (open = !open)}
        >
            <span class="preview">
                {#if selected?.image}
                    <img src={selected.image} width="20" height="20" alt="">
                {:else}
                    <Icon name="TbPhoto" />
                {/if}
            </span>
            <span id={`${id}-value`}
                >{value || locale.t('svg-icon-select.component.select-icon')}</span
            >
        </button>
        {#if optional && value}
            <ActionIcon
                icon="TbX"
                label={locale.t('common.action.clear')}
                subtle
                size={22}
                iconSize={14}
                onclick={() => (value = undefined)}
            />
        {/if}
    </div>
    {#if open && anchor}
        <TablePopup
            {anchor}
            {id}
            role="dialog"
            label={caption}
            width={320}
            maxHeight={216}
            focusOnOpen
            close={() => (open = false)}
        >
            {#if previews.length}
                <div class="grid">
                    {#each previews as item (item.key)}
                        <button
                            type="button"
                            class="tile mantine-UnstyledButton-root"
                            title={item.key}
                            aria-label={item.key}
                            aria-pressed={value === item.key}
                            onclick={() => {
                                value = item.key
                                open = false
                                anchor?.focus({ preventScroll: true })
                            }}
                        >
                            {#if item.image}
                                <img src={item.image} width="24" height="24" alt="">
                            {:else}
                                <Icon name="TbPhoto" />
                            {/if}
                            {#if value === item.key}
                                <span class="check"><Icon name="TbCheck" size={12} /></span>
                            {/if}
                        </button>
                    {/each}
                </div>
            {:else}
                <div class="empty">
                    <Icon name="TbPhoto" size={24} />
                    {locale.t('svg-icon-select.component.no-icons-in-library')}
                </div>
            {/if}
        </TablePopup>
    {/if}
</div>

<style>
    .field {
        display: grid;
        gap: 4px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }
    b {
        color: var(--mantine-color-red-5);
    }
    .trigger,
    .tile {
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.1);
        border-radius: var(--mantine-radius-md);
    }
    .trigger,
    .choose {
        display: flex;
        align-items: center;
    }
    .trigger {
        padding-inline-end: 10px;
    }
    .choose {
        flex: 1;
        min-width: 0;
        gap: var(--mantine-spacing-sm);
        padding: 8px 12px;
        color: var(--mantine-color-text);
        font: inherit;
    }
    .choose.has-value {
        color: var(--mantine-color-bright);
    }
    .trigger.optional .choose {
        flex: 0 1 auto;
    }
    .preview,
    .tile,
    .check {
        display: grid;
        place-items: center;
    }
    .preview {
        width: 32px;
        height: 32px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-cyan-light);
        border: 1px solid rgba(34, 211, 238, 0.2);
    }
    .grid {
        display: grid;
        grid-template-columns: repeat(5, minmax(0, 1fr));
        gap: 6px;
        padding: 10px;
    }
    .tile {
        width: 100%;
        height: 52px;
        position: relative;
        color: var(--mantine-color-text);
    }
    .tile:hover,
    .tile[aria-pressed="true"] {
        background: var(--mantine-color-cyan-light);
        border-color: var(--mantine-color-cyan-6);
    }
    .check {
        position: absolute;
        right: 2px;
        bottom: 2px;
        width: 16px;
        height: 16px;
        border-radius: 50%;
        background: var(--mantine-color-cyan-6);
        color: white;
    }
    .empty {
        display: grid;
        justify-items: center;
        gap: 8px;
        padding: var(--mantine-spacing-lg);
    }
</style>
