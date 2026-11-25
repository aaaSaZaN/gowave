<script lang="ts">
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
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
    let t = $derived(locale.t)
    let image: HTMLImageElement | undefined = $state()
    const foregroundColors = [
        '#3CC9DB',
        '#A78BFA',
        '#38BDF8',
        '#FB923C',
        '#34D399',
        '#F472B6',
        '#FACC15',
        '#EF4444',
        '#2DD4BF',
        '#C084FC',
        '#FBB324',
        '#4ADE80',
        '#F97316',
        '#EC4899',
        '#6366F1',
        '#E9ECEF'
    ]
    const backgroundColors = [
        '#000000',
        '#0D1117',
        '#161B22',
        '#1A1B26',
        '#1E1E2E',
        '#1E293B',
        '#2E3440',
        '#282A36',
        '#0F172A',
        '#18181B',
        '#1C1917',
        '#172554',
        '#14532D',
        '#7F1D1D',
        '#FFFFFF',
        '#F5F5F4'
    ]
    let foreground = $state('#3CC9DB')
    let background = $state('#161B22')
    let pending = $state(false)
    let error = $state('')
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
        void import('@shared/utils/copy-screenshot.util').then(
            (module) => (exporter = module),
            (failure: unknown) =>
                (error = failure instanceof Error ? failure.message : t('common.message.error'))
        )
    })

    async function exportImage(action: 'copy' | 'download'): Promise<void> {
        if (!image || !exporter || pending) return
        pending = true
        error = ''
        try {
            if (action === 'copy') {
                await exporter.copyImageScreenshotToClipboard(image, filename)
                notifications.show({
                    title: t('common.message.copied'),
                    message: label,
                    color: 'teal'
                })
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

<Overlay title={label} icon="TbQrcode" tone="teal" width={640} fullscreenOnMobile={false} {close}>
    <div class="builder">
        <div class="palette">
            {@render swatches(
                'Foreground color',
                foregroundColors,
                foreground,
                (color) => (foreground = color)
            )}
            {@render swatches(
                'Background color',
                backgroundColors,
                background,
                (color) => (background = color)
            )}
        </div>
        <div class="preview">
            {#if preview.src}
                <img bind:this={image} src={preview.src} alt={t('view-user-modal.widget.qr-code')}>
            {/if}
            {#if preview.error || error}
                <p role="alert">{preview.error || error}</p>
            {/if}
            <div class="actions">
                <Button
                    variant="filled"
                    color="cyan"
                    icon="TbCopy"
                    disabled={!exporter || !preview.src || pending}
                    onclick={() => void exportImage('copy')}
                >
                    {t('common.action.copy')}
                </Button>
                <Button
                    variant="default"
                    icon="TbDownload"
                    disabled={!exporter || !preview.src || pending}
                    onclick={() => void exportImage('download')}
                >
                    {t('common.action.download')}
                </Button>
            </div>
        </div>
    </div>
</Overlay>

{#snippet swatches(
    label: string,
    colors: string[],
    selection: string,
    choose: (color: string) => void
)}
    <fieldset>
        <legend>{label}</legend>
        <div class="swatches">
            {#each colors as color (color)}
                <button
                    type="button"
                    class="mantine-UnstyledButton-root"
                    style:background={color}
                    aria-label={`${label} ${color}`}
                    aria-pressed={selection === color}
                    onclick={() => choose(color)}
                >
                    {#if selection === color}
                        <Icon name="TbCheck" size={14} />
                    {/if}
                </button>
            {/each}
        </div>
    </fieldset>
{/snippet}

<style>
    .builder {
        display: grid;
        grid-template-columns: minmax(190px, 238px) minmax(0, 1fr);
        gap: var(--mantine-spacing-md);
    }
    .palette,
    .preview {
        display: grid;
        align-content: start;
        gap: var(--mantine-spacing-sm);
    }
    fieldset {
        min-width: 0;
        margin: 0;
        padding: 8px;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
    }
    legend {
        padding-inline: 4px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .swatches {
        display: grid;
        grid-template-columns: repeat(8, minmax(0, 1fr));
        gap: 4px;
    }
    .swatches button {
        display: grid;
        aspect-ratio: 1;
        place-items: center;
        color: white;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-sm);
    }
    .swatches button :global(svg) {
        filter: drop-shadow(0 0 2px black);
    }
    img {
        display: block;
        width: 100%;
        height: auto;
        border-radius: var(--mantine-radius-md);
    }
    .actions {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-xs);
    }
    @media (max-width: 38rem) {
        .builder {
            grid-template-columns: minmax(0, 1fr);
        }
        .preview {
            grid-row: 1;
        }
    }
</style>
