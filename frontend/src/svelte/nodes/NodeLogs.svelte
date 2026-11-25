<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy } from 'svelte'
    import { z } from 'zod'
    import { callCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Icon from '../ui/Icon.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Switch from '../ui/Switch.svelte'

    let { uuid }: { uuid: string } = $props()
    let type = $state('xray')
    let lines = $state('500')
    let live = $state(false)
    let tail = $state(true)
    let copied = $state(false)
    let copyError = $state('')
    let timer: ReturnType<typeof setTimeout> | undefined
    let t = $derived(locale.t)
    let viewer = $state<HTMLPreElement>()
    const responseSchema = z.object({ response: z.object({ logs: z.array(z.string()) }) })
    const logs = createQuery(() => ({
        queryKey: ['node-logs', uuid, type, lines],
        queryFn: ({ signal }) =>
            callCommand(
                {
                    TSQ_url: `/api/nodes/${uuid}/logs`,
                    endpointDetails: { REQUEST_METHOD: 'GET' },
                    ResponseSchema: responseSchema
                },
                { params: { type, lines }, signal }
            ),
        refetchInterval: live ? 2500 : false,
        refetchOnWindowFocus: false,
        retry: false
    }))
    let count = $derived(logs.data?.logs.length ?? 0)
    let output = $derived(logs.data?.logs.join('\n') ?? '')
    $effect(() => {
        if (output && tail && viewer) viewer.scrollTop = viewer.scrollHeight
    })
    onDestroy(() => clearTimeout(timer))

    async function copyLogs(): Promise<void> {
        copyError = ''
        try {
            await navigator.clipboard.writeText(output)
            copied = true
            clearTimeout(timer)
            timer = setTimeout(() => (copied = false), 2000)
        } catch (error) {
            copyError = error instanceof Error ? error.message : t('common.message.error')
        }
    }
</script>

<div class="toolbar">
    <div class="controls">
        <fieldset class="sources" aria-label={t('node-logs.source')}>
            {#each [
                ['xray', 'Xray Core'],
                ['node', 'Gowave Node (Go)']
            ] as [value, label] (value)}
                <Button
                    variant={type === value ? 'default' : 'subtle'}
                    color="gray"
                    size="xs"
                    aria-pressed={type === value}
                    onclick={() => (type = value ?? 'xray')}
                    >{label}</Button
                >
            {/each}
        </fieldset>
        <div class="line-count">
            <SelectField
                label={t('node-logs.line-count')}
                placeholder=""
                hideLabel
                searchable={false}
                clearable={false}
                options={['100', '500', '1000', '2000'].map((value) => ({
                    value,
                    label: t('node-logs.lines', { value })
                }))}
                bind:value={lines}
            />
        </div>
        <span class="count">{t('node-logs.lines', { value: count })}</span>
    </div>
    <div class="controls">
        <Switch
            label={t('node-logs.auto-refresh')}
            showLabel
            switchSize="sm"
            tone="cyan"
            bind:checked={live}
        />
        <Switch
            label={t('node-logs.auto-scroll')}
            showLabel
            switchSize="sm"
            tone="cyan"
            bind:checked={tail}
        />
        <ActionIcon
            icon="TbRefresh"
            label={t('common.action.refresh')}
            tone="gray"
            size={30}
            iconSize={16}
            disabled={logs.isFetching}
            onclick={() => void logs.refetch()}
        />
        <Button
            variant="light"
            color={copied ? 'teal' : 'gray'}
            size="xs"
            icon={copied ? 'TbCheck' : 'TbCopy'}
            disabled={!output}
            onclick={() => void copyLogs()}
        >
            {copied
                ? t('common.message.copied')
                : t('config-editor-actions.feature.copy-all-content')}
        </Button>
    </div>
</div>
<div class="viewer">
    {#if logs.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {:else if logs.error || copyError}
        <p role="alert">{logs.error?.message || copyError}</p>
    {:else if count}
        <pre bind:this={viewer}>{output}</pre>
    {:else}
        <p><Icon name="TbListDetails" size={32} />{t('common.message.no-data-available')}</p>
    {/if}
</div>

<style>
    .toolbar,
    .controls,
    .sources {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
    }
    .toolbar {
        justify-content: space-between;
        margin-block-end: var(--mantine-spacing-sm);
    }
    .sources {
        gap: 0;
        border: 0;
        margin: 0;
        padding: 2px;
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .line-count {
        width: 130px;
    }
    .count {
        border-radius: 99px;
        padding-inline: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        background: var(--mantine-color-default-hover);
        font-size: var(--mantine-font-size-xs);
    }
    .viewer {
        min-height: 360px;
        padding: var(--mantine-spacing-sm);
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-background);
    }
    .viewer p {
        min-height: 332px;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        margin: 0;
    }
    pre {
        max-height: 55dvh;
        margin: 0;
        overflow: auto;
        white-space: pre-wrap;
        overflow-wrap: anywhere;
        font-size: 12px;
    }
</style>
