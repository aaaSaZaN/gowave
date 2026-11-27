<script lang="ts">
    import {
        BASE_TRANSLATION_LABELS,
        getLanguagesArray,
        INSTALLATION_GUIDE_BLOCKS_VARIANTS_VALUES,
        SUBSCRIPTION_INFO_BLOCK_VARIANTS_VALUES
    } from '@remnawave/subscription-page-types'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import ThemeIcon from '../ui/ThemeIcon.svelte'
    import { type PageConfig, type PlatformKey, platformLabels } from './config'
    import Icons from './Icons.svelte'
    import LocalizedFields from './LocalizedFields.svelte'
    import PlatformFields from './PlatformFields.svelte'

    const askConfirmation = useConfirmation()

    let { config = $bindable() }: { config: PageConfig } = $props()
    let t = $derived(locale.t)
    const languages = getLanguagesArray()
    const platformKeys: PlatformKey[] = [
        'ios',
        'android',
        'linux',
        'macos',
        'windows',
        'androidTV',
        'appleTV'
    ]
    const configured = $derived(Object.keys(config.platforms) as PlatformKey[])
    const platformIcons = {
        android: 'TbBrandAndroid',
        androidTV: 'TbDeviceTv',
        appleTV: 'TbDeviceTv',
        ios: 'TbBrandApple',
        linux: 'TbDeviceDesktop',
        macos: 'TbBrandApple',
        windows: 'TbBrandWindows'
    } as const
    let available = $derived(platformKeys.filter((key) => !config.platforms[key]))
    let loaded = $state<Partial<Record<PlatformKey, boolean>>>({})
    let overlay = $state<'translations' | 'icons'>()
    const translationKeys = Object.keys(
        BASE_TRANSLATION_LABELS
    ) as (keyof PageConfig['baseTranslations'])[]
    let translated = $derived(
        translationKeys.filter((key) =>
            config.locales.every((code) => config.baseTranslations[key][code]?.trim())
        ).length
    )

    function addPlatform(key: PlatformKey | ''): void {
        if (!key || config.platforms[key]) return
        config.platforms[key] = {
            displayName: config.locales[0] ? { [config.locales[0]]: platformLabels[key] } : {},
            svgIconKey: '',
            apps: []
        }
    }

    async function removePlatform(key: PlatformKey): Promise<void> {
        if (await askConfirmation(t('common.message.confirm-action-description'))) {
            delete config.platforms[key]
            delete loaded[key]
        }
    }

    function setLocale(code: PageConfig['locales'][number], enabled: boolean): void {
        if (enabled) config.locales = [...config.locales, code]
        else if (config.locales.length > 1)
            config.locales = config.locales.filter((key) => key !== code)
    }
</script>

{#snippet heading(title: string, icon: IconName, tone: string, description?: string)}
    <div class="section-heading">
        <ThemeIcon {icon} {tone} />
        <div>
            <h2>{title}</h2>
            {#if description}
                <p>{description}</p>
            {/if}
        </div>
    </div>
{/snippet}

<div class="sections">
    <div class="two-column intro">
        <section class="section">
            {@render heading(
                t('subpage-config-visual-editor.widget.branding'),
                'TbPalette',
                'cyan',
                t('subpage-config-visual-editor.widget.brand-appearance')
            )}
            <hr>
            <label class="field branded">
                <span
                    >{t('subpage-config-visual-editor.widget.brand-title')}
                    <span class="required">*</span></span
                >
                <input
                    class="gw-input"
                    name="brandTitle"
                    required
                    placeholder={t('subpage-config-visual-editor.widget.your-brand-name')}
                    bind:value={config.brandingSettings.title}
                >
            </label>
            <label class="field branded">
                <span>{t('common.field.logo-url')} <span class="required">*</span></span>
                <input
                    class="gw-input"
                    required
                    placeholder="https://example.com/logo.png"
                    bind:value={config.brandingSettings.logoUrl}
                >
            </label>
            <label class="field branded">
                <span
                    >{t('subpage-config-visual-editor.widget.support-url')}
                    <span class="required">*</span></span
                >
                <input
                    class="gw-input"
                    type="url"
                    required
                    placeholder="https://t.me/support"
                    bind:value={config.brandingSettings.supportUrl}
                >
            </label>
        </section>
        <section class="section">
            <div class="section-line">
                {@render heading(
                    t('subpage-config-visual-editor.widget.localization'),
                    'TbGlobe',
                    'teal',
                    t('subpage-config-visual-editor.widget.additional-languages')
                )}
                <span class="pill" data-tone="teal">{config.locales.length} active</span>
            </div>
            <hr>
            <div class="locale-scroll two-column">
                {#each languages as language (language.code)}
                    {@const enabled = config.locales.includes(language.code)}
                    <button
                        type="button"
                        class="mantine-UnstyledButton-root locale-card"
                        class:enabled
                        aria-pressed={enabled}
                        disabled={enabled && config.locales.length === 1}
                        onclick={() => setLocale(language.code, !enabled)}
                    >
                        <span class="emoji">{language.emoji}</span>
                        <span class="locale-name">
                            <strong>{language.name}</strong>
                            <span>{language.nativeName}</span>
                        </span>
                        <span class="pill code" data-tone={enabled ? 'teal' : 'gray'}
                            >{language.code}</span
                        >
                    </button>
                {/each}
            </div>
        </section>
    </div>

    <section class="section">
        {@render heading(
            t('base-settings-block.component.base-settings'),
            'TbHealthRecognition',
            'teal'
        )}
        <hr>
        <div class="two-column">
            <label class="field">
                <span
                    >{t('base-settings-block.component.meta-title')}
                    <span class="required">*</span></span
                >
                <input
                    class="gw-input"
                    name="metaTitle"
                    required
                    bind:value={config.baseSettings.metaTitle}
                >
            </label>
            <label class="field">
                <span
                    >{t('base-settings-block.component.meta-description')}
                    <span class="required">*</span></span
                >
                <input class="gw-input" required bind:value={config.baseSettings.metaDescription}>
            </label>
            <label class="option">
                <ThemeIcon icon="TbKey" />
                <span class="option-copy">
                    <strong>{t('base-settings-block.component.show-connection-keys')}</strong>
                    <span
                        >{t('base-settings-block.component.show-or-hide-raw-connection-keys')}</span
                    >
                </span>
                <input type="checkbox" bind:checked={config.baseSettings.showConnectionKeys}>
            </label>
            <label class="option">
                <ThemeIcon icon="TbLink" />
                <span class="option-copy">
                    <strong>{t('base-settings-block.component.hide-get-link-button')}</strong>
                    <span
                        >{t(
                            'base-settings-block.component.hide-the-get-link-button-top-right-corner'
                        )}</span
                    >
                </span>
                <input type="checkbox" bind:checked={config.baseSettings.hideGetLinkButton}>
            </label>
        </div>
    </section>

    <section class="section toolbar section-line">
        <div class="section-label">
            {@render heading(t('svg-library-modal.component.svg-library'), 'TbPhoto', 'violet')}
            <span class="pill" data-tone="violet"
                >{Object.keys(config.svgLibrary).length}
                icons</span
            >
        </div>
        <Button variant="soft" icon="TbPhoto" onclick={() => (overlay = 'icons')}>
            {t('subpage-config-visual-editor.widget.open-svg-library')}
        </Button>
    </section>

    <section class="section toolbar section-line">
        <div class="section-label">
            {@render heading('Base Translations', 'TbLanguage', 'violet')}
            <span
                class="pill"
                data-tone={translated === translationKeys.length ? 'green' : 'yellow'}
            >
                {translated}/{translationKeys.length}
            </span>
        </div>
        <Button variant="soft" icon="TbLanguage" onclick={() => (overlay = 'translations')}>
            Configure
        </Button>
    </section>

    <section class="section">
        {@render heading(
            t('subpage-config-visual-editor.widget.ui-configuration'),
            'TbPalette',
            'yellow'
        )}
        <hr>
        <div class="two-column">
            <SelectField
                label={t('subpage-config-visual-editor.widget.subscription-info-block-design')}
                placeholder=""
                options={SUBSCRIPTION_INFO_BLOCK_VARIANTS_VALUES.map((value) => ({
                    value,
                    label: value
                }))}
                value={config.uiConfig.subscriptionInfoBlockType}
                onchoose={(value) => {
                    const variant = SUBSCRIPTION_INFO_BLOCK_VARIANTS_VALUES.find(
                        (item) => item === value
                    )
                    if (variant) config.uiConfig.subscriptionInfoBlockType = variant
                }}
                searchable={false}
                clearable={false}
                required
            />
            <SelectField
                label={t('subpage-config-visual-editor.widget.installation-guides-design')}
                placeholder=""
                options={INSTALLATION_GUIDE_BLOCKS_VARIANTS_VALUES.map((value) => ({
                    value,
                    label: value
                }))}
                value={config.uiConfig.installationGuidesBlockType}
                onchoose={(value) => {
                    const variant = INSTALLATION_GUIDE_BLOCKS_VARIANTS_VALUES.find(
                        (item) => item === value
                    )
                    if (variant) config.uiConfig.installationGuidesBlockType = variant
                }}
                searchable={false}
                clearable={false}
                required
            />
        </div>
    </section>

    <section class="section">
        <div class="section-line">
            {@render heading(
                t('subpage-config-visual-editor.widget.platforms'),
                'TbDeviceDesktop',
                'violet',
                t('subpage-config-visual-editor.widget.configure-apps-for-each-platform')
            )}
            {#if available.length}
                <div class="add-platform">
                    <SelectField
                        label={t('subpage-config-visual-editor.widget.add-platform')}
                        placeholder={t('subpage-config-visual-editor.widget.add-platform')}
                        hideLabel
                        searchable={false}
                        clearable={false}
                        options={available.map((value) => ({
                            value,
                            label: platformLabels[value]
                        }))}
                        onchoose={(value) => {
                            const platform = available.find((item) => item === value)
                            if (platform) addPlatform(platform)
                        }}
                    />
                </div>
            {/if}
        </div>
        <hr>
        {#if available.length === platformKeys.length}
            <div class="platform-empty">
                <Icon name="TbDeviceDesktop" size={32} />
                <span>{t('subpage-config-visual-editor.widget.no-platforms-configured')}</span>
                <small
                    >{t('subpage-config-visual-editor.widget.add-a-platform-to-get-started')}</small
                >
            </div>
        {/if}
        {#each configured as key (key)}
            {@const platform = config.platforms[key]}
            {#if platform}
                <div class="platform-card">
                    <details ontoggle={(event) => (loaded[key] = event.currentTarget.open)}>
                        <summary>
                            <span class="platform-icon"
                                ><Icon name={platformIcons[key]} size={20} /></span
                            >
                            <span class="platform-title"
                                ><strong>{platformLabels[key]}</strong
                                ><small>{platform.apps.length} app(s) configured</small></span
                            >
                            <span class="chevron"><Icon name="TbChevronDown" size={18} /></span>
                        </summary>
                        {#if loaded[key]}
                            <div class="fields">
                                <PlatformFields
                                    bind:platform={
                                        () => platform,
                                        (value) => (config.platforms[key] = value)
                                    }
                                    locales={config.locales}
                                    library={config.svgLibrary}
                                />
                            </div>
                        {/if}
                    </details>
                    <div class="platform-remove">
                        <ActionIcon
                            icon="TbTrash"
                            tone="red"
                            subtle
                            class="platform-remove-action"
                            size={34}
                            iconSize={18}
                            label={`${t('common.action.remove')} ${platformLabels[key]}`}
                            onclick={() => removePlatform(key)}
                        />
                    </div>
                </div>
            {/if}
        {/each}
    </section>
</div>

{#if overlay === 'icons'}
    <Overlay
        title={t('svg-library-modal.component.svg-library')}
        icon="TbPhoto"
        tone="violet"
        width={620}
        fullscreenOnMobile={false}
        close={() => (overlay = undefined)}
    >
        <Icons bind:library={config.svgLibrary} />
    </Overlay>
{:else if overlay === 'translations'}
    <Overlay
        title={t('base-translations-drawer.component.base-translations')}
        icon="TbLanguage"
        tone="violet"
        drawer
        close={() => (overlay = undefined)}
    >
        <div class="fields">
            {#each translationKeys as key (key)}
                <LocalizedFields
                    label={BASE_TRANSLATION_LABELS[key]}
                    locales={config.locales}
                    bind:value={config.baseTranslations[key]}
                />
            {/each}
        </div>
    </Overlay>
{/if}

<style>
    .sections,
    .two-column,
    .section,
    .field,
    .fields {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-md);
    }
    .sections,
    .intro {
        gap: var(--mantine-spacing-lg);
    }
    .two-column {
        grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    .section {
        align-content: start;
        padding: var(--mantine-spacing-lg);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-lg);
        transition: border-color 0.3s ease;
    }
    .two-column.intro > .section,
    .two-column.intro + .section {
        padding-block-end: calc(var(--mantine-spacing-lg) - 1px);
    }
    .section:hover {
        border-color: rgb(34 211 238 / 25%);
    }
    .section-line,
    .section-label,
    .section-heading {
        display: flex;
        align-items: center;
        min-width: 0;
        gap: var(--mantine-spacing-xs);
    }
    .section-line {
        justify-content: space-between;
        flex-wrap: wrap;
    }
    .section-heading h2 {
        margin: 0;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h5-font-size);
        line-height: var(--mantine-h5-line-height);
    }
    .section-heading p {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .section-line > .section-heading,
    .section-line > .section-label {
        flex: 1 1 max-content;
    }
    hr {
        width: 100%;
        margin: 0;
        border: 0;
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    .field {
        gap: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .branded > span:first-child {
        min-height: calc(var(--mantine-font-size-md) * var(--mantine-line-height-md));
    }
    .field input {
        width: 100%;
    }
    .branded input,
    .two-column :global(.select-field input),
    .add-platform {
        background: var(--gw-raised-bg);
        border-color: rgba(var(--gw-fg-rgb), 0.1);
    }
    .required {
        color: var(--mantine-color-error);
    }
    .locale-scroll {
        max-height: 200px;
        padding-inline: var(--mantine-spacing-md);
        overflow: auto;
        scrollbar-color: var(--mantine-color-default-border) transparent;
        scrollbar-width: thin;
        gap: var(--mantine-spacing-sm);
    }
    .locale-card {
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        border-radius: var(--mantine-radius-md);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md);
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .locale-card:hover {
        background: rgba(var(--gw-fg-rgb), 0.04);
        border-color: rgba(var(--gw-fg-rgb), 0.12);
    }
    .locale-card.enabled {
        background: rgb(18 184 134 / 8%);
        border-color: rgb(18 184 134 / 25%);
    }
    .locale-card.enabled:hover {
        background: rgb(18 184 134 / 12%);
        border-color: rgb(18 184 134 / 35%);
    }
    .locale-card:disabled {
        cursor: not-allowed;
        opacity: 0.8;
    }
    .locale-name {
        display: grid;
        flex: 1;
        min-width: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .locale-name strong {
        color: var(--locale-label-color, var(--mantine-color-dimmed));
        font-weight: 500;
    }
    .locale-card.enabled {
        --locale-label-color: var(--mantine-color-bright);
    }
    .locale-name > * {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .emoji {
        font-size: var(--mantine-font-size-lg);
        line-height: 1;
    }
    .pill {
        background: var(--badge-shade);
        color: var(--badge-text);
        border-radius: 100px;
        font-size: 10px;
        font-weight: 700;
        line-height: 18px;
        letter-spacing: 0.25px;
        padding-inline: 8px;
        text-transform: uppercase;
        white-space: nowrap;
    }
    .pill[data-tone="teal"] {
        --badge-shade: var(--mantine-color-teal-light);
        --badge-text: var(--mantine-color-teal-light-color);
    }
    .pill[data-tone="violet"] {
        --badge-shade: var(--mantine-color-violet-light);
        --badge-text: var(--mantine-color-violet-light-color);
    }
    .pill[data-tone="green"] {
        --badge-shade: var(--mantine-color-green-light);
        --badge-text: var(--mantine-color-green-light-color);
    }
    .pill[data-tone="yellow"] {
        --badge-shade: var(--mantine-color-yellow-light);
        --badge-text: var(--mantine-color-yellow-light-color);
    }
    .pill.code {
        line-height: 16px;
        padding-inline: 6px;
        font-size: 9px;
    }
    .code[data-tone="gray"] {
        --badge-shade: var(--mantine-color-gray-light);
        --badge-text: var(--mantine-color-gray-light-color);
    }
    .code[data-tone="teal"] {
        --badge-shade: var(--mantine-color-teal-filled);
        --badge-text: white;
    }
    .option {
        --gw-checkbox-size: 24px;
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-md);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        cursor: pointer;
    }
    .option:has(input:checked) {
        border-color: var(--mantine-color-cyan-8);
    }
    .option-copy {
        display: grid;
        gap: 2px;
        margin-inline-end: auto;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .option strong {
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }
    .option input:focus-visible {
        outline: 2px solid var(--mantine-color-cyan-4);
        outline-offset: 2px;
    }
    .section > .platform-card:last-child {
        margin-block-end: 12px;
    }
    .toolbar {
        display: flex;
        padding: var(--mantine-spacing-md);
    }
    .add-platform {
        width: min(180px, 100%);
    }
    .platform-empty {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-xl) var(--mantine-spacing-md);
        color: var(--mantine-color-dimmed);
    }
    details {
        border: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        border-radius: var(--mantine-radius-md);
        background: rgba(var(--gw-fg-rgb), 0.01);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md);
    }
    summary {
        padding-inline-end: 38px;
        padding-block: 12px;
        min-height: 38px;
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        cursor: pointer;
    }
    summary::-webkit-details-marker {
        display: none;
    }
    summary small {
        color: var(--mantine-color-dimmed);
    }
    .platform-icon,
    .chevron {
        display: grid;
        place-items: center;
        flex: none;
    }
    .platform-icon {
        width: 34px;
        height: 34px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid color-mix(in srgb, var(--mantine-color-violet-6) 30%, transparent);
        background: transparent;
        color: var(--mantine-color-violet-4);
    }
    details[open] > summary .chevron {
        rotate: 180deg;
    }
    details[open] > summary {
        padding-bottom: var(--mantine-spacing-sm);
        border-bottom: 1px solid var(--gw-border);
    }
    .platform-card {
        position: relative;
    }
    .platform-remove {
        position: absolute;
        inset-block-start: 28.4px;
        inset-inline-end: 13px;
    }
    .platform-title {
        display: grid;
        flex: 1;
        gap: 0;
    }

    .platform-title strong {
        color: var(--mantine-color-bright);
        font-weight: 600;
        line-height: 24px;
    }

    :global(.action-icon.platform-remove-action.tinted) {
        color: var(--mantine-color-red-3);
    }

    .platform-title small {
        font-size: var(--mantine-font-size-xs);
        line-height: 16.8px;
    }
    .fields {
        margin-block-start: var(--mantine-spacing-sm);
        justify-items: stretch;
    }
    @media (max-width: 62em) {
        .two-column {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
