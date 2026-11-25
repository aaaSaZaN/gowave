<script lang="ts">
    import type Highcharts from 'highcharts'
    import { onMount } from 'svelte'
    import { locale } from '../i18n.svelte'

    let { options }: { options: Highcharts.Options } = $props()
    let container: HTMLDivElement
    let width = $state(0)
    let chart: Highcharts.Chart | undefined
    let engine = $state.raw<typeof Highcharts>()
    let loadFailed = $state(false)

    onMount(() => {
        let mounted = true
        void import('highcharts').then(
            ({ default: highcharts }) => {
                if (mounted) engine = highcharts
            },
            () => {
                if (mounted) loadFailed = true
            }
        )

        return () => {
            mounted = false
            chart?.destroy()
        }
    })

    $effect(() => {
        if (!engine || !width) return
        const height = options.chart?.height
        const measuredOptions =
            typeof height === 'string' && height.endsWith('%')
                ? {
                      ...options,
                      chart: { ...options.chart, height: (Number.parseFloat(height) * width) / 100 }
                  }
                : options
        chart ??= engine.chart(container, {})
        chart.update(measuredOptions, true, true)
    })
</script>

<div bind:this={container} bind:clientWidth={width}></div>
{#if loadFailed}
    <p role="alert">{locale.t('common.message.error')}</p>
{/if}
