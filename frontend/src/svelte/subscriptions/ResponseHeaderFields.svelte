<script lang="ts">
    import { sortResponseHeadersByPriority } from '@shared/utils/misc/sort-response-headers'
    import { type Snippet, untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import TemplateVariables from './TemplateVariables.svelte'

    let {
        headers,
        children,
        compact = false
    }: {
        headers: Record<string, string> | null
        children?: Snippet<[Snippet, () => void]>
        compact?: boolean
    } = $props()
    const id = $props.id()
    let t = $derived(locale.t)
    let nextId = 0
    let rows = $state(
        untrack(() =>
            sortResponseHeadersByPriority(
                Object.entries(headers ?? {}).map(([key, value]) => ({ id: nextId++, key, value }))
            )
        )
    )

    function addHeader(): void {
        rows.push({ id: nextId++, key: '', value: '' })
    }
</script>

{#snippet fields()}
    {#each rows as row (row.id)}
        <div class="header-row" class:compact>
            <label>
                {#if !compact}
                    {t('headers-manager.widget.key')}
                {/if}
                <input
                    name="headerName"
                    aria-label={t('headers-manager.widget.key')}
                    placeholder={compact ? t('headers-manager.widget.key') : undefined}
                    bind:value={row.key}
                    spellcheck="false"
                >
            </label>
            <div class="value">
                {#if !compact}
                    <label for={id + row.id}>{t('headers-manager.widget.value')}</label>
                {/if}
                <div>
                    <TemplateVariables />
                    <textarea
                        id={id + row.id}
                        name="headerValue"
                        rows="1"
                        aria-label={t('headers-manager.widget.value')}
                        placeholder={compact ? t('headers-manager.widget.value') : undefined}
                        bind:value={row.value}
                    ></textarea>
                </div>
            </div>
            <ActionIcon
                icon="PiTrashDuotone"
                label={t('common.action.delete')}
                tone="red"
                size={36}
                iconSize={18}
                onclick={() => (rows = rows.filter(({ id }) => id !== row.id))}
            />
        </div>
    {/each}
{/snippet}
{#if children}
    {@render children(fields, addHeader)}
{:else}
    {@render fields()}
    <div class="add">
        <Button variant="light" icon="TbPlus" onclick={addHeader}>
            {t('headers-manager.widget.add-header')}
        </Button>
    </div>
{/if}

<style>
    label,
    .value {
        display: grid;
        gap: 4px;
        min-width: 0;
        font-size: var(--mantine-font-size-sm);
    }
    .header-row {
        display: grid;
        grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) auto;
        align-items: end;
        gap: var(--mantine-spacing-xs);
    }
    .value > div {
        display: flex;
        gap: 4px;
        align-items: center;
    }
    textarea {
        flex: 1;
        min-width: 0;
    }
    .header-row.compact {
        grid-template-columns: auto minmax(0, 35%) minmax(0, 1fr);
        align-items: start;
        gap: var(--mantine-spacing-sm);
    }
    .compact > :global(.action-icon) {
        grid-column: 1;
        grid-row: 1;
    }
    .compact label {
        grid-column: 2;
        grid-row: 1;
    }
    .compact .value {
        grid-column: 3;
        grid-row: 1;
    }
    .compact .value > div {
        position: relative;
    }
    .compact .value :global(.action-icon) {
        position: absolute;
        inset-block-start: 3px;
        inset-inline-start: 3px;
    }
    .compact :is(input, textarea) {
        width: 100%;
        min-width: 0;
        min-height: 36px;
        border-radius: var(--mantine-radius-md);
    }
    .compact textarea {
        field-sizing: content;
        max-height: 9em;
        margin: 0;
        padding-inline-start: 36px;
        resize: none;
    }
    .add {
        justify-self: end;
    }
    @media (max-width: 36em) {
        .header-row:not(.compact) {
            grid-template-columns: minmax(0, 1fr) auto;
        }
        .header-row:not(.compact) > label {
            grid-column: 1 / -1;
        }
    }
</style>
