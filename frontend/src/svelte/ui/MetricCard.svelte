<script lang="ts">
    import { formatInt } from '@shared/utils/misc/number'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'
    import metric from './metric-card.module.css'

    let {
        label,
        value,
        icon,
        color,
        valueFont = 'monospace',
        subtitle,
        mobileLabel,
        comparison,
        period
    }: {
        label: string
        value: number | string
        icon: IconName
        color: string
        valueFont?: 'monospace' | 'text'
        subtitle?: string
        mobileLabel?: string
        comparison?: string | undefined
        period?: string | undefined
    } = $props()

    const negative = $derived(comparison?.startsWith('-'))
</script>

<div class="metric-card" class:compact-on-mobile={mobileLabel !== undefined}>
    <span
        class="metric-icon"
        style:--metric-ink={`var(--mantine-color-${color}-4)`}
        style:--metric-edge={`var(--mantine-color-${color}-6)`}
        style:--metric-shade={`var(--mantine-color-${color}-7)`}
    >
        <Icon name={icon} size={24} />
    </span>
    <div class="metric-copy">
        <p class={[metric['title'], 'metric-primary-title']} title={label}>{label}</p>
        <p class={[metric['value'], 'metric-value']} class:text-value={valueFont === 'text'}>
            {typeof value === 'number' ? formatInt(value) : value}
        </p>
        {#if mobileLabel}
            <p class="mobile-label">{mobileLabel}</p>
        {/if}
        {#if subtitle}
            <p class={[metric['subtitle'], 'metric-secondary-title']}>{subtitle}</p>
        {/if}
        {#if comparison !== undefined}
            <div
                class={metric['trendContainer']}
                style:--trend-color={negative
                    ? 'var(--mantine-color-red-6)'
                    : 'var(--mantine-color-teal-6)'}
            >
                <Icon
                    name={negative ? 'PiTrendDown' : 'PiTrendUp'}
                    size="0.85rem"
                    color="var(--trend-color)"
                />
                <span class={metric['trendValue']} style:color="var(--trend-color)"
                    >{comparison}</span
                >
                {#if period}
                    <p class={metric['trendText']}>{period}</p>
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .metric-card {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
        min-width: 0;
        height: 100%;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }

    .metric-icon {
        display: inline-flex;
        width: 44px;
        height: 44px;
        flex: 0 0 auto;
        align-items: center;
        justify-content: center;
        border: 1px solid color-mix(in srgb, var(--metric-edge) 30%, transparent);
        border-radius: var(--mantine-radius-lg);
        color: var(--metric-ink);
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--metric-edge) 15%, transparent),
            color-mix(in srgb, var(--metric-shade) 10%, transparent)
        );
    }

    .metric-copy {
        min-width: 0;
    }

    p {
        margin: 0;
    }
    .metric-value.text-value {
        font-family: var(--mantine-font-family);
        font-size: var(--mantine-font-size-md);
    }
    .mobile-label {
        display: none;
    }
    @media (max-width: 64em) {
        .metric-card.compact-on-mobile {
            padding: var(--mantine-spacing-sm);
            gap: var(--mantine-spacing-xs);
            background: var(--gw-card-bg);
        }
        .compact-on-mobile .metric-icon {
            width: 34px;
            height: 34px;
            border-radius: var(--mantine-radius-md);
        }
        .compact-on-mobile .metric-icon :global(svg) {
            width: 18px;
            height: 18px;
        }
        .compact-on-mobile .metric-primary-title,
        .compact-on-mobile .metric-secondary-title {
            display: none;
        }
        .compact-on-mobile .metric-value {
            font-family: var(--mantine-font-family);
            font-size: var(--mantine-font-size-lg);
            line-height: var(--mantine-line-height-lg);
        }
        .mobile-label {
            display: block;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            font-size: var(--mantine-font-size-xs);
            color: var(--mantine-color-dimmed);
            line-height: var(--mantine-line-height-xs);
        }
    }
</style>
