<script lang="ts">
    import { CreateHostCommand } from '@remnawave/backend-contract'
    import { z } from 'zod'
    import { callCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { formatPreviewBody } from './format-preview-body'
    import { formText, hostPatch } from './host-fields'
    import type { Host } from './queries'

    let { target, close }: { target: { saved: Host } | { draft: FormData }; close: () => void } =
        $props()

    let t = $derived(locale.t)
    let userAgent = $state('ClashMeta')
    let headers = $state('')
    let previewAsEnabled = $state(false)
    let copied = $state(false)
    let copyError = $state('')
    let pending = $state(false)
    let problem = $state('')
    let disabled = $derived(
        'saved' in target ? target.saved.isDisabled : target.draft.has('isDisabled')
    )

    const previewCommand = {
        TSQ_url: '/api/hosts/preview',
        endpointDetails: { REQUEST_METHOD: 'POST' },
        ResponseSchema: z.object({
            response: z.object({
                clientType: z.string(),
                contentType: z.string(),
                headers: z.record(z.string(), z.string()),
                previewContent: z.string(),
                ruleName: z.string().nullable(),
                status: z.number().nullable()
            })
        })
    }
    let result = $state<z.output<typeof previewCommand.ResponseSchema>['response']>()
    let responseHeaders = $derived(
        result
            ? Object.entries(result.headers)
                  .map(([name, value]) => `${name}: ${value}`)
                  .join('\n')
            : ''
    )

    function reset(): void {
        result = undefined
        problem = ''
        copied = false
        copyError = ''
    }

    async function generate(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (pending) return
        reset()
        pending = true
        try {
            const requestHeaders = headers
                .split('\n')
                .map((line) => line.trim())
                .filter(Boolean)
                .map((line) => {
                    const separator = line.indexOf(':')
                    const name = line.slice(0, separator).trim()
                    if (separator <= 0 || !name) throw new Error(t('host-preview.invalid-header'))
                    return { name, value: line.slice(separator + 1).trim() }
                })
            if ('draft' in target && !formText(target.draft, 'inbound')) {
                throw new Error(t('common.message.no-inbound-selected'))
            }
            const hostTarget =
                'saved' in target
                    ? { hostUuid: target.saved.uuid }
                    : {
                          host: CreateHostCommand.RequestBodySchema.parse(
                              hostPatch(target.draft, () => true)
                          )
                      }
            result = await callCommand(previewCommand, {
                data: { ...hostTarget, userAgent, requestHeaders, previewAsEnabled }
            })
        } catch (error) {
            problem = error instanceof Error ? error.message : String(error)
        } finally {
            pending = false
        }
    }

    async function copy(content: string): Promise<void> {
        copyError = ''
        try {
            await navigator.clipboard.writeText(content)
            copied = true
        } catch (error) {
            copyError = error instanceof Error ? error.message : String(error)
        }
    }
</script>

<Overlay title={t('host-preview.title')} icon="PiEye" width={840} {close}>
    <form onsubmit={(event) => void generate(event)}>
        <p class="description">
            {t('saved' in target ? 'host-preview.saved-host' : 'host-preview.draft-host')}
        </p>
        <div class="preview-columns">
            <label>
                User-Agent
                <input
                    disabled={pending}
                    bind:value={userAgent}
                    oninput={reset}
                    placeholder="ClashMeta, singbox, v2rayN/7.0.0"
                >
            </label>
            <label>
                {t('host-preview.request-headers')}
                <textarea
                    rows="4"
                    disabled={pending}
                    bind:value={headers}
                    oninput={reset}
                    placeholder={'Accept: application/json\nX-Client: desktop'}
                ></textarea>
                <small class="description">{t('host-preview.headers-format')}</small>
            </label>
        </div>
        <fieldset class="generate" disabled={pending}>
            <Button type="submit" variant="filled" disabled={pending}>
                {t(pending ? 'common.message.loading' : 'host-preview.generate')}
            </Button>
            {#if disabled}
                <label class="checkbox">
                    <input type="checkbox" bind:checked={previewAsEnabled} onchange={reset}>
                    {t('host-preview.as-enabled')}
                </label>
            {/if}
        </fieldset>

        {#if result}
            <div role="status">
                <div>
                    {result.status === null
                        ? t('host-preview.socket-drop')
                        : t('host-preview.status', { status: result.status })}
                    {result.clientType ? ` · ${result.clientType}` : ''}
                </div>
                <div>
                    {t('host-preview.matched-rule', {
                        rule: result.ruleName ?? t('common.message.not-set')
                    })}
                </div>
            </div>
        {/if}
        {#if problem || copyError}
            <p role="alert">{problem || copyError}</p>
        {/if}
        {#if responseHeaders}
            <section class="headers">
                <strong>{t('host-preview.response-headers')}</strong>
                <pre>{responseHeaders}</pre>
            </section>
        {/if}
        {#if result?.previewContent}
            {@const content = result.previewContent}
            <section class="body">
                <div class="body-heading">
                    <strong>{t('host-preview.response-body')}</strong>
                    <Button variant="light" onclick={() => void copy(content)}
                        >{copied ? t('common.message.copied') : t('common.action.copy')}</Button
                    >
                </div>
                <pre>{formatPreviewBody(content, result.contentType)}</pre>
            </section>
        {/if}
    </form>
</Overlay>

<style>
    form,
    fieldset,
    label {
        display: grid;
        gap: var(--mantine-spacing-xs);
        align-content: start;
        min-width: 0;
    }
    form,
    fieldset {
        gap: var(--mantine-spacing-sm);
    }
    fieldset {
        border: 0;
        margin: 0;
        padding: 0;
    }
    .preview-columns {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-md);
    }
    .description {
        color: var(--mantine-color-dimmed);
        margin: 0;
    }
    .checkbox {
        display: flex;
        align-items: center;
    }
    textarea {
        font-family: var(--mantine-font-family-monospace);
    }
    .generate,
    .body-heading {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-sm);
    }
    section,
    .body-heading {
        min-width: 0;
    }
    pre {
        margin: var(--mantine-spacing-xs) 0 0;
        padding: var(--mantine-spacing-sm);
        overflow: auto;
        max-height: 280px;
        background: var(--gw-raised-bg);
        border-radius: var(--mantine-radius-sm);
        font-family: var(--mantine-font-family-monospace);
    }
    .headers pre {
        max-height: 180px;
    }
    @media (max-width: 36em) {
        .preview-columns {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
