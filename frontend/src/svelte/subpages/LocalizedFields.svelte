<script lang="ts">
    import { getLanguageName } from '@remnawave/subscription-page-types'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import type { LocalizedText, PageConfig } from './config'

    let {
        label,
        locales,
        multiline = false,
        value = $bindable()
    }: {
        label: string
        locales: PageConfig['locales']
        multiline?: boolean
        value: LocalizedText
    } = $props()

    const id = $props.id()
    let open = $state(false)
    const filled = $derived(locales.filter((key) => value[key]?.trim()).length)
    function update(key: string, text: string): void {
        value = { ...value, [key]: text }
    }
</script>

<button
    type="button"
    class="localized mantine-UnstyledButton-root"
    aria-haspopup="dialog"
    onclick={() => (open = true)}
>
    <Icon name="TbLanguage" size={16} />
    <span class="copy"
        ><small>{label}</small
        ><span
            >{(locales[0] && value[locales[0]]) || locale.t('common.message.not-set')}</span
        ></span
    >
    <span class="badge" class:complete={filled === locales.length}>{filled}/{locales.length}</span>
</button>
{#if open}
    <Overlay
        title={label}
        icon="TbLanguage"
        width={680}
        fullscreenOnMobile={false}
        close={() => (open = false)}
    >
        <div class="translations">
            {#each locales as key, index (key)}
                <label for={`${id}-${key}`}>
                    {getLanguageName(key)}
                    {#if index === 0}
                        <small>Primary language</small>
                    {/if}
                    {#if multiline}
                        <textarea
                            id={`${id}-${key}`}
                            rows="3"
                            data-autofocus={index === 0 || undefined}
                            value={value[key] ?? ''}
                            oninput={(event) => update(key, event.currentTarget.value)}
                        ></textarea>
                    {:else}
                        <input
                            id={`${id}-${key}`}
                            data-autofocus={index === 0 || undefined}
                            value={value[key] ?? ''}
                            oninput={(event) => update(key, event.currentTarget.value)}
                        >
                    {/if}
                </label>
            {/each}
            <Button variant="light" block onclick={() => (open = false)}
                >{locale.t('localized-text-editor.component.done')}</Button
            >
        </div>
    </Overlay>
{/if}
<style>
    .localized {
        width: 100%;
        min-width: 0;
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        text-align: start;
        color: var(--mantine-color-dimmed);
        font: inherit;
    }
    .localized:hover {
        background: rgba(var(--gw-fg-rgb), 0.04);
        border-color: rgba(34, 211, 238, 0.3);
    }
    .copy {
        display: grid;
        min-width: 0;
        flex: 1;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-font-size-sm);
    }
    .copy > * {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .badge {
        border: 1px solid currentColor;
        border-radius: 99px;
        padding: 0 8px;
        font-size: 10px;
        font-weight: 700;
    }
    .complete {
        color: var(--mantine-color-green-5);
    }
    .translations,
    label {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    label {
        gap: 4px;
        font-size: var(--mantine-font-size-sm);
    }
</style>
