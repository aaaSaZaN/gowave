<script lang="ts">
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'
    import BrandIcon from './BrandIcon.svelte'
    import { cancelDialog, dismissDialog } from './dialog-motion'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        title,
        titlePrefix,
        icon,
        tone = 'cyan',
        width = 620,
        drawer = false,
        hideHeader = false,
        fullscreenOnMobile = true,
        class: className,
        largePadding = false,
        subtitle,
        actions,
        inlineActions = false,
        wrapActions = false,
        children,
        preventClose = false,
        close
    }: {
        title: string
        titlePrefix?: string | undefined
        icon: IconName | 'Logo'
        tone?: string
        width?: number
        fullscreenOnMobile?: boolean
        class?: string
        drawer?: boolean
        hideHeader?: boolean
        largePadding?: boolean
        subtitle?: string | undefined
        actions?: Snippet | undefined
        inlineActions?: boolean
        wrapActions?: boolean
        children: Snippet
        preventClose?: boolean
        close: () => void
    } = $props()
    const id = $props.id()
    let dialog: HTMLDialogElement

    let backdropPointer: number | undefined

    function isBackdrop(event: MouseEvent): boolean {
        if (event.target !== dialog) return false
        const { left, right, top, bottom } = dialog.getBoundingClientRect()
        return (
            event.clientX < left ||
            event.clientX >= right ||
            event.clientY < top ||
            event.clientY >= bottom
        )
    }

    function dismiss(): void {
        if (!preventClose) dismissDialog(dialog)
    }

    $effect(() => {
        dialog.showModal()
        const initialFocus = dialog.querySelector<HTMLElement>('[data-autofocus]')
        if (initialFocus) initialFocus.focus({ preventScroll: true })
        else dialog.focus({ preventScroll: true })
    })
</script>

<dialog
    bind:this={dialog}
    tabindex="-1"
    aria-label={hideHeader ? title : undefined}
    aria-labelledby={hideHeader ? undefined : id}
    onclose={close}
    oncancel={(event) => cancelDialog(event, preventClose)}
    onpointerdown={(event) => {
        backdropPointer = event.button === 0 && isBackdrop(event) ? event.pointerId : undefined
    }}
    onpointercancel={() => (backdropPointer = undefined)}
    onpointerup={(event) => {
        if (backdropPointer === event.pointerId && isBackdrop(event)) dismiss()
        backdropPointer = undefined
    }}
    closedby={preventClose ? 'none' : 'closerequest'}
    class={['panel-overlay', className]}
    class:mobile-fullscreen={fullscreenOnMobile}
    class:drawer
    class:modal={!drawer}
    class:large-padding={largePadding}
    style:--drawer-width={`${width}px`}
>
    <div class="drawer-content">
        {#if !hideHeader}
            <header class="overlay-header">
                <div
                    class="overlay-heading"
                    class:with-inline-actions={inlineActions}
                    class:wrap-actions={wrapActions}
                >
                    <span
                        class="drawer-icon"
                        style:--drawer-ink={`var(--mantine-color-${tone}-4)`}
                        style:--drawer-tint={`var(--mantine-color-${tone}-6)`}
                    >
                        {#if icon === 'Logo'}
                            <BrandIcon size={20} />
                        {:else}
                            <Icon name={icon} size={20} />
                        {/if}
                    </span>
                    <div class="overlay-title">
                        {#if titlePrefix}
                            <h2 {id}>
                                <span class="title-prefix" aria-hidden="true">{titlePrefix}</span>
                                {title}
                            </h2>
                        {:else}
                            <h2 {id}>{title}</h2>
                        {/if}
                        {#if subtitle}
                            <div class="subtitle">{subtitle}</div>
                        {/if}
                    </div>
                    {#if inlineActions && actions}
                        <div class="inline-actions">{@render actions()}</div>
                    {/if}
                </div>
                {#if !inlineActions && actions}
                    {@render actions()}
                {/if}
                <ActionIcon
                    icon="TbX"
                    label={locale.t('common.action.close')}
                    subtle
                    size={34}
                    iconSize={24}
                    disabled={preventClose}
                    onclick={dismiss}
                />
            </header>
        {/if}
        <div class="drawer-body">{@render children()}</div>
    </div>
</dialog>

<style>
    .panel-overlay {
        --overlay-padding: var(--mantine-spacing-sm);
        position: fixed;
        padding: 0;
        max-width: none;
        overflow-x: hidden;
        outline: none;
        border: 0;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-body);
        color: var(--mantine-color-text);
        box-shadow: var(--mantine-shadow-xl);
    }
    .large-padding {
        --overlay-padding: var(--mantine-spacing-lg);
    }
    .drawer {
        inset: 0 0 0 auto;
        margin: 0;
        width: min(100%, var(--drawer-width));
        height: 100dvh;
        max-height: none;
        border-start-end-radius: 0;
        border-end-end-radius: 0;
    }
    .modal {
        inset: 0;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        margin: auto;
        width: min(calc(100% - 32px), var(--drawer-width));
        max-height: 90dvh;
    }
    .panel-overlay::backdrop {
        background: rgb(0 0 0 / 60%);
    }
    .overlay-header {
        position: sticky;
        top: 0;
        z-index: 2;
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-lg);
        border-block-end: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        background: rgba(var(--gw-surface-rgb), 0.95);
    }
    .overlay-heading {
        display: flex;
        flex: 1;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        min-width: 0;
    }
    .overlay-title {
        min-width: 0;
    }
    .inline-actions {
        flex: 0 0 auto;
    }
    h2 {
        margin: 0;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h4-font-size);
        font-weight: var(--mantine-h4-font-weight);
        line-height: var(--mantine-h4-line-height);
    }
    .title-prefix {
        margin-inline-end: 8px;
        font-size: 24px;
        font-weight: 400;
        vertical-align: middle;
    }
    .subtitle {
        overflow-wrap: anywhere;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .drawer-icon {
        display: grid;
        place-items: center;
        width: 34px;
        height: 34px;
        flex: 0 0 auto;
        border: 1px solid color-mix(in srgb, var(--drawer-tint) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--drawer-tint) 14%, transparent);
        color: var(--drawer-ink);
    }
    .drawer-body {
        padding: var(--overlay-padding);
    }
    .large-padding.drawer .drawer-body {
        padding: var(--mantine-spacing-sm);
    }
    @media (max-width: 48em) {
        .overlay-heading.with-inline-actions.wrap-actions {
            flex-wrap: wrap;
            row-gap: var(--mantine-spacing-md);
        }
        .wrap-actions .inline-actions {
            flex-basis: 100%;
        }
        .drawer.mobile-fullscreen {
            width: 100vw;
            border-radius: 0;
        }
    }
    @media (max-width: 64rem) {
        .modal.mobile-fullscreen {
            inset: 0;
            width: 100vw;
            height: 100dvh;
            max-height: none;
            border-radius: 0;
        }
    }
</style>
