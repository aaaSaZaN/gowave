<script lang="ts">
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { qrImage } from './delivery'

    let {
        value,
        label,
        filename,
        close
    }: {
        value: string
        label: string
        filename: string
        close: () => void
    } = $props()
    const id = $props.id()
    let t = $derived(locale.t)
    let dialog: HTMLDialogElement
    let image: HTMLImageElement | undefined = $state()
    let foreground = $state('#000000')
    let background = $state('#ffffff')
    let pending = $state(false)
    let error = $state('')
    let copied = $state(false)
    let exporter: typeof import('@shared/utils/copy-screenshot.util') | undefined = $state()
    let preview = $derived.by(() => {
        try {
            return { src: qrImage(value, foreground, background), error: '' }
        } catch (failure) {
            return {
                src: undefined,
                error: failure instanceof Error ? failure.message : t('common.message.error')
            }
        }
    })

    onMount(() => {
        dialog.showModal()
        void import('@shared/utils/copy-screenshot.util').then(
            (module) => (exporter = module),
            (failure: unknown) =>
                (error = failure instanceof Error ? failure.message : t('common.message.error'))
        )
    })

    async function exportImage(action: 'copy' | 'download'): Promise<void> {
        if (!image || !exporter || pending) return
        pending = true
        copied = false
        error = ''
        try {
            if (action === 'copy') {
                await exporter.copyImageScreenshotToClipboard(image, filename)
                copied = true
            } else {
                await exporter.downloadImageScreenshot(image, filename)
            }
        } catch (failure) {
            error = failure instanceof Error ? failure.message : t('common.message.error')
        } finally {
            pending = false
        }
    }
</script>

<dialog class="subscription-qr" bind:this={dialog} aria-labelledby={id} onclose={close}>
    <h2 {id}>{label}</h2>
    {#if preview.src}
        <img bind:this={image} src={preview.src} alt={t('view-user-modal.widget.qr-code')}>
    {/if}
    <div class="button-row">
        <label
            >Foreground color <input type="color" name="foreground" bind:value={foreground}></label
        >
        <label
            >Background color <input type="color" name="background" bind:value={background}></label
        >
    </div>
    {#if preview.error || error}
        <p role="alert">{preview.error || error}</p>
    {/if}
    {#if copied}
        <p role="status">{t('common.message.copied')}</p>
    {/if}
    <div class="button-row">
        <button
            type="button"
            disabled={!exporter || !preview.src || pending}
            onclick={() => void exportImage('copy')}
        >
            {t('common.action.copy')}
        </button>
        <button
            type="button"
            disabled={!exporter || !preview.src || pending}
            onclick={() => void exportImage('download')}
        >
            {t('common.action.download')}
        </button>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
</dialog>

<style>
    h2 {
        margin-block-start: 0;
        overflow-wrap: anywhere;
    }
    img {
        display: block;
        width: min(100%, 22rem);
        height: auto;
        margin-block-end: 0.75rem;
    }
    label {
        display: flex;
        align-items: center;
        gap: 0.4rem;
    }
    .button-row {
        margin-block-start: 0.75rem;
    }
</style>
