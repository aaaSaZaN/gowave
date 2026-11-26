<script lang="ts">
    import type { UpdateRemnawaveSettingsCommand } from '@remnawave/backend-contract'
    import { parseColoredTextUtil } from '@shared/utils/misc/parse-colored-text'
    import { untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import { optionalText } from './auth-fields'
    import type { Settings } from './queries'
    import SettingsCard from './SettingsCard.svelte'

    let {
        branding,
        busy,
        save
    }: {
        branding: Settings['brandingSettings']
        busy: boolean
        save: (values: UpdateRemnawaveSettingsCommand.RequestBody) => Promise<void>
    } = $props()

    let t = $derived(locale.t)
    let title = $state(untrack(() => branding?.title ?? ''))
    let logo = $state(untrack(() => branding?.logoUrl ?? ''))
    let preview = $derived(parseColoredTextUtil(title || 'Gowave'))
    let format = $state(false)
    let error = $state('')

    async function submit(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (busy || !(event.currentTarget instanceof HTMLFormElement)) return
        const form = new FormData(event.currentTarget)
        error = ''
        try {
            await save({
                brandingSettings: {
                    title: optionalText(form, 'title'),
                    logoUrl: optionalText(form, 'logoUrl')
                }
            })
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        } catch (reason) {
            error = reason instanceof Error ? reason.message : String(reason)
        }
    }
</script>

<form onsubmit={(event) => void submit(event)}>
    <SettingsCard
        title={t('branding-settings-card.widget.branding-settings')}
        description={t('branding-settings-card.widget.customize-your-gowave-instance')}
        icon="Logo"
    >
        {#snippet footer()}
            <div class="save">
                <Button type="submit" color="teal" size="md" variant="soft" disabled={busy}
                    >{t('common.action.save')}</Button
                >
            </div>
        {/snippet}
        <fieldset disabled={busy}>
            <div class="brand-field">
                <label for="brand-name">{t('branding-settings-card.widget.brand-name')}</label>
                <small id="brand-name-description"
                    >{t(
                        'branding-settings-card.widget.the-title-that-will-be-displayed-on-login-page'
                    )}</small
                >
                <div class="input-wrap">
                    <span class="start"><Icon name="TbStar" size={16} /></span>
                    <input
                        class="gw-input"
                        id="brand-name"
                        name="title"
                        bind:value={title}
                        placeholder="Gowave"
                        aria-describedby="brand-name-description"
                    >
                    <span class="end">
                        <ActionIcon
                            icon="HiQuestionMarkCircle"
                            label={t('branding-settings-card.widget.colored-title-format')}
                            size={18}
                            iconSize={20}
                            subtle
                            aria-expanded={format}
                            onclick={() => (format = !format)}
                        />
                    </span>
                </div>
                {#if format}
                    <div class="format">
                        <strong>{t('branding-settings-card.widget.colored-title-format')}</strong>
                        <p>{t('branding-settings-card.widget.colored-title-description')}</p>
                        {#each preview as part}
                            <span style:color={part.color === 'white' ? undefined : part.color}
                                >{part.text}</span
                            >
                        {/each}
                    </div>
                {/if}
            </div>
            <div class="brand-field">
                <label for="brand-logo">{t('common.field.logo-url')}</label>
                <small id="brand-logo-description"
                    >{t('branding-settings-card.widget.the-url-to-your-brand-logo-image')}</small
                >
                <div class="input-wrap">
                    <span class="start"><Icon name="TbLink" size={16} /></span>
                    <input
                        class="gw-input"
                        id="brand-logo"
                        name="logoUrl"
                        bind:value={logo}
                        placeholder="https://example.com/logo.png"
                        aria-describedby="brand-logo-description"
                    >
                </div>
            </div>
        </fieldset>
        {#if error}
            <p role="alert">{error}</p>
        {/if}
    </SettingsCard>
</form>

<style>
    fieldset {
        display: grid;
        gap: var(--mantine-spacing-md);
        border: 0;
        margin: 0;
        min-width: 0;
        padding: 0;
    }
    .brand-field {
        display: grid;
        position: relative;
        padding-top: 2px;
    }
    label {
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: 21.7px;
    }
    small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: 14.4px;
        margin-top: 1px;
    }
    .input-wrap {
        display: grid;
        position: relative;
        margin-top: 5px;
    }
    .start,
    .end {
        position: absolute;
        inset-block: 0;
        display: grid;
        width: 36px;
        place-items: center;
        color: var(--mantine-color-placeholder);
    }
    .start {
        inset-inline-start: 0;
        pointer-events: none;
    }
    .end {
        inset-inline-end: 0;
    }
    .input-wrap input {
        padding-inline: 36px;
        height: 36px;
        min-height: 36px;
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
    }
    .format {
        position: absolute;
        z-index: 2;
        max-width: 100%;
        width: 350px;
        inset-inline-end: 0;
        top: 100%;
        padding: var(--mantine-spacing-sm);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-body);
        box-shadow: var(--mantine-shadow-md);
        font-size: var(--mantine-font-size-sm);
    }
    .save {
        display: flex;
        justify-content: end;
    }
    @media (1400px <= width < 2000px) {
        form {
            margin-block-start: 0.6px;
        }
    }
</style>
