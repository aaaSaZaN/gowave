<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import { downloadProfile } from './download-profile'
    import { computedProfileQuery, type Profile } from './queries'

    let { profile, computed, close }: { profile: Profile; computed: boolean; close: () => void } =
        $props()
    let dialog: HTMLDialogElement
    const title = $props.id()
    let t = $derived(locale.t)
    let error = $state('')
    let copied = $state(false)
    const query = createQuery(() => ({ ...computedProfileQuery(profile.uuid), enabled: computed }))
    let json = $derived(JSON.stringify(computed ? query.data?.config : profile.config, null, 2))

    onMount(() => dialog.showModal())

    async function copy(): Promise<void> {
        if (json === undefined) return
        error = ''
        try {
            await navigator.clipboard.writeText(json)
            copied = true
        } catch (reason) {
            error = reason instanceof Error ? reason.message : t('common.message.error')
        }
    }

    function download(): void {
        if (json !== undefined) downloadProfile(profile.name, json)
    }
</script>

<dialog oncancel={cancelDialog} bind:this={dialog} onclose={close} aria-labelledby={title}>
    <div class="button-row">
        <h2 id={title}>
            {computed
                ? t('config-profile-card.widget.view-computed')
                : t('config-profiles-grid.widget.xray-config')}
            · {profile.name}
        </h2>
        <button type="button" onclick={() => dismissDialog(dialog)}>
            {t('common.action.close')}
        </button>
    </div>
    {#if computed && query.isPending}
        <p role="status">{t('config-profile-card.widget.loading-computed-config-profile')}</p>
    {:else if computed && query.error}
        <p role="alert">{query.error.message}</p>
        <button type="button" onclick={() => void query.refetch()}>
            {t('common.action.try-again')}
        </button>
    {:else if json !== undefined}
        <pre dir="ltr">{json}</pre>
        <div class="button-row">
            <button type="button" onclick={() => void copy()}>
                {copied ? t('common.message.copied') : t('common.action.copy')}
            </button>
            <button type="button" onclick={download}>{t('common.action.download')}</button>
        </div>
        {#if error}
            <p role="alert">{error}</p>
        {/if}
    {/if}
</dialog>

<style>
    dialog {
        width: min(55rem, calc(100vw - 2rem));
    }

    pre {
        max-height: 58vh;
        overflow: auto;
    }
</style>
