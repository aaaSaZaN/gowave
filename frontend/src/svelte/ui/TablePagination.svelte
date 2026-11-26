<script lang="ts">
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'
    import SelectField from './SelectField.svelte'

    let {
        page,
        size,
        sizes,
        total,
        paused = false,
        onpage,
        onsize
    }: {
        page: number
        size: number
        sizes: readonly number[]
        total: number
        paused?: boolean
        onpage: (page: number) => void
        onsize: (size: number) => void
    } = $props()

    const t = $derived(locale.t)
    const choices = $derived(
        [...new Set([...sizes, size])]
            .sort((a, b) => a - b)
            .map((size) => ({
                value: String(size),
                label: String(size)
            }))
    )
    const first = $derived(total ? page * size + 1 : 0)
    const last = $derived(Math.min(total, (page + 1) * size))
</script>

<footer class="pagination">
    <div class="page-size">
        <span>{t('common.field.rows-per-page')}</span>
        <div class="choices">
            <SelectField
                label={t('common.field.rows-per-page')}
                placeholder=""
                options={choices}
                value={String(size)}
                searchable={false}
                clearable={false}
                hideLabel
                onchoose={(value) => onsize(Number(value))}
            />
        </div>
    </div>
    <span role="status">
        {t('common.message.table-range', {
            first: first.toLocaleString(locale.language),
            last: last.toLocaleString(locale.language),
            total: total.toLocaleString(locale.language)
        })}
    </span>
    <div class="page-actions">
        <ActionIcon
            icon="PiCaretLeft"
            size={28}
            iconSize={18}
            label={t('common.action.previous')}
            disabled={page === 0 || paused}
            subtle
            onclick={() => onpage(page - 1)}
        />
        <ActionIcon
            icon="PiCaretRight"
            size={28}
            iconSize={18}
            label={t('common.action.next')}
            disabled={last >= total || paused}
            subtle
            onclick={() => onpage(page + 1)}
        />
    </div>
</footer>

<style>
    .pagination,
    .page-size,
    .page-actions {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }

    .pagination {
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        border-block-start: 1px solid var(--mantine-color-default-border);
    }

    .choices {
        width: 80px;
    }
</style>
