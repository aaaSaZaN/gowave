<script lang="ts">
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'

    let {
        title,
        description,
        summary,
        label,
        icon,
        tone = 'cyan',
        close,
        confirm
    }: {
        title: string
        description: string
        summary?: string
        label: string
        icon: IconName
        tone?: string
        close: () => void
        confirm: () => void
    } = $props()
</script>

<Overlay {title} {icon} {tone} width={480} fullscreenOnMobile={false} {close}>
    <p>{description}</p>
    {#if summary}
        <pre>{summary}</pre>
    {/if}
    <div class="actions">
        <Button data-autofocus onclick={dismissContainingDialog}
            >{locale.t('common.action.cancel')}</Button
        >
        <Button color={tone} variant="soft" {icon} onclick={confirm}>{label}</Button>
    </div>
</Overlay>

<style>
    p {
        margin: 0;
    }
    pre {
        white-space: pre-wrap;
        overflow-wrap: anywhere;
    }
    .actions {
        display: flex;
        justify-content: flex-end;
        gap: var(--mantine-spacing-xs);
        margin-block-start: var(--mantine-spacing-md);
    }
</style>
