<script lang="ts">
    import { locale } from '../i18n.svelte'
    import Button from './Button.svelte'
    import type { ConfirmOptions } from './confirmation'
    import Overlay from './Overlay.svelte'
    import ThemeIcon from './ThemeIcon.svelte'

    let {
        message,
        title = locale.t('common.action.confirm-action'),
        label = title,
        tone = 'red',
        close
    }: ConfirmOptions & { message: string; close: (confirmed: boolean) => void } = $props()

    let overlay: ReturnType<typeof Overlay>
    let confirmed = false
</script>

<Overlay
    bind:this={overlay}
    {title}
    {tone}
    icon="TbAlertTriangle"
    width={440}
    hideHeader
    fullscreenOnMobile={false}
    largePadding
    close={() => close(confirmed)}
>
    <div class="confirmation">
        <ThemeIcon icon="TbAlertTriangle" {tone} size={44} iconSize={24} glow />
        <h2>{title}</h2>
        <p>{message}</p>
        <footer>
            <Button data-autofocus onclick={() => overlay.dismiss()}>
                {locale.t('common.action.cancel')}
            </Button>
            <Button
                variant="soft"
                color={tone}
                onclick={() => {
                    confirmed = true
                    overlay.dismiss()
                }}
                >{label}</Button
            >
        </footer>
    </div>
</Overlay>

<style>
    .confirmation {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-md);
        text-align: center;
    }

    h2,
    p {
        margin: 0;
    }

    h2 {
        font-size: var(--mantine-font-size-lg);
    }

    p {
        color: var(--mantine-color-dimmed);
        white-space: pre-line;
        overflow-wrap: anywhere;
    }

    footer {
        display: flex;
        justify-content: center;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-sm);
        width: 100%;
        padding-block-start: var(--mantine-spacing-md);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.08);
    }
</style>
