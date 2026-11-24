<script lang="ts">
    import { RESET_PERIODS } from '@remnawave/backend-contract'
    import { prettifyBytesUtil } from '@shared/utils/bytes'
    import { locale } from '../i18n.svelte'
    import type { User } from './queries'

    let { user }: { user: User } = $props()
    const t = $derived(locale.t)
    const unlimited = $derived(user.trafficLimitBytes === 0)
    const percentage = $derived(
        unlimited ? 0 : (user.userTraffic.usedTrafficBytes * 100) / user.trafficLimitBytes
    )
    const strategy = $derived(
        {
            [RESET_PERIODS.MONTH]: t('data-usage.column.monthly'),
            [RESET_PERIODS.WEEK]: t('data-usage.column.weekly'),
            [RESET_PERIODS.DAY]: t('data-usage.column.daily'),
            [RESET_PERIODS.MONTH_ROLLING]: `${t('data-usage.column.monthly')} ↻`,
            [RESET_PERIODS.NO_RESET]: '∞'
        }[user.trafficLimitStrategy]
    )
    const color = $derived(
        unlimited || percentage <= 80
            ? 'var(--mantine-color-teal-8)'
            : percentage > 95
              ? 'var(--mantine-color-red-8)'
              : 'var(--mantine-color-yellow-4)'
    )
</script>

<div class="usage">
    <div class="percent">
        <span class="used">{percentage.toFixed(2)}% <span>{strategy}</span></span>
        <span class="left"
            ><span class="lifetime"
                >Σ {prettifyBytesUtil(user.userTraffic.lifetimeUsedTrafficBytes) || '0 B'}</span
            > {(100 - percentage).toFixed(2)}%</span
        >
    </div>
    <div
        class="progress"
        role="progressbar"
        aria-label={t('use-table-columns.data-usage')}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.min(100, percentage)}
    >
        <div
            style:width={`${unlimited ? 100 : Math.min(100, percentage)}%`}
            style:background={color}
        ></div>
    </div>
    <div class="bytes">
        <span>{prettifyBytesUtil(user.userTraffic.usedTrafficBytes) || '0 B'}</span>
        <span>{unlimited ? '∞' : prettifyBytesUtil(user.trafficLimitBytes) || '∞'}</span>
    </div>
</div>

<style>
    .usage {
        min-width: 300px;
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }

    .percent,
    .bytes {
        display: flex;
        justify-content: space-between;
    }

    .used {
        color: var(--mantine-color-red-5);
        font-weight: 700;
        line-height: var(--mantine-line-height);
    }

    .used span,
    .lifetime,
    .bytes {
        color: var(--mantine-color-dimmed);
    }

    .used span {
        font-weight: 400;
    }

    .lifetime,
    .bytes {
        font-weight: 550;
    }

    .left {
        color: var(--mantine-color-teal-5);
        font-weight: 700;
        line-height: var(--mantine-line-height);
    }

    .progress {
        height: 8px;
        overflow: hidden;
        border-radius: var(--mantine-radius-xs);
        background: light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-4));
    }

    .progress div {
        height: 100%;
    }

    .bytes {
        margin-top: 2px;
    }
</style>
