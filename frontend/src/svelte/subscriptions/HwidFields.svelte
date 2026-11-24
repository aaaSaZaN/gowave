<script lang="ts">
    import { untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Icon from '../ui/Icon.svelte'
    import Switch from '../ui/Switch.svelte'
    import type { HwidSettings } from './settings-fields'
    import TemplateVariables from './TemplateVariables.svelte'

    let { value, defaultLimit }: { value: HwidSettings | null; defaultLimit: number } = $props()
    const announceId = $props.id()
    const limitId = `${announceId}-limit`
    let limitInput: HTMLInputElement
    let limit = $state<number | undefined>(
        untrack(() => value?.fallbackDeviceLimit ?? defaultLimit)
    )
    let t = $derived(locale.t)

    $effect(() => {
        limit = value?.fallbackDeviceLimit ?? defaultLimit
    })

    function adjustLimit(direction: 1 | -1): void {
        if (direction === 1) limitInput.stepUp()
        else limitInput.stepDown()
        limit = limitInput.valueAsNumber
    }
</script>

<div class="toggle">
    <strong>{t('subscription-hwid-settings.widget.hwid-limit')}</strong>
    <Switch
        tone="cyan"
        label={t('subscription-hwid-settings.widget.hwid-limit')}
        name="enabled"
        checked={value?.enabled ?? false}
    />
</div>
<div class="field">
    <label for={limitId}>{t('subscription-hwid-settings.widget.fallback-device-limit')}</label>
    <small id={`${limitId}-description`}>
        {t('subscription-hwid-settings.widget.fallback-device-limit-description')}
    </small>
    <span>
        <span class="leading"><Icon name="TbDevices2" size={16} /></span>
        <input
            id={limitId}
            aria-describedby={`${limitId}-description`}
            name="fallbackDeviceLimit"
            min="0"
            max="9999"
            required
            type="number"
            bind:this={limitInput}
            bind:value={limit}
            placeholder="999"
        >
        <span class="limit-controls">
            <button
                type="button"
                class="mantine-UnstyledButton-root"
                aria-label="Increment"
                disabled={limit === 9999}
                onclick={() => adjustLimit(1)}
            >
                <Icon name="TbChevronUp" size={14} />
            </button>
            <button
                type="button"
                class="mantine-UnstyledButton-root"
                aria-label="Decrement"
                disabled={limit === 0}
                onclick={() => adjustLimit(-1)}
            >
                <Icon name="TbChevronDown" size={14} />
            </button>
        </span>
    </span>
</div>
<div class="field">
    <label for={announceId}>{t('subscription-hwid-settings.widget.max-devices-announce')}</label>
    <small id={`${announceId}-description`}>
        {t('subscription-hwid-settings.widget.max-devices-announce-description')}
    </small>
    <span>
        <span class="variables"><TemplateVariables /></span>
        <textarea
            id={announceId}
            aria-describedby={`${announceId}-description`}
            name="maxDevicesAnnounce"
            maxlength="200"
            rows="2"
            value={value?.maxDevicesAnnounce ?? ''}
            placeholder={t('subscription-hwid-settings.widget.max-200-characters')}
        ></textarea>
    </span>
</div>

<style>
    .toggle {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
    }
    .toggle strong {
        font-weight: 600;
    }
    .field label {
        display: inline-block;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height);
    }
    .field small {
        display: block;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: 1.2;
    }
    .field > span {
        display: grid;
        position: relative;
        margin-block-start: 5px;
    }
    .leading {
        position: absolute;
        inset-inline-start: 10px;
        top: 50%;
        transform: translateY(-50%);
        color: var(--mantine-color-dimmed);
    }
    .variables {
        position: absolute;
        inset-inline-start: 4px;
        top: 50%;
        transform: translateY(-50%);
    }
    .field :is(input, textarea) {
        min-width: 0;
        min-height: 36px;
        padding-inline-start: 34px;
        font-size: var(--mantine-font-size-sm);
        border-radius: var(--mantine-radius-md);
    }
    .field :is(input, textarea)::placeholder {
        color: var(--mantine-color-placeholder);
    }
    input[type="number"] {
        appearance: textfield;
        height: 36px;
        padding-block: 0;
        padding-inline-end: 24px;
        line-height: 34px;
    }
    input::-webkit-inner-spin-button {
        appearance: none;
    }
    textarea {
        height: calc(2lh + 13px);
        padding-block: 5.5px;
        line-height: var(--mantine-line-height);
        resize: none;
    }
    .limit-controls {
        position: absolute;
        inset: 1px 1px 1px auto;
        display: grid;
        width: 24px;
        border-inline-start: 1px solid var(--mantine-color-default-border);
    }
    .limit-controls button {
        display: grid;
        place-items: center;
    }
    .limit-controls button:disabled {
        color: var(--mantine-color-placeholder);
    }
</style>
