<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { downloadProfile } from './download-profile'
    import { computedProfileQuery, type Profile } from './queries'

    let { profile, computed, close }: { profile: Profile; computed: boolean; close: () => void } =
        $props()
    let t = $derived(locale.t)
    let error = $state('')
    let copied = $state(false)
    const query = createQuery(() => ({ ...computedProfileQuery(profile.uuid), enabled: computed }))
    let json = $derived(JSON.stringify(computed ? query.data?.config : profile.config, null, 2))

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

<Overlay
    title={computed
        ? t('config-profile-card.widget.view-computed')
        : t('config-profiles-grid.widget.xray-config')}
    icon="TbCode"
    width={880}
    {close}
    subtitle={profile.name}
    fullscreenOnMobile={false}
>
    {#if computed && query.isPending}
        <p role="status">{t('config-profile-card.widget.loading-computed-config-profile')}</p>
    {:else if computed && query.error}
        <p role="alert">{query.error.message}</p>
        <Button onclick={() => void query.refetch()}>
            {t('common.action.try-again')}
        </Button>
    {:else if json !== undefined}
        <pre dir="ltr">{json}</pre>
        <div class="button-row">
            <Button onclick={() => void copy()}>
                {copied ? t('common.message.copied') : t('common.action.copy')}
            </Button>
            <Button onclick={download}>{t('common.action.download')}</Button>
        </div>
        {#if error}
            <p role="alert">{error}</p>
        {/if}
    {/if}
</Overlay>

<style>
    pre {
        max-height: 58vh;
        overflow: auto;
    }
</style>
