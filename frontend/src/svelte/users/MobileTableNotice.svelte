<script lang="ts">
    import {
        readMiscPreferences,
        writeMiscPreferences
    } from '@entities/dashboard/misc-store/storage'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'

    let dialog: HTMLDialogElement
    const heading = $props.id()
    const t = $derived(locale.t)
    const actions = $derived<{ name: IconName; text: string }[]>([
        { name: 'TbColumns', text: t('mobile-warning-overlay.show-or-hide-columns') },
        {
            name: 'TbBaselineDensitySmall',
            text: t('mobile-warning-overlay.adjust-row-spacing-density')
        },
        { name: 'TbMaximize', text: t('mobile-warning-overlay.toggle-fullscreen-table-view') }
    ])

    onMount(() => {
        try {
            if (readMiscPreferences(localStorage).mobileWarningClosed) return
        } catch {
            // Storage can be disabled.
        }
        dialog.showModal()
    })

    function save(): void {
        try {
            writeMiscPreferences(localStorage, { mobileWarningClosed: true })
        } catch {
            // The dialog can be dismissed for this visit.
        }
    }
</script>

<dialog
    class="mobile-warning notice-dialog"
    aria-labelledby={heading}
    bind:this={dialog}
    closedby="any"
    onclose={save}
>
    <div class="modal-copy">
        <span class="warning"><Icon name="PiWarning" size={32} /></span>
        <div class="description">
            <h2 id={heading}>{t('mobile-warning-overlay.mobile-device-detected')}</h2>
            <p class="lead">{t('mobile-warning-overlay.description-line-1')}</p>
            <p>{t('mobile-warning-overlay.description-line-2')}</p>
            {#each actions as action (action.name)}
                <div class="hint">
                    <Icon name={action.name} size="1.2rem" />{action.text}
                </div>
            {/each}
        </div>
        <hr>
        <div class="device-recommendations">
            <span class="desktop hint">
                <Icon name="PiMonitor" size="1.2rem" />
                {t('mobile-warning-overlay.desktop-recommended')}
            </span>
            <span class="or">{t('mobile-warning-overlay.or')}</span>
            <span class="rotate hint">
                <Icon name="TbRotate2" size="1.2rem" />{t('mobile-warning-overlay.rotate-device')}
            </span>
        </div>
        <Button
            block
            variant="light"
            color="orange"
            icon="PiDeviceMobile"
            size="md"
            onclick={() => dialog.close()}
        >
            {t('mobile-warning-overlay.i-understand-continue-anyway')}
        </Button>
    </div>
</dialog>

<style>
    .mobile-warning {
        width: min(440px, calc(100vw - 40px));
        max-height: calc(100dvh - 40px);
        max-width: none;
        inset: 50dvh auto auto 50vw;
        margin: 0;
        transform: translate(-50%, -50%);
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-gray-4);
        background: rgb(var(--gw-surface-rgb));
        box-shadow: var(--mantine-shadow-xl);
        text-align: center;
    }

    .mobile-warning::backdrop {
        background: rgb(0 0 0 / 60%);
    }

    .modal-copy,
    .description {
        display: flex;
        flex-direction: column;
        align-items: center;
    }

    .modal-copy {
        width: min(100%, 320px);
        margin-inline: auto;
        gap: var(--mantine-spacing-lg);
    }

    .description {
        gap: var(--mantine-spacing-sm);
    }

    .warning {
        display: grid;
        width: 44px;
        height: 44px;
        place-items: center;
        border: 2px solid rgb(251 146 60 / 30%);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-orange-light-color);
        background: rgb(251 146 60 / 10%);
    }

    h2 {
        margin: 0;
        color: var(--mantine-color-orange-4);
        font-size: var(--mantine-h3-font-size);
        font-weight: var(--mantine-h3-font-weight);
        line-height: var(--mantine-h3-line-height);
    }

    p {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }

    .lead {
        font-weight: 800;
    }

    hr {
        width: 100%;
        margin: 0;
        border: 0;
        border-top: 1px dashed var(--mantine-color-cyan-4);
        opacity: 0.3;
    }

    .device-recommendations,
    .hint {
        display: flex;
        justify-content: center;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }

    .device-recommendations {
        flex-wrap: wrap;
        gap: var(--mantine-spacing-md);
    }

    .desktop {
        color: var(--mantine-color-blue-4);
    }

    .rotate {
        color: var(--mantine-color-teal-4);
    }

    .or {
        color: var(--mantine-color-gray-5);
    }
</style>
