<script lang="ts">
    import { buildNodeInstallCommand } from '@shared/lib/node-install-command'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import CopyableField from '../ui/CopyableField.svelte'
    import { nodeSecretQuery } from './queries'

    let { port }: { port: number | undefined } = $props()
    let feedback = $state('')
    let failed = $state(false)
    let t = $derived(locale.t)
    const secret = createQuery(() => nodeSecretQuery)

    function copyInstall(): void {
        const key = secret.data?.secretKey.trim()
        if (!key) return
        const val = buildNodeInstallCommand(key, port)
        void navigator.clipboard.writeText(val).then(
            () => {
                failed = false
                feedback = t('common.message.copied')
            },
            (error: unknown) => {
                failed = true
                feedback = error instanceof Error ? error.message : String(error)
            }
        )
    }
</script>

<div class="install">
    <CopyableField
        label="Secret Key (SECRET_KEY)"
        leftIcon="TbServerCog"
        value={secret.data?.secretKey ?? ''}
        oncopy={(ok) => {
            if (ok) {
                failed = false
                feedback = t('common.message.copied')
            }
        }}
    />
    <Button
        variant="light"
        color="teal"
        icon="TbTerminal2"
        disabled={!secret.data}
        onclick={copyInstall}
    >
        {t('copy-node-install-command.widget.copy-install-command')}
    </Button>
    {#if secret.error}
        <p role="alert">{secret.error.message}</p>
    {/if}
    {#if feedback}
        <p role={failed ? 'alert' : 'status'}>{feedback}</p>
    {/if}
</div>
<style>
    .install {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-xs);
    }
    p {
        margin: 0;
    }
</style>
