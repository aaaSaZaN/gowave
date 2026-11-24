<script lang="ts">
    import {
        readMiscPreferences,
        writeMiscPreferences
    } from '@entities/dashboard/misc-store/storage'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'

    let dialog: HTMLDialogElement
    const heading = $props.id()
    const t = $derived(locale.t)
    const terms = 'https://github.com/aaaSaZaN/gowave/blob/main/LICENSE'

    onMount(() => {
        try {
            if (readMiscPreferences(localStorage).disclaimerAccepted) return
        } catch {
            // Storage can be disabled.
        }
        dialog.showModal()
    })

    function accept(): void {
        try {
            writeMiscPreferences(localStorage, { disclaimerAccepted: true })
        } catch {
            // The dialog can be dismissed for this visit.
        }
        dialog.close()
    }

    function parts(value: string) {
        return value
            .split(/(<(?:highlight|warning|anchor)>.*?<\/(?:highlight|warning|anchor)>)/g)
            .map((segment) => {
                const match = /^<(highlight|warning|anchor)>(.*?)<\/\1>$/.exec(segment)
                return match ? { type: match[1], text: match[2] } : { type: 'text', text: segment }
            })
    }
</script>

{#snippet rich(value: string)}
    {#each parts(value) as part}
        {#if part.type === 'anchor'}
            <a href={terms} target="_blank" rel="noopener noreferrer">{part.text}</a>
        {:else if part.type === 'highlight' || part.type === 'warning'}
            <strong class:warning={part.type === 'warning'}>{part.text}</strong>
        {:else}
            {part.text}
        {/if}
    {/each}
{/snippet}

<dialog
    class="disclaimer notice-dialog"
    bind:this={dialog}
    aria-labelledby={heading}
    oncancel={(event) => event.preventDefault()}
>
    <div class="notice">
        <div class="warning-icon"><Icon name="PiShieldWarning" size={36} /></div>
        <div class="notice-title">
            <h2 id={heading}>{t('disclaimer-overlay.title')}</h2>
            <p>{t('disclaimer-overlay.subtitle')}</p>
        </div>
        <hr>
        <div class="notice-body">
            <p>{@render rich(t('disclaimer-overlay.intro'))}</p>
            <ul>
                {#each [
                    t('disclaimer-overlay.responsibility'),
                    t('disclaimer-overlay.compliance'),
                    t('disclaimer-overlay.liability')
                ] as item (item)}
                    <li>
                        <span class="gavel"><Icon name="PiGavel" size={12} /></span>
                        <span>{@render rich(item)}</span>
                    </li>
                {/each}
            </ul>
            <p class="license">{@render rich(t('disclaimer-overlay.license-acknowledgement'))}</p>
        </div>
        <Button
            block
            color="yellow"
            variant="light"
            size="md"
            icon="PiCheckCircle"
            iconSize={18}
            onclick={accept}
        >
            {t('disclaimer-overlay.accept-button')}
        </Button>
    </div>
</dialog>

<style>
    .disclaimer {
        width: min(620px, calc(100vw - 40px));
        max-height: calc(100dvh - 40px);
        max-width: none;
        inset: 50dvh auto auto 50vw;
        margin: 0;
        transform: translate(-50%, -50%);
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: rgb(var(--gw-surface-rgb));
        box-shadow: var(--mantine-shadow-xl);
    }

    .disclaimer::backdrop {
        background: rgb(0 0 0 / 60%);
    }

    .notice {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--mantine-spacing-lg);
    }

    .warning-icon {
        display: grid;
        width: 64px;
        height: 64px;
        place-items: center;
        border: 2px solid rgb(250 204 21 / 30%);
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-yellow-light-color);
        background: rgb(250 204 21 / 10%);
    }

    .notice-title {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-gray-4);
        font-weight: 700;
        text-align: center;
    }

    .notice-title h2 {
        margin: 0;
        color: var(--mantine-color-yellow-4);
        font-size: var(--mantine-h3-font-size);
        font-weight: var(--mantine-h3-font-weight);
        line-height: var(--mantine-h3-line-height);
    }

    p {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }

    hr {
        width: 100%;
        margin: 0;
        border: 0;
        border-top: 1px dashed var(--mantine-color-yellow-4);
        opacity: 0.3;
    }

    .notice-body,
    ul {
        display: flex;
        flex-direction: column;
    }

    .notice-body {
        width: 100%;
        gap: var(--mantine-spacing-md);
        color: var(--mantine-color-gray-3);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }

    ul {
        margin: 0;
        padding: 0;
        gap: var(--mantine-spacing-sm);
        list-style: none;
    }

    li {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
    }

    .gavel {
        display: grid;
        width: 20px;
        height: 20px;
        place-items: center;
        flex: 0 0 auto;
        border-radius: var(--mantine-radius-xl);
        color: var(--mantine-color-yellow-light-color);
        background: var(--mantine-color-yellow-light);
    }

    strong {
        color: var(--mantine-color-gray-1);
    }

    .warning {
        color: var(--mantine-color-yellow-4);
    }

    .license {
        color: var(--mantine-color-gray-5);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
        text-align: center;
    }

    a {
        color: var(--mantine-color-cyan-4);
        font-weight: 600;
        text-decoration: none;
    }

    a:hover {
        text-decoration: underline;
    }
</style>
