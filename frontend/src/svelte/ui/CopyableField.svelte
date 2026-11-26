<script lang="ts">
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'
    import { notifications } from './notification-state.svelte'

    let {
        id: passedId,
        label,
        value,
        size = 'sm',
        monospace = true,
        leftIcon,
        leftIconSize = 16,
        left,
        ariaLabel,
        timeout = 2000,
        oncopy
    }: {
        id?: string
        label?: string
        value: string | number
        size?: 'xs' | 'sm' | 'md' | 'lg'
        monospace?: boolean
        leftIcon?: IconName
        leftIconSize?: number
        left?: Snippet
        ariaLabel?: string
        timeout?: number
        oncopy?: (ok: boolean) => void
    } = $props()

    const fallbackId = $props.id()
    let inputId = $derived(passedId ?? fallbackId)
    let copied = $state(false)
    let timer: ReturnType<typeof setTimeout> | undefined
    let t = $derived(locale.t)
    let hasLeft = $derived(Boolean(left || leftIcon))

    async function copy(): Promise<void> {
        const text = String(value)
        if (!text) return
        try {
            if (!navigator.clipboard) throw new Error('Clipboard unavailable')
            await navigator.clipboard.writeText(text)
            copied = true
            clearTimeout(timer)
            timer = setTimeout(() => {
                copied = false
            }, timeout)
            oncopy?.(true)
        } catch (error) {
            oncopy?.(false)
            notifications.show({
                title: t('common.message.error'),
                message: error instanceof Error ? error.message : t('common.message.error'),
                color: 'red'
            })
        }
    }

    $effect(() => () => clearTimeout(timer))
</script>

<div class="mantine-InputWrapper-root copyable-field" data-size={size}>
    {#if label}
        <label for={inputId} class="mantine-InputWrapper-label copyable-label">{label}</label>
    {/if}
    <div
        class="mantine-Input-wrapper input-wrapper"
        data-size={size}
        data-with-left-section={hasLeft ? true : undefined}
        data-with-right-section="true"
    >
        {#if left}
            <div class="mantine-Input-section section" data-position="left">{@render left()}</div>
        {:else if leftIcon}
            <div class="mantine-Input-section section" data-position="left" aria-hidden="true">
                <Icon name={leftIcon} size={leftIconSize} />
            </div>
        {/if}

        <input
            id={inputId}
            type="text"
            class="mantine-Input-input copy-input"
            class:monospace
            readonly
            aria-label={label ? undefined : (ariaLabel ?? t('common.action.copy'))}
            value={String(value)}
            onclick={copy}
        >

        <div class="mantine-Input-section section" data-position="right">
            <ActionIcon
                icon={copied ? 'PiCheck' : 'PiCopy'}
                label={copied
                    ? t('common.message.copied')
                    : [t('common.action.copy'), ariaLabel ?? label].filter(Boolean).join(' ')}
                tone={copied ? 'teal' : 'gray'}
                subtle
                size={size === 'xs' ? 24 : 28}
                iconSize={16}
                onclick={copy}
            />
        </div>
    </div>
</div>

<style>
    .copyable-field {
        width: 100%;
        min-width: 0;
    }

    .copyable-label {
        display: block;
        margin-bottom: 4px;
    }

    .input-wrapper {
        position: relative;
        width: 100%;
        min-width: 0;
    }

    .input-wrapper[data-size="xs"] {
        --input-height: var(--input-height-xs);
        --input-fz: var(--mantine-font-size-xs);
    }

    .input-wrapper[data-size="md"] {
        --input-height: var(--input-height-md);
        --input-fz: var(--mantine-font-size-md);
    }

    .input-wrapper[data-size="lg"] {
        --input-height: var(--input-height-lg);
        --input-fz: var(--mantine-font-size-lg);
    }

    .section {
        z-index: 2;
        pointer-events: all;
    }

    .copy-input {
        cursor: copy;
        padding-inline-end: 34px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: var(--input-fz, var(--mantine-font-size-sm));
    }

    .copy-input.monospace {
        font-family: var(--mantine-font-family-monospace);
    }
</style>
