<script lang="ts">
    import { onDestroy, tick } from 'svelte'
    import {
        generateMlDsa65,
        generateMlKem768,
        generateX25519
    } from '../../widgets/dashboard/config-profiles/keypair-generator/keypair-utils'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'

    let { close }: { close: () => void } = $props()
    const tabs = [
        ['X25519', 'TbKey'],
        ['ML-DSA65', 'TbCertificate'],
        ['ML-KEM768', 'TbLock']
    ] as const satisfies readonly [string, IconName][]
    type Algorithm = (typeof tabs)[number][0]
    const id = $props.id()
    let algorithm = $state<Algorithm>('X25519')
    let x25519 = $state(generateX25519())
    let dsa = $state<ReturnType<typeof generateMlDsa65>>()
    let kem = $state<ReturnType<typeof generateMlKem768>>()
    let t = $derived(locale.t)
    let copied = $state('')
    let failed = $state(false)
    let timer: ReturnType<typeof setTimeout> | undefined

    onDestroy(() => clearTimeout(timer))

    function select(next: Algorithm): void {
        algorithm = next
        if (next === 'ML-DSA65') dsa ??= generateMlDsa65()
        if (next === 'ML-KEM768') kem ??= generateMlKem768()
    }

    function regenerate(): void {
        copied = ''
        failed = false
        if (algorithm === 'X25519') x25519 = generateX25519()
        else if (algorithm === 'ML-DSA65') dsa = generateMlDsa65()
        else kem = generateMlKem768()
    }

    async function copy(label: string, value: string): Promise<void> {
        try {
            await navigator.clipboard.writeText(value)
            failed = false
            copied = label
            clearTimeout(timer)
            timer = setTimeout(() => (copied = ''), 2000)
        } catch {
            failed = true
        }
    }
</script>

<Overlay
    title={t('config-editor-actions.feature.tools')}
    icon="TbTools"
    tone="teal"
    width={440}
    {close}
>
    <div
        class="tabs"
        role="tablist"
        aria-label={t('config-editor-actions.feature.generate-keypair')}
    >
        {#each tabs as [name, icon], index (name)}
            <button
                type="button"
                class="mantine-UnstyledButton-root"
                role="tab"
                id={`${id}-${name}`}
                aria-controls={`${id}-panel`}
                aria-selected={name === algorithm}
                tabindex={name === algorithm ? 0 : -1}
                onclick={() => select(name)}
                onkeydown={(event) => {
                    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
                    event.preventDefault()
                    const offset = event.key === 'ArrowLeft' ? -1 : 1
                    const next = tabs[(index + offset + tabs.length) % tabs.length]?.[0]
                    if (next) {
                        select(next)
                        void tick().then(() => document.getElementById(`${id}-${next}`)?.focus())
                    }
                }}
            >
                <Icon name={icon} size={16} />{name}
            </button>
        {/each}
    </div>
    <div class="keys" role="tabpanel" id={`${id}-panel`} aria-labelledby={`${id}-${algorithm}`}>
        {#if algorithm === 'X25519'}
            {@render field('Password', x25519.password)}
            {@render field('Private Key', x25519.privateKey)}
            <hr>
            {@render field(
                t('keypair.widget.both-keys'),
                `"password": "${x25519.password}",\n"privateKey": "${x25519.privateKey}",`,
                true
            )}
        {:else if algorithm === 'ML-DSA65' && dsa}
            {@render field('mldsa65Seed (server side)', dsa.mldsa65Seed)}
            {@render field('mldsa65Verify (Client side, pqv)', dsa.mldsa65Verify)}
            <hr>
            {@render field(
                t('keypair.widget.both-keys'),
                `"mldsa65Seed": "${dsa.mldsa65Seed}",\n"mldsa65Verify": "${dsa.mldsa65Verify}",`,
                true
            )}
        {:else if kem}
            {@render field('Server side, used in decryption', kem.mlkem768Seed)}
            {@render field('Client side, used in encryption', kem.mlkem768PublicKey)}
        {/if}
        {#if failed}
            <p role="alert">{t('common.message.error')}</p>
        {/if}
        <div class="actions">
            <Button variant="default" icon="PiKey" onclick={regenerate}>
                {t(
                    algorithm === 'X25519'
                        ? 'keypair-generator.widget.generate'
                        : 'keypair.widget.generate-key-pair'
                )}
            </Button>
        </div>
    </div>
</Overlay>

{#snippet field(label: string, value: string, multiline = false)}
    <div class="field">
        <span>{label}</span>
        <div class="control">
            {#if multiline}
                <textarea class="gw-input" aria-label={label} readonly {value} rows="3"></textarea>
            {:else}
                <input class="gw-input" aria-label={label} readonly {value} spellcheck={false}>
            {/if}
            <ActionIcon
                icon={copied === label ? 'PiCheck' : 'PiCopy'}
                tone={copied === label ? 'teal' : 'gray'}
                label={t(copied === label ? 'common.message.copied' : 'common.action.copy')}
                subtle
                size={30}
                iconSize={16}
                onclick={() => void copy(label, value)}
            />
        </div>
    </div>
{/snippet}

<style>
    .tabs {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
    }
    .tabs button {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 6px;
        padding: 9px 3px;
        border-block-end: 2px solid var(--mantine-color-default-border);
        font-size: var(--mantine-font-size-xs);
        white-space: nowrap;
    }
    .tabs button[aria-selected="true"] {
        border-color: var(--mantine-primary-color-filled);
    }
    .keys {
        display: grid;
        gap: var(--mantine-spacing-xs);
        margin-block-start: var(--mantine-spacing-md);
    }
    .field > span {
        font-size: var(--mantine-font-size-sm);
    }
    .control {
        position: relative;
        display: grid;
    }
    .control :is(input, textarea) {
        width: 100%;
        min-width: 0;
        padding-inline-end: 36px;
        font-family: var(--mantine-font-family-monospace);
    }
    .control :global(.action-icon) {
        position: absolute;
        inset-block-start: 3px;
        inset-inline-end: 3px;
    }
    hr {
        width: 100%;
        border: 0;
        border-block-start: 1px solid var(--mantine-color-default-border);
    }
    .actions {
        display: flex;
        justify-content: end;
        margin-block-start: var(--mantine-spacing-xs);
    }
</style>
