<script lang="ts">
    import { untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { CustomRemarks } from './settings-fields'
    import TemplateVariables from './TemplateVariables.svelte'

    const categories = [
        {
            field: 'HWIDMaxDevicesExceeded',
            label: 'subscription-user-remarks-card.widget.hwid-max-devices-exceeded',
            icon: 'TbDevices2',
            color: 'red'
        },
        {
            field: 'HWIDNotSupported',
            label: 'subscription-user-remarks-card.widget.hwid-not-supported',
            icon: 'TbX',
            color: 'red'
        },
        { field: 'expiredUsers', label: 'EXPIRED', icon: 'PiClockUser', color: 'red' },
        { field: 'limitedUsers', label: 'LIMITED', icon: 'PiClockCountdown', color: 'orange' },
        { field: 'disabledUsers', label: 'DISABLED', icon: 'PiProhibit', color: 'gray' },
        {
            field: 'emptyHosts',
            label: 'subscription-user-remarks-card.widget.empty-hosts',
            icon: 'PiListChecks',
            color: 'blue'
        }
    ] as const

    let { remarks }: { remarks: CustomRemarks | null } = $props()
    let t = $derived(locale.t)
    let nextId = 0
    const groups = $state(
        untrack(() =>
            categories.map((category) => {
                const values = remarks?.[category.field]
                return {
                    ...category,
                    height: 0,
                    rows: (values?.length ? values : ['']).map((value) => ({ id: nextId++, value }))
                }
            })
        )
    )
</script>

<div class="remark-groups">
    {#each groups as group (group.field)}
        <div style:grid-row-end={`span ${Math.max(1, group.height)}`}>
            <div class="remark-measure" bind:offsetHeight={group.height}>
                <section>
                    <div class="heading">
                        <h3>
                            <span
                                class="remark-icon"
                                style:--icon-background={`var(--mantine-color-${group.color}-light)`}
                                style:--icon-color={`var(--mantine-color-${group.color}-light-color)`}
                                ><Icon name={group.icon} size={24} /></span
                            >
                            <span>
                                {group.label === 'EXPIRED' ||
                                group.label === 'LIMITED' ||
                                group.label === 'DISABLED'
                                    ? `${t('common.field.user-status')}: ${group.label}`
                                    : t(group.label)}
                            </span>
                        </h3>
                        <Button
                            size="sm"
                            variant="light"
                            icon="TbPlus"
                            onclick={() => group.rows.push({ id: nextId++, value: '' })}
                        >
                            {t('common.action.add')}
                        </Button>
                    </div>
                    <div class="remark-rows">
                        {#each group.rows as row (row.id)}
                            <div class="remark">
                                <div class="remark-input">
                                    <span><TemplateVariables /></span>
                                    <input
                                        name={group.field}
                                        bind:value={row.value}
                                        aria-label={t('remarks-manager.widget.enter-remark')}
                                        placeholder={t('remarks-manager.widget.enter-remark')}
                                        required
                                    >
                                </div>
                                <ActionIcon
                                    icon="PiTrashDuotone"
                                    label={t('common.action.delete')}
                                    tone="red"
                                    size={34}
                                    iconSize={16}
                                    disabled={group.rows.length === 1}
                                    onclick={() =>
                                        (group.rows = group.rows.filter(({ id }) => id !== row.id))}
                                />
                            </div>
                        {/each}
                    </div>
                </section>
            </div>
        </div>
    {/each}
</div>

<style>
    .remark-groups {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        grid-auto-rows: 1px;
        column-gap: var(--mantine-spacing-md);
        margin-inline: calc(-1 * var(--mantine-spacing-md));
        padding: var(--mantine-spacing-lg);
    }
    .remark-measure {
        padding-bottom: var(--mantine-spacing-md);
    }
    @media (min-width: 1400px) {
        .remark-groups {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 2000px) {
        .remark-groups {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }
    }
    @media (min-width: 3000px) {
        .remark-groups {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
    section {
        display: grid;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        box-shadow: var(--mantine-shadow-xl);
    }
    .heading,
    h3 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .heading {
        justify-content: space-between;
        padding-bottom: var(--mantine-spacing-md);
        border-bottom: 1px solid var(--mantine-color-default-border);
    }
    h3 {
        margin: 0;
        font-size: var(--mantine-font-size-md);
        font-weight: 600;
    }
    .remark-icon {
        flex: none;
        display: grid;
        place-items: center;
        width: 34px;
        height: 34px;
        border-radius: var(--mantine-radius-md);
        background: var(--icon-background);
        color: var(--icon-color);
    }
    .remark-rows {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    .remark {
        display: flex;
        align-items: flex-start;
        gap: var(--mantine-spacing-sm);
    }
    .remark-input {
        position: relative;
        flex: 1;
        min-width: 0;
    }
    .remark-input > span {
        position: absolute;
        inset-inline-start: 3px;
        top: 50%;
        transform: translateY(-50%);
    }
    .remark-input input {
        width: 100%;
        min-height: 36px;
        border-radius: var(--mantine-radius-md);
        padding-inline-start: 36px;
        font-size: var(--mantine-font-size-sm);
    }
</style>
