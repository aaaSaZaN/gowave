<script lang="ts">
    import { setupHostJsonFields, setupHostMapperSchema } from '@shared/utils/monaco/host-schemas'
    import CodeEditor from '../editor/CodeEditor.svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { dismissContainingDialog } from '../ui/dialog-motion'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'
    import { type jsonOptions, parseHostJson } from './host-fields'

    let {
        field,
        label,
        value,
        rawInbound,
        disabled = false,
        onchange
    }: {
        field: keyof typeof jsonOptions
        label: string
        value: string
        rawInbound: unknown
        disabled?: boolean
        onchange: () => void
    } = $props()
    const fields = {
        xhttpExtraParams: {
            scheme: 'host-xhttp-extra',
            icon: 'PiPencil',
            docs: 'https://xtls.github.io/ru/config/transports/splithttp.html',
            sample: `{
    "headers": {},
    "xPaddingBytes": "100-1000",
    "noGRPCHeader": false,
    "scMaxEachPostBytes": 1000000,
    "scMinPostsIntervalMs": 30,
    "scStreamUpServerSecs": "20-80",
    "xmux": {
      "maxConcurrency": "16-32",
      "maxConnections": 0,
      "cMaxReuseTimes": 0,
      "hMaxRequestTimes": "600-900",
      "hMaxReusableSecs": "1800-3000",
      "hKeepAlivePeriod": 0
    }
  }`
        },
        muxParams: {
            scheme: 'host-mux',
            icon: 'TbCloudNetwork',
            docs: 'https://xtls.github.io/ru/config/outbound.html#muxobject',
            sample: `{
    "enabled": true,
    "concurrency": -1,
    "xudpConcurrency": 16,
    "xudpProxyUDP443": "reject"
  }`,
            title: 'MUX'
        },
        sockoptParams: {
            scheme: 'host-sockopt',
            icon: 'PiNetwork',
            docs: 'https://xtls.github.io/ru/config/transports/sockopt.html',
            sample: `{
    "mark": 0,
    "tcpMaxSeg": 1440,
    "tcpFastOpen": false,
    "tproxy": "off",
    "domainStrategy": "AsIs",
    "dialerProxy": "",
    "happyEyeballs": {},
    "acceptProxyProtocol": false,
    "tcpKeepAliveInterval": 0,
    "tcpKeepAliveIdle": 300,
    "tcpUserTimeout": 10000,
    "tcpcongestion": "bbr",
    "interface": "wg0",
    "V6Only": false,
    "tcpWindowClamp": 600,
    "tcpMptcp": false,
    "tcpNoDelay": false
  }`,
            title: 'SockOpt'
        },
        finalMask: {
            scheme: 'host-final-mask',
            icon: 'TbMask',
            docs: 'https://xtls.github.io/ru/config/transports/finalmask.html',
            sample: `{
    "tcp": [
      {
        "type": "",
        "settings": {}
      }
    ],
    "udp": [
      {
        "type": "",
        "settings": {}
      }
    ],
    "quicParams": {}
  }`,
            title: 'Final Mask'
        },
        mapper: { scheme: 'host-mapper', icon: 'TbJson' }
    } satisfies Record<
        keyof typeof jsonOptions,
        {
            scheme: string
            icon: IconName
            docs?: string
            sample?: string
            title?: string
        }
    >
    const config = $derived(fields[field])
    const docs = $derived('docs' in config ? config.docs : undefined)
    const sample = $derived('sample' in config ? config.sample : undefined)
    const title = $derived('title' in config ? config.title : label)
    let saved = $state<string>()
    let text = $derived(saved ?? value)
    let draft = $state('')
    let open = $state(false)
    let markerError = $state('')
    let fullscreen = $state(false)
    let ready = $state(false)
    let t = $derived(locale.t)
    const error = $derived.by(() => {
        try {
            parseHostJson(draft, field === 'mapper')
            return markerError
        } catch {
            return t('common.message.invalid-json')
        }
    })

    $effect(() => {
        if (!ready) return
        if (field === 'mapper') setupHostMapperSchema(rawInbound)
        else if (field !== 'xhttpExtraParams') void setupHostJsonFields(locale.language)
    })

    function edit(): void {
        draft = text
        markerError = ''
        open = true
    }

    function close(): void {
        open = false
        ready = false
        fullscreen = false
    }

    function apply(event: MouseEvent): void {
        if (!ready || error) return
        if (draft.trim() !== text.trim()) {
            saved = draft.trim()
            onchange()
        }
        dismissContainingDialog(event)
    }
</script>

<input type="hidden" name={field} value={text}>
<Button
    size="compact-sm"
    color="gray"
    variant="soft"
    icon={config.icon}
    {disabled}
    aria-haspopup="dialog"
    aria-label={`${t('common.action.edit')} ${label}`}
    onclick={edit}
>
    <span class="edit-label">
        {text.trim() && text.trim() !== '{}'
            ? t('common.message.configured')
            : t('common.message.not-set')}
        <Icon name="PiPencil" size={14} />
    </span>
</Button>

{#if open}
    <Overlay
        {title}
        icon={config.icon}
        tone="teal"
        width={900}
        class={fullscreen ? 'host-json-expanded' : ''}
        {close}
    >
        <div class="editor" class:invalid={Boolean(error)}>
            <CodeEditor
                language="json"
                modelUri={`${config.scheme}://editor`}
                {label}
                value={draft}
                compact
                onchange={(next) => {
                    draft = next
                    markerError = ''
                }}
                onready={(editor) => (ready = Boolean(editor))}
                onvalidate={(error) => (markerError = error)}
            />
            {#if error}
                <p role="alert">{error}</p>
            {/if}
        </div>
        <div class="actions" class:invalid={Boolean(error)}>
            <ActionIcon
                icon={fullscreen ? 'TbMinimize' : 'TbMaximize'}
                label={fullscreen ? 'Exit fullscreen' : 'Fullscreen'}
                aria-pressed={fullscreen}
                size={36}
                iconSize={18}
                tone="gray"
                onclick={() => (fullscreen = !fullscreen)}
            />
            {#if docs}
                <ActionIcon
                    icon="TbBook"
                    label={t('common.action.documentation')}
                    href={docs}
                    target="_blank"
                    rel="noopener noreferrer"
                    size={36}
                    iconSize={18}
                    tone="gray"
                />
            {/if}
            {#if sample}
                <Button
                    variant="soft"
                    color="gray"
                    icon="TbArrowUp"
                    disabled={!ready}
                    onclick={() => {
                        markerError = ''
                        draft = sample
                    }}
                >
                    {t('common.action.paste-default')}
                </Button>
            {/if}
            <div class="save">
                <Button variant="subtle" color="cyan" onclick={dismissContainingDialog}
                    >{t('common.action.cancel')}</Button
                >
                <Button
                    color="cyan"
                    variant="soft"
                    disabled={!ready || Boolean(error)}
                    onclick={apply}
                >
                    {t('common.action.save')}
                </Button>
            </div>
        </div>
    </Overlay>
{/if}

<style>
    .editor {
        display: flex;
        flex-direction: column;
        height: min(460px, calc(100dvh - 10rem));
        min-height: 12rem;
        overflow: hidden;
        border: 1px solid var(--mantine-color-dark-4);
        border-radius: var(--mantine-radius-md) var(--mantine-radius-md) 0 0;
    }
    .actions,
    .save,
    .edit-label {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .actions {
        flex-wrap: wrap;
        padding: var(--mantine-spacing-xs);
        border: 1px solid var(--mantine-color-dark-4);
        border-block-start: 0;
        border-radius: 0 0 var(--mantine-radius-md) var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .save {
        margin-inline-start: auto;
    }
    .invalid {
        border-color: var(--mantine-color-red-5);
    }
    .editor > [role="alert"] {
        margin: 0;
        padding: var(--mantine-spacing-xs);
        color: var(--mantine-color-red-4);
        background: var(--gw-raised-bg);
        font-size: var(--mantine-font-size-sm);
    }
    :global(dialog.host-json-expanded) {
        inset: 0;
        margin: 0;
        width: 100%;
        max-height: 100dvh;
        height: 100dvh;
        border-radius: 0;
    }
    :global(.host-json-expanded) .editor {
        height: calc(100dvh - 10rem);
    }
</style>
