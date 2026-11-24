<script lang="ts">
    import { TEMPLATE_KEYS } from '@remnawave/backend-contract'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Overlay from '../ui/Overlay.svelte'

    let { keys = TEMPLATE_KEYS }: { keys?: readonly string[] } = $props()

    let open = $state(false)
    let copied = $state('')
    let t = $derived(locale.t)
    let title = $derived(t('template-info-popover.shared.template-variables'))

    async function copy(value: string): Promise<void> {
        await navigator.clipboard.writeText(value)
        copied = value
    }
</script>

<ActionIcon
    icon="TbInfoSquare"
    label={title}
    tone="lime"
    size={30}
    iconSize={18}
    subtle
    onclick={() => {
        copied = ''
        open = true
    }}
/>
{#if open}
    <Overlay {title} icon="TbInfoSquare" tone="lime" close={() => (open = false)}>
        <p>{t('template-info-popover.shared.you-can-use-template-variables-in-this-field')}</p>
        <div class="variables">
            {#each keys as key (key)}
                {@const value = `{{${key}}}`}
                <div>
                    <code>{value}</code>
                    <ActionIcon
                        icon={copied === value ? 'TbCheck' : 'TbCopy'}
                        label={t('common.action.copy')}
                        size={28}
                        iconSize={14}
                        subtle
                        onclick={() => void copy(value).catch(() => (copied = ''))}
                    />
                </div>
            {/each}
        </div>
    </Overlay>
{/if}

<style>
    .variables {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr));
        gap: var(--mantine-spacing-xs);
    }
    .variables > div {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 4px;
        padding: 4px 8px;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
    }
    code {
        overflow-wrap: anywhere;
    }
</style>
