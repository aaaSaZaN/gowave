<script lang="ts">
    import { blockNavigation } from 'sv-router'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { session } from '../session.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'

    let { dirty }: { dirty: boolean } = $props()
    const t = $derived(locale.t)
    let pending = $state<Promise<boolean>>()
    let resolve: ((leave: boolean) => void) | undefined

    function settle(leave: boolean): void {
        resolve?.(leave)
        resolve = undefined
        pending = undefined
    }

    onMount(() => {
        const unblock = blockNavigation({
            beforeUnload: () => !dirty,
            onNavigate: () => {
                if (!session.authenticated || !dirty) return true
                pending ??= new Promise<boolean>((answer) => {
                    resolve = answer
                })
                return pending
            }
        })
        return () => {
            unblock()
            settle(false)
        }
    })
</script>

{#if pending}
    <Overlay
        title={t('config-editor.widget.unsaved-changes')}
        icon="TbAlertTriangle"
        tone="red"
        width={455}
        fullscreenOnMobile={false}
        close={() => settle(false)}
    >
        <p>
            {t(
                'config-editor.widget.your-changes-will-be-lost-if-you-leave-this-page-without-saving'
            )}
        </p>
        <div class="actions">
            <Button variant="light" color="gray" onclick={() => settle(false)}
                >{t('config-editor.widget.stay')}</Button
            >
            <Button variant="soft" color="red" onclick={() => settle(true)}
                >{t('config-editor.widget.leave')}</Button
            >
        </div>
    </Overlay>
{/if}

<style>
    p {
        margin: 0 0 var(--mantine-spacing-md);
    }
    .actions {
        display: flex;
        gap: var(--mantine-spacing-sm);
        justify-content: end;
        flex-wrap: wrap;
    }
</style>
