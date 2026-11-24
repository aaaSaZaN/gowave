<script lang="ts">
    import { locale } from '../i18n.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'
    import Overlay from './Overlay.svelte'

    let {
        icon,
        tone,
        close,
        choose
    }: {
        icon: IconName
        tone: string
        close: () => void
        choose: (immediate: boolean) => void
    } = $props()
    const t = $derived(locale.t)
</script>

<Overlay title={t('apply-to-nodes.title')} {icon} {tone} width={440} {close}>
    <div class="stack">
        <button type="button" class="apply-choice" onclick={() => choose(true)}>
            <Icon name="TbRocket" size={22} />
            <span
                ><strong>{t('apply-to-nodes.apply')}</strong
                ><small>{t('apply-to-nodes.apply-description')}</small></span
            >
        </button>
        <button type="button" class="apply-choice" onclick={() => choose(false)}>
            <Icon name="TbDeviceFloppy" size={22} />
            <span
                ><strong>{t('apply-to-nodes.later')}</strong
                ><small>{t('apply-to-nodes.later-description')}</small></span
            >
        </button>
    </div>
</Overlay>

<style>
    .stack {
        display: grid;
        gap: var(--mantine-spacing-sm);
    }
    .apply-choice {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm);
        text-align: start;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-text);
    }
    .apply-choice:hover {
        background: var(--mantine-color-default-hover);
    }
    .apply-choice :is(strong, small) {
        display: block;
    }
    .apply-choice small {
        color: var(--mantine-color-dimmed);
    }
</style>
