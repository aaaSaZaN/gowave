<script lang="ts">
    import { buildNodeInstallCommand } from '@shared/lib/node-install-command'
    import { createQuery } from '@tanstack/svelte-query'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import { nodeSecretQuery } from './queries'

    let { port }: { port: number | undefined } = $props()
    const id = $props.id()
    let feedback = $state('')
    let failed = $state(false)
    let t = $derived(locale.t)
    const secret = createQuery(() => nodeSecretQuery)

    function copy(install: boolean): void {
        const key = secret.data?.secretKey.trim()
        if (!key) return
        const val = install ? buildNodeInstallCommand(key, port) : key
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
    <div>
        <label for={id}>Secret Key (SECRET_KEY)</label>
        <div class="secret-field">
            <span class="key-icon"><Icon name="TbServerCog" size={16} /></span>
            <input
                class="gw-input"
                {id}
                readonly
                value={secret.data?.secretKey ?? ''}
                spellcheck="false"
            >
            <span class="copy-key">
                <ActionIcon
                    icon="TbCopy"
                    size={24}
                    iconSize={16}
                    subtle
                    label={t('common.action.copy')}
                    disabled={!secret.data}
                    onclick={() => copy(false)}
                />
            </span>
        </div>
    </div>
    <Button
        variant="light"
        color="teal"
        icon="TbTerminal2"
        disabled={!secret.data}
        onclick={() => copy(true)}
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
    .install,
    label,
    .secret-field {
        display: grid;
        min-width: 0;
    }
    .install {
        gap: var(--mantine-spacing-xs);
    }
    label {
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .secret-field {
        position: relative;
    }
    .secret-field input {
        min-width: 0;
        padding-inline: 36px;
        font-family: var(--mantine-font-family-monospace);
    }
    .key-icon,
    .copy-key {
        position: absolute;
        inset-block: 0;
        display: grid;
        place-items: center;
        color: var(--mantine-color-dimmed);
    }
    .key-icon {
        inset-inline-start: 10px;
        pointer-events: none;
    }
    .copy-key {
        inset-inline-end: 6px;
    }
    p {
        margin: 0;
    }
</style>
