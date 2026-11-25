<script lang="ts">
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import TablePopup from '../ui/TablePopup.svelte'
    import type { UserTableTemplate } from './table-state'

    let {
        templates,
        onsave,
        onapply,
        onremove
    }: {
        templates: UserTableTemplate[]
        onsave: (name: string, color: string) => void
        onapply: (snapshot: UserTableTemplate['snapshot']) => void
        onremove: (id: string) => void
    } = $props()
    const colors = [
        'violet',
        'blue',
        'teal',
        'green',
        'lime',
        'yellow',
        'orange',
        'red',
        'pink',
        'grape',
        'cyan'
    ]
    const menuId = $props.id()
    let anchor = $state<HTMLElement>()
    let saving = $state(false)
    let name = $state('')
    let color = $state('violet')
    let t = $derived(locale.t)

    function swatch(value: string): string {
        if (/^#[\da-f]{6}$/i.test(value)) return value
        return `var(--mantine-color-${colors.includes(value) ? value : 'violet'}-6)`
    }

    function closeMenu(): void {
        anchor?.focus({ preventScroll: true })
        anchor = undefined
    }

    function save(event: SubmitEvent): void {
        event.preventDefault()
        const trimmed = name.trim()
        if (!trimmed) return
        onsave(trimmed, color)
        saving = false
    }
</script>

<ActionIcon
    icon="TbBookmark"
    tone="indigo"
    label={t('constants.templates')}
    aria-haspopup="menu"
    aria-expanded={Boolean(anchor)}
    aria-controls={anchor ? menuId : undefined}
    onclick={(event) => {
        anchor = anchor ? undefined : event.currentTarget
    }}
/>

{#if anchor}
    <TablePopup
        {anchor}
        id={menuId}
        label={t('constants.templates')}
        width={260}
        alignEnd
        arrow
        close={() => (anchor = undefined)}
    >
        {#each templates as template (template.id)}
            <div class="template-row" role="presentation">
                <button
                    type="button"
                    role="menuitem"
                    class="template-option m_99ac2aa1 mantine-UnstyledButton-root"
                    onclick={() => {
                        closeMenu()
                        onapply(template.snapshot)
                    }}
                >
                    <span
                        class="swatch m_de3d2490"
                        style:--cs-size="14px"
                        style:--swatch={swatch(template.color)}
                        aria-hidden="true"
                    ></span>
                    <span class="template-name">{template.name}</span>
                </button>
                <ActionIcon
                    icon="TbTrash"
                    label={`${t('common.action.delete')} ${template.name}`}
                    tone="red"
                    size={22}
                    iconSize={14}
                    subtle
                    role="menuitem"
                    onclick={(event) => {
                        const row = event.currentTarget.parentElement
                        const nextItem = row?.nextElementSibling?.querySelector('button')
                        onremove(template.id)
                        if (nextItem instanceof HTMLButtonElement) nextItem.focus()
                        else document.getElementById(`${menuId}-save`)?.focus()
                    }}
                />
            </div>
        {/each}
        {#if templates.length}
            <hr class="template-divider m_efdf90cb">
        {/if}
        <button
            id={`${menuId}-save`}
            type="button"
            role="menuitem"
            class="template-option m_99ac2aa1 mantine-UnstyledButton-root"
            onclick={() => {
                closeMenu()
                name = ''
                color = 'violet'
                saving = true
            }}
        >
            <Icon name="TbPlus" size={14} />
            {t('common.action.save')}
        </button>
    </TablePopup>
{/if}

{#if saving}
    <Overlay
        title={t('constants.templates')}
        icon="TbBookmark"
        width={440}
        fullscreenOnMobile={false}
        class="save-user-view"
        close={() => (saving = false)}
    >
        <form class="template-form" onsubmit={save}>
            <label class="template-field">
                <span class="m_8fdc1311">{t('header-action-buttons.feature.template-name')}</span>
                <input
                    class="mantine-Input-input"
                    bind:value={name}
                    placeholder="ACTIVE users with sort"
                    maxlength="40"
                    data-autofocus
                    required
                >
            </label>
            <fieldset>
                <legend>{t('common.field.color')}</legend>
                <div class="swatches">
                    {#each colors as option (option)}
                        <button
                            type="button"
                            class="swatch m_de3d2490 mantine-UnstyledButton-root"
                            style:--swatch={swatch(option)}
                            aria-label={option}
                            aria-pressed={color === option}
                            onclick={() => (color = option)}
                        >
                            {#if color === option}
                                <Icon name="TbCheck" size={14} />
                            {/if}
                        </button>
                    {/each}
                </div>
            </fieldset>
            <Button
                type="submit"
                variant="soft"
                {color}
                icon="TbDeviceFloppy"
                block
                disabled={!name.trim()}
            >
                {t('common.action.save')}
            </Button>
        </form>
    </Overlay>
{/if}

<style>
    .template-row {
        position: relative;
        border-radius: var(--mantine-radius-md);
    }

    .template-row:hover {
        background: var(--mantine-color-default-hover);
    }

    .template-option {
        gap: var(--mantine-spacing-xs);
    }

    .template-row .template-option {
        padding-inline-end: 38px;
    }

    .template-name {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .template-row :global(.action-icon) {
        position: absolute;
        inset-block-start: 50%;
        inset-inline-end: 12px;
        transform: translateY(-50%);
    }

    .template-divider {
        border-inline: 0;
        border-block-end: 0;
    }

    .template-form {
        display: grid;
        gap: var(--mantine-spacing-md);
    }

    .template-field {
        display: grid;
        gap: 3px;
    }

    .template-field input {
        width: 100%;
        height: 36px;
        padding-inline: var(--mantine-spacing-xs);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        outline: none;
        background: var(--mantine-color-default);
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
    }

    .template-field input:focus {
        border-color: var(--mantine-primary-color-filled);
    }

    fieldset {
        min-width: 0;
        margin: 0;
        padding: 0;
        border: 0;
    }

    legend {
        padding: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: var(--mantine-font-weight-medium);
    }

    .swatches {
        display: flex;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
        margin-block-start: var(--mantine-spacing-xs);
    }

    .swatch {
        display: inline-grid;
        flex: none;
        place-items: center;
        background: var(--swatch);
        color: white;
        box-shadow:
            inset 0 0 0 1px rgb(0 0 0 / 10%),
            inset 0 0 4px rgb(0 0 0 / 15%);
    }
</style>
