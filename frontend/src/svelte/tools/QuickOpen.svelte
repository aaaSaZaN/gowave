<script lang="ts">
    import { buildOpenEntityUrl, OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
    import { locale } from '../i18n.svelte'
    import { navigate } from '../router'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import { type QuickTarget, targets, validEntityId } from './quick-open'

    let t = $derived(locale.t)
    const icons = {
        user: 'PiUsers',
        host: 'PiListChecks',
        node: 'HiServer',
        'config-profile': 'XrayLogo',
        'internal-squad': 'TbCirclesRelation',
        'external-squad': 'TbWebhook',
        'node-plugin': 'TbPackage',
        'subpage-config': 'PiArrowsInCardinalFill'
    } as const
    let values = $state<Partial<Record<QuickTarget, string>>>({})
    let notice = $state('')
    let failure = $state('')

    function copy(entity: QuickTarget, id: string): void {
        notice = ''
        failure = ''
        void navigator.clipboard.writeText(buildOpenEntityUrl(entity, id)).then(
            () => (notice = t('common.message.copied')),
            (error: unknown) => (failure = error instanceof Error ? error.message : String(error))
        )
    }
</script>

<svelte:head><title>{t('constants.quick-open')} | Gowave</title></svelte:head>
<main>
    <PageHeader
        title={t('constants.quick-open')}
        description={t('quick-open.page.description')}
        icon="TbLink"
        iconSize={20}
    />
    <div class="targets">
        {#each targets as [entity, label] (entity)}
            {@const id = values[entity]?.trim() ?? ''}
            {@const valid = validEntityId(entity, id)}
            <form
                onsubmit={(event) => {
                    event.preventDefault()
                    if (valid)
                        void navigate(ROUTES.DASHBOARD.OPEN_ENTITY, { params: { entity, id } })
                }}
                aria-label={t(label)}
            >
                <div class="target-heading">
                    <span class="target-icon"><Icon name={icons[entity]} size={20} /></span>
                    <div>
                        <h2>{t(label)}</h2>
                        <code>{`/dashboard/open/${entity}/:id`}</code>
                    </div>
                </div>
                <div class="target-form">
                    <input
                        name={entity}
                        aria-label={entity === OPEN_ENTITY.USER ? 'User ID' : 'UUID'}
                        bind:value={values[entity]}
                        spellcheck="false"
                        autocomplete="off"
                        inputmode={entity === OPEN_ENTITY.USER ? 'numeric' : 'text'}
                        placeholder={entity === OPEN_ENTITY.USER
                            ? '77'
                            : '00000000-0000-0000-0000-000000000000'}
                        aria-invalid={!!id && !valid}
                    >
                    <ActionIcon
                        icon="TbLink"
                        label={t('common.action.copy-link')}
                        size={36}
                        iconSize={18}
                        disabled={!valid}
                        onclick={() => copy(entity, id)}
                    />
                    <ActionIcon
                        icon="TbArrowRight"
                        label={t('common.action.open')}
                        type="submit"
                        tone="teal"
                        size={36}
                        iconSize={18}
                        disabled={!valid}
                    />
                </div>
            </form>
        {/each}
    </div>
    {#if notice}
        <p role="status">{notice}</p>
    {/if}
    {#if failure}
        <p role="alert">{failure}</p>
    {/if}
</main>

<style>
    main,
    .targets,
    form {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-md);
    }
    form {
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .target-heading,
    .target-form {
        display: flex;
        align-items: center;
        min-width: 0;
        gap: var(--mantine-spacing-sm);
    }
    .target-form {
        gap: var(--mantine-spacing-xs);
    }
    .target-icon {
        display: grid;
        place-items: center;
        flex: none;
        width: 34px;
        height: 34px;
        border: 1px solid rgb(21 170 191 / 30%);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(135deg, rgb(21 170 191 / 15%), rgb(16 152 173 / 10%));
        color: var(--mantine-color-cyan-4);
    }
    h2 {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        line-height: var(--mantine-line-height-sm);
    }
    .target-heading > div {
        display: grid;
        min-width: 0;
        gap: 2px;
    }
    code {
        padding: 2px 6px;
        overflow: hidden;
        font: 0.7rem / var(--mantine-line-height) var(--mantine-font-family-monospace);
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    input {
        min-width: 0;
        flex: 1;
        height: 36px;
        min-height: 36px;
        font-size: var(--mantine-font-size-sm);
        border-radius: var(--mantine-radius-md);
        background: light-dark(var(--mantine-color-white), var(--mantine-color-dark-6));
    }
    input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    input[aria-invalid="true"] {
        border-color: var(--mantine-color-error);
    }
    @media (min-width: 48em) {
        .targets {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
</style>
