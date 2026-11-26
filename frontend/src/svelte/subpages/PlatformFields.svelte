<script lang="ts">
    import {
        BUTTON_TYPES_VALUES,
        SUBSCRIPTION_PAGE_TEMPLATE_KEYS
    } from '@remnawave/subscription-page-types'
    import { moveItem } from '@shared/lib/move-selection'
    import { locale } from '../i18n.svelte'
    import TemplateVariables from '../subscriptions/TemplateVariables.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Switch from '../ui/Switch.svelte'
    import { localized, type PageConfig, type Platform } from './config'
    import IconKey from './IconKey.svelte'
    import LocalizedFields from './LocalizedFields.svelte'

    let {
        platform = $bindable(),
        locales,
        library
    }: {
        platform: Platform
        locales: PageConfig['locales']
        library: Record<string, string>
    } = $props()

    type App = Platform['apps'][number]
    type Block = App['blocks'][number]
    let editing = $state<number>()
    let editingBlock = $state<Block>()
    let colorHelp = $state(false)
    let t = $derived(locale.t)

    const buttonTypes = BUTTON_TYPES_VALUES.map((value) => ({ value, label: value }))

    function addBlock(app: App): void {
        app.blocks.push({
            title: localized(locales, ''),
            description: localized(locales, ''),
            svgIconKey: '',
            svgIconColor: 'cyan',
            buttons: []
        })
    }

    function addButton(block: Block): void {
        block.buttons.push({
            text: localized(locales, ''),
            link: '',
            type: 'external',
            svgIconKey: ''
        })
    }
</script>

<div class="platform-settings">
    <IconKey
        label={t('platform-editor.component.platform-svg-icon')}
        {library}
        bind:value={platform.svgIconKey}
    />
    <LocalizedFields
        label={t('platform-editor.component.display-name')}
        {locales}
        bind:value={platform.displayName}
    />
</div>
<div class="apps-heading">
    <strong>{t('platform-editor.component.apps')}</strong>
</div>
<div class="apps">
    {#each platform.apps as app, appIndex (app)}
        <div class="app-card">
            <button
                type="button"
                class="app-edit mantine-UnstyledButton-root"
                onclick={() => (editing = appIndex)}
            >
                <span class="app-icon">
                    {#if app.svgIconKey && library[app.svgIconKey]}
                        <img
                            width="20"
                            height="20"
                            alt=""
                            src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(library[app.svgIconKey] ?? '')}`}
                        >
                    {:else}
                        <Icon name="TbPhoto" size={18} />
                    {/if}
                </span>
                <strong>{app.name || `App ${appIndex + 1}`}</strong>
                {#if app.featured}
                    <span class="star"><Icon name="TbStar" size={16} /></span>
                {/if}
            </button>
            <span class="blocks">{app.blocks.length} blocks</span>
            <ActionIcon
                icon="PiArrowUpDuotone"
                label="Move up"
                subtle
                size={24}
                iconSize={18}
                disabled={appIndex === 0}
                onclick={() => moveItem(platform.apps, appIndex, -1)}
            />
            <ActionIcon
                icon="PiArrowDownDuotone"
                label="Move down"
                subtle
                size={24}
                iconSize={18}
                disabled={appIndex + 1 === platform.apps.length}
                onclick={() => moveItem(platform.apps, appIndex, 1)}
            />
            <ActionIcon
                icon="TbCopy"
                label={t('common.action.clone')}
                subtle
                size={24}
                iconSize={18}
                onclick={() =>
                    platform.apps.splice(appIndex + 1, 0, {
                        ...$state.snapshot(app),
                        name: `${app.name} (copy)`
                    })}
            />
            <ActionIcon
                icon="TbTrash"
                label={t('common.action.remove')}
                subtle
                tone="red"
                size={24}
                iconSize={18}
                onclick={() => platform.apps.splice(appIndex, 1)}
            />
        </div>
    {/each}
    <Button
        size="sm"
        variant="default"
        class="add-app"
        icon="TbPlus"
        onclick={() => {
            platform.apps.push({ name: '', featured: false, blocks: [] })
            editing = platform.apps.length - 1
        }}
    >
        {t('platform-editor.component.add-app')}
    </Button>
</div>
{#if editing !== undefined}
    {@const appIndex = editing}
    {@const app = platform.apps[appIndex]}
    {#if app}
        <Overlay
            class="app-editor-overlay"
            title={t('platform-editor.component.edit-app')}
            subtitle={app.name || t('platform-editor.component.unnamed-app')}
            icon="TbEdit"
            tone="cyan"
            drawer
            close={() => {
                editingBlock = undefined
                editing = undefined
            }}
        >
            <div class="app-editor">
                <section class="section-card">
                    <div class="section-stack">
                        <div class="settings-heading">
                            <div class="heading-copy">
                                <span class="heading-icon cyan"
                                    ><Icon name="TbStar" size={16} /></span
                                >
                                <strong
                                    >{t('app-editor-drawer-content.component.app-settings')}</strong
                                >
                            </div>
                            <Switch
                                label={t('app-editor-drawer-content.component.featured')}
                                showLabel
                                switchSize="sm"
                                bind:checked={app.featured}
                            />
                        </div>
                        <label class="app-name-field">
                            <span>{t('app-editor-drawer-content.component.app-name')}</span>
                            <input class="mantine-Input-input input-dark" bind:value={app.name}>
                        </label>
                        <IconKey {library} optional bind:value={app.svgIconKey} />
                    </div>
                </section>

                <section class="section-card">
                    <div class="section-stack">
                        <div class="section-heading">
                            <span class="heading-icon violet"
                                ><Icon name="TbChevronRight" size={16} /></span
                            >
                            <div class="section-title">
                                <strong>{t('app-editor-drawer-content.component.blocks')}</strong>
                                <span>{app.blocks.length} blocks</span>
                            </div>
                        </div>
                        <div class="block-list">
                            {#each app.blocks as block, blockIndex (block)}
                                <div class="block-row">
                                    <button
                                        type="button"
                                        class="block-edit mantine-UnstyledButton-root"
                                        onclick={() => (editingBlock = block)}
                                    >
                                        {block.title[locales[0] ?? 'en'] ||
                                            `Block ${blockIndex + 1}`}
                                    </button>
                                    <span class="block-badge">{block.buttons.length} buttons</span>
                                    <ActionIcon
                                        icon="TbArrowUp"
                                        label="Move block up"
                                        subtle
                                        size={22}
                                        iconSize={14}
                                        disabled={blockIndex === 0}
                                        onclick={() => moveItem(app.blocks, blockIndex, -1)}
                                    />
                                    <ActionIcon
                                        icon="TbArrowDown"
                                        label="Move block down"
                                        subtle
                                        size={22}
                                        iconSize={14}
                                        disabled={blockIndex + 1 === app.blocks.length}
                                        onclick={() => moveItem(app.blocks, blockIndex, 1)}
                                    />
                                    <ActionIcon
                                        icon="TbTrash"
                                        label={t('common.action.remove')}
                                        subtle
                                        tone="red"
                                        size={22}
                                        iconSize={14}
                                        onclick={() => app.blocks.splice(blockIndex, 1)}
                                    />
                                </div>
                            {/each}
                            <Button
                                size="xs"
                                variant="default"
                                icon="TbPlus"
                                block
                                class="add-block"
                                onclick={() => addBlock(app)}
                            >
                                {t('app-editor-drawer-content.component.add-block')}
                            </Button>
                        </div>
                    </div>
                </section>
            </div>
            {#if editingBlock}
                {@const block = editingBlock}
                <Overlay
                    title={block.title[locales[0] ?? 'en'] || 'Block'}
                    icon="TbEdit"
                    close={() => {
                        editingBlock = undefined
                        colorHelp = false
                    }}
                >
                    <div class="fields">
                        <h3>{t('block-editor.modal.component.block-settings')}</h3>
                        <IconKey {library} bind:value={block.svgIconKey} />
                        <div class="input-with-help">
                            <label>
                                {t('block-editor.modal.component.icon-color')}
                                <input class="gw-input" bind:value={block.svgIconColor}>
                            </label>
                            <ActionIcon
                                icon="TbInfoSquare"
                                label={t('block-editor.modal.component.icon-color')}
                                subtle
                                tone="lime"
                                size={30}
                                iconSize={18}
                                aria-expanded={colorHelp}
                                onclick={() => (colorHelp = !colorHelp)}
                            />
                        </div>
                        {#if colorHelp}
                            <p class="color-help">
                                {t('block-editor.modal.component.icon-color-help', {
                                    colors: 'blue, cyan, dark, grape, gray, green, indigo, lime, orange, pink, red, teal, violet, yellow'
                                })}
                            </p>
                        {/if}
                        <LocalizedFields
                            label={t('block-editor.modal.component.title')}
                            {locales}
                            bind:value={block.title}
                        />
                        <LocalizedFields
                            label={t('common.field.description')}
                            {locales}
                            multiline
                            bind:value={block.description}
                        />
                        <h3>{t('block-editor.modal.component.buttons')}</h3>
                        {#each block.buttons as button, buttonIndex (button)}
                            <div class="button-card">
                                <details>
                                    <summary>
                                        <Icon name="TbChevronRight" size={16} />
                                        <span
                                            >Button {buttonIndex + 1}:
                                            {button.text[locales[0] ?? 'en'] ||
                                                t('common.message.not-set')}</span
                                        >
                                        <span class="blocks">{button.type}</span>
                                    </summary>
                                    <div class="fields">
                                        <SelectField
                                            label={t('button-editor.component.type')}
                                            leading="TbExternalLink"
                                            placeholder=""
                                            options={buttonTypes}
                                            value={button.type}
                                            searchable={false}
                                            clearable={false}
                                            onchoose={(value) => {
                                                const type = BUTTON_TYPES_VALUES.find(
                                                    (item) => item === value
                                                )
                                                if (type) button.type = type
                                            }}
                                        />
                                        <div class="input-with-help">
                                            <label>
                                                {t('button-editor.component.link')}
                                                <input
                                                    class="gw-input"
                                                    placeholder="https:// or app://"
                                                    bind:value={button.link}
                                                >
                                            </label>
                                            <TemplateVariables
                                                keys={SUBSCRIPTION_PAGE_TEMPLATE_KEYS}
                                            />
                                        </div>
                                        <IconKey {library} bind:value={button.svgIconKey} />
                                        <LocalizedFields
                                            label={t('button-editor.component.button-text')}
                                            {locales}
                                            bind:value={button.text}
                                        />
                                    </div>
                                </details>
                                <div class="button-actions">
                                    <ActionIcon
                                        icon="PiArrowUpDuotone"
                                        label="Move button up"
                                        subtle
                                        size={24}
                                        iconSize={14}
                                        disabled={buttonIndex === 0}
                                        onclick={() => moveItem(block.buttons, buttonIndex, -1)}
                                    />
                                    <ActionIcon
                                        icon="PiArrowDownDuotone"
                                        label="Move button down"
                                        subtle
                                        size={24}
                                        iconSize={14}
                                        disabled={buttonIndex + 1 === block.buttons.length}
                                        onclick={() => moveItem(block.buttons, buttonIndex, 1)}
                                    />
                                    <ActionIcon
                                        icon="TbTrash"
                                        label={t('common.action.remove')}
                                        subtle
                                        tone="red"
                                        size={24}
                                        iconSize={14}
                                        onclick={() => block.buttons.splice(buttonIndex, 1)}
                                    />
                                </div>
                            </div>
                        {/each}
                        <Button
                            size="xs"
                            variant="default"
                            icon="TbPlus"
                            class="add-block"
                            onclick={() => addButton(block)}
                            >{t('block-editor.modal.component.add-button')}</Button
                        >
                    </div>
                </Overlay>
            {/if}
        </Overlay>
    {/if}
{/if}

<style>
    :global(.app-editor-overlay .overlay-title h2) {
        font-size: var(--mantine-h5-font-size);
        font-weight: var(--mantine-h5-font-weight);
        line-height: var(--mantine-h5-line-height);
    }

    .input-with-help {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        align-items: end;
        gap: 4px;
    }
    .color-help {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
    }

    .platform-settings,
    .apps {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: var(--mantine-spacing-sm);
    }
    .apps-heading,
    .app-card {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
    }
    .apps :global(.add-app) {
        border-style: dashed;
    }
    .apps-heading {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .apps-heading strong {
        font-weight: 400;
    }
    .apps-heading::after {
        content: "";
        flex: 1;
        border-top: 1px solid var(--gw-border);
    }
    .app-card {
        gap: 4px;
        background: rgba(0, 0, 0, 0.13);
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        padding: var(--mantine-spacing-sm);
    }
    .app-icon {
        display: grid;
        place-items: center;
        width: 28px;
        height: 28px;
        flex: none;
        color: var(--mantine-color-violet-light-color);
        background: var(--mantine-color-violet-light);
        border-radius: var(--mantine-radius-md);
    }
    .app-edit {
        display: flex;
        flex: 1 0 30%;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        min-width: 0;
        color: var(--mantine-color-bright);
        font: inherit;
        font-size: var(--mantine-font-size-sm);
    }
    .app-edit strong {
        font-weight: 600;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }
    .star {
        color: var(--mantine-color-yellow-5);
    }
    .blocks {
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: var(--mantine-color-violet-light-color);
        background: var(--mantine-color-violet-light);
        border-radius: 99px;
        padding: 2px 6px;
        font-weight: 700;
        font-size: 10px;
        text-transform: uppercase;
    }
    @media (max-width: 40em) {
        .app-icon {
            display: none;
        }
    }
    .fields,
    label {
        display: grid;
        gap: 0.5rem;
    }

    .app-editor {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .section-card {
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .section-stack,
    .block-list {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .settings-heading,
    .heading-copy,
    .section-heading,
    .section-title {
        display: flex;
        align-items: center;
    }
    .settings-heading {
        justify-content: space-between;
        gap: var(--mantine-spacing-sm);
    }
    .section-heading {
        justify-content: flex-start;
        gap: var(--mantine-spacing-sm);
    }
    .heading-copy,
    .section-heading {
        min-width: 0;
        gap: var(--mantine-spacing-sm);
    }
    .heading-copy strong,
    .section-title strong {
        color: var(--mantine-color-bright);
        font-size: var(--mantine-font-size-md);
        font-weight: 700;
        line-height: var(--mantine-line-height);
    }
    .heading-icon {
        display: grid;
        width: 34px;
        height: 34px;
        flex: none;
        place-items: center;
        border: 1px solid color-mix(in srgb, currentColor 30%, transparent);
        border-radius: var(--mantine-radius-md);
    }
    .heading-icon.cyan {
        color: var(--mantine-color-cyan-4);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--mantine-color-cyan-6) 15%, transparent),
            color-mix(in srgb, var(--mantine-color-cyan-7) 10%, transparent)
        );
    }
    .heading-icon.violet {
        color: var(--mantine-color-violet-4);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--mantine-color-violet-6) 15%, transparent),
            color-mix(in srgb, var(--mantine-color-violet-7) 10%, transparent)
        );
    }
    .section-title {
        display: grid;
        min-width: 0;
        gap: 0;
    }
    .section-title span {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .section-title strong {
        line-height: var(--mantine-h5-line-height);
    }
    .app-name-field {
        gap: 4.5px;
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }
    .app-name-field .input-dark {
        height: 36px;
        padding-inline: var(--mantine-spacing-sm);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.1);
        color: var(--mantine-color-bright);
        font-size: var(--mantine-font-size-sm);
    }
    .block-list {
        gap: var(--mantine-spacing-xs);
    }
    .block-row {
        display: flex;
        align-items: center;
        gap: 4px;
        min-width: 0;
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.028);
        border-radius: var(--mantine-radius-md);
        background: rgba(0, 0, 0, 0.13);
        transition: all 0.3s ease;
    }
    .block-row:hover {
        border-color: rgba(34, 211, 238, 0.12);
        background: rgba(34, 211, 238, 0.04);
    }
    .block-row :global(.action-icon:last-child) {
        opacity: 0.6;
        transition: opacity 0.2s ease;
    }
    .block-row :global(.action-icon:last-child:hover) {
        opacity: 1;
    }
    .block-edit {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        color: var(--mantine-color-bright);
        font: inherit;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        text-align: start;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .block-badge {
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
        border-radius: var(--mantine-radius-md);
        padding: 2px 8px;
        font-size: 9px;
        font-weight: 700;
        line-height: 1.33;
        text-transform: uppercase;
        white-space: nowrap;
    }
    .block-list :global(.add-block) {
        border-style: dashed;
    }
    @media (max-width: 48em) {
        .block-badge {
            display: none;
        }
    }
    .fields :global(.add-block) {
        border-style: dashed;
    }
    h3 {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
    }
    .button-card {
        position: relative;
    }
    .button-actions {
        position: absolute;
        inset-block-start: 10px;
        inset-inline-end: 10px;
        display: flex;
    }

    details {
        padding: 0.625rem;
        border: 1px solid var(--gw-border);
        border-radius: 0.375rem;
    }
    summary {
        display: flex;
        align-items: center;
        gap: 6px;
        min-height: 24px;
        padding-inline-end: 76px;
        cursor: pointer;
        font-size: var(--mantine-font-size-sm);
    }
    summary span:first-of-type {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    details[open] summary :global(svg) {
        rotate: 90deg;
    }

    .fields {
        margin-block-start: 0.75rem;
    }
</style>
