<script lang="ts">
    import { formatInt } from '@shared/utils/misc/number'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'
    import metric from './metric-card.module.css'
    import ThemeIcon from './ThemeIcon.svelte'

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
    <ThemeIcon class="metric-icon" {icon} tone={color} size={44} iconSize={24} radius="lg" />
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
        .compact-on-mobile :global(.metric-icon) {
            width: 34px;
            height: 34px;
            border-radius: var(--mantine-radius-md);
        }
        .compact-on-mobile :global(.metric-icon) :global(svg) {
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
