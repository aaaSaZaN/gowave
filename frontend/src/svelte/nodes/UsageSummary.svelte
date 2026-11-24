<script lang="ts">
    import { sparklinePaths } from '@shared/lib/sparkline'
    import { prettifyBytesUtil } from '@shared/utils/bytes/pretty-bytes/pretty-bytes.util'
    import { locale } from '../i18n.svelte'
    import { usageSummary } from './analytics'

    let { values, title }: { values: number[]; title?: string } = $props()
    const headingId = $props.id()
    const gradientId = `${headingId}-traffic-gradient`
    let t = $derived(locale.t)
    let summary = $derived(usageSummary(values))
    let spark = $derived(sparklinePaths(values.length ? values : [1, 1, 1], true))
    let first = $derived(values[0] ?? 0)
    let last = $derived(values.at(-1) ?? 0)
    let stroke = $derived(
        values.length === 0 || first === last
            ? 'var(--mantine-color-gray-5)'
            : last > first
              ? 'var(--mantine-color-teal-6)'
              : 'var(--mantine-color-red-6)'
    )

    function formatBytes(value: number): string {
        return value > 0 ? (prettifyBytesUtil(value) ?? '0 GiB') : '0 GiB'
    }
</script>

<section aria-labelledby={headingId}>
    <h2 id={headingId}>
        {title ?? t('statistic-nodes.component.total-traffic')}
    </h2>
    <p class="total">{formatBytes(summary.total)}</p>
    <dl>
        <div>
            <dt>Ø / day</dt>
            <dd>{formatBytes(summary.average)}</dd>
        </div>
        <div>
            <dt>{t('statistic-sparkline-card.widget.peak')}</dt>
            <dd>{formatBytes(summary.peak)}</dd>
        </div>
    </dl>
    <svg class="trend" viewBox="0 0 1000 100" preserveAspectRatio="none" aria-hidden="true">
        <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color={stroke} stop-opacity="0.3" />
                <stop offset="100%" stop-color={stroke} stop-opacity="0" />
            </linearGradient>
        </defs>
        <path d={spark.area} fill={`url(#${gradientId})`} />
        <path
            d={spark.line}
            fill="none"
            {stroke}
            stroke-width="2"
            vector-effect="non-scaling-stroke"
        />
    </svg>
</section>

<style>
    section {
        min-width: 0;
        display: flex;
        flex-direction: column;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    h2 {
        margin: 0 0 4px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 500;
        line-height: var(--mantine-line-height-xs);
        text-transform: uppercase;
    }
    .total {
        font-size: 2rem;
        font-weight: 700;
        margin: 0;
        line-height: 1;
    }
    dl {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-sm);
        margin: var(--mantine-spacing-md) 0;
        font-size: var(--mantine-font-size-sm);
    }
    dt {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    dd {
        margin: 0;
        font-weight: 600;
        line-height: var(--mantine-line-height-sm);
    }
    .trend {
        width: 100%;
        height: 100px;
    }
</style>
