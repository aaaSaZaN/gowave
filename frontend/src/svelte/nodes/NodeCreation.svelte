<script lang="ts">
    import { CreateNodeCommand } from '@remnawave/backend-contract'
    import { buildNodeInstallCommand } from '@shared/lib/node-install-command'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { onDestroy, type Snippet } from 'svelte'
    import { submitCommand } from '../commands'
    import { profilesQuery } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import { nodePluginsQuery } from '../plugins/queries'
    import { queryClient } from '../query-client'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import { COUNTRIES } from './countries'
    import { type NodePatch, readConfigProfile, readNodePatch } from './fields'
    import NodeProviderField from './NodeProviderField.svelte'
    import ProfileFields from './ProfileFields.svelte'
    import { nodeQuery, nodeSecretQuery, nodesKey, providersQuery } from './queries'

    let { close, openNode }: { close: () => void; openNode: (uuid: string) => void } = $props()
    let step = $state(0)
    let port = $state<number | undefined>(2222)
    let name = $state('')
    let address = $state('')
    let countryCode = $state('')
    let provider = $state('')
    let plugin = $state('')
    let tags = $state('')
    let additional = $state(false)
    let connection = $state<NodePatch>()
    let uuid = $state('')
    let feedback = $state('')
    let failed = $state(false)
    let copied = $state<'secret' | 'install'>()
    let copyTimer: ReturnType<typeof setTimeout> | undefined
    const t = $derived(locale.t)
    const fieldPrefix = $props.id()
    const secret = createQuery(() => nodeSecretQuery)
    const providers = createQuery(() => ({ ...providersQuery, enabled: additional }))
    const plugins = createQuery(() => ({ ...nodePluginsQuery, enabled: additional }))
    const node = createQuery(() => ({
        ...nodeQuery(uuid),
        enabled: Boolean(uuid),
        refetchInterval: uuid ? 2000 : false
    }))
    const connectionState = $derived(
        node.data?.isConnected && !node.data.isConnecting
            ? 'connected'
            : node.data?.lastStatusMessage && !node.data?.isConnecting
              ? 'error'
              : 'connecting'
    )
    const save = createMutation(() => ({
        mutationFn: (profile: ReturnType<typeof readConfigProfile>) =>
            submitCommand(CreateNodeCommand, { ...connection, configProfile: profile }),
        onSuccess: async (created) => {
            queryClient.setQueryData(nodeQuery(created.uuid).queryKey, created)
            uuid = created.uuid
            step = 2
            await Promise.all([
                queryClient.invalidateQueries({ queryKey: nodesKey }),
                queryClient.invalidateQueries({ queryKey: profilesQuery.queryKey })
            ])
        }
    }))

    function next(event: SubmitEvent): void {
        event.preventDefault()
        if (!(event.currentTarget instanceof HTMLFormElement)) return
        const data = new FormData(event.currentTarget)
        connection = {
            ...readNodePatch(data, (key) => data.has(key), 'create'),
            countryCode: countryCode || 'XX'
        }
        step = 1
    }

    function create(event: SubmitEvent): void {
        event.preventDefault()
        if (save.isPending || !(event.currentTarget instanceof HTMLFormElement)) return
        save.mutate(readConfigProfile(new FormData(event.currentTarget)))
    }

    function copy(install: boolean): void {
        const key = secret.data?.secretKey.trim()
        if (!key) return
        const value = install ? buildNodeInstallCommand(key, port) : key
        void navigator.clipboard.writeText(value).then(
            () => {
                failed = false
                feedback = t('common.message.copied')
                copied = install ? 'install' : 'secret'
                clearTimeout(copyTimer)
                copyTimer = setTimeout(() => (copied = undefined), 2000)
            },
            (error: unknown) => {
                failed = true
                feedback = error instanceof Error ? error.message : String(error)
            }
        )
    }

    onDestroy(() => clearTimeout(copyTimer))
</script>

{#snippet field(
    label: string,
    icon: IconName | undefined,
    fieldContent: Snippet<[string]>,
    required = false
)}
    {@const controlId = `${fieldPrefix}-${encodeURIComponent(label)}`}
    <label class="node-field" for={controlId}>
        <span
            >{label}
            {#if required}
                <span class="required"> *</span>
            {/if}</span
        >
        <span class="control" class:with-icon={!!icon}>
            {#if icon}
                <span class="leading"><Icon name={icon} size={16} /></span>
            {/if}
            {@render fieldContent(controlId)}
        </span>
    </label>
{/snippet}

{#snippet installButton()}
    <Button
        class="install-command"
        data-copied={copied === 'install' || undefined}
        icon={copied === 'install' ? 'PiCheck' : 'TbTerminal'}
        iconSize={18}
        variant="filled"
        color={copied === 'install' ? 'teal' : 'gray'}
        size="md"
        block
        disabled={!secret.data}
        onclick={() => copy(true)}
        >{t('copy-node-install-command.widget.copy-install-command')}</Button
    >
    {#if feedback}
        <p class="feedback" role={failed ? 'alert' : 'status'}>{feedback}</p>
    {/if}
{/snippet}

<Overlay
    title={t('create-node-modal.widget.create-node')}
    icon="TbCpu"
    tone="teal"
    width={440}
    class="node-creation"
    {close}
    preventClose={save.isPending}
>
    <div
        class="step-bars"
        role="progressbar"
        aria-label="Node setup"
        aria-valuemin={0}
        aria-valuemax={3}
        aria-valuenow={step + 1}
    >
        {#each [0, 1, 2] as index (index)}
            <span class:filled={index <= step}></span>
        {/each}
    </div>
    {#if step === 0}
        <form class="connection" onsubmit={next}>
            <p class="instructions">
                {t('create-node-step-1-connection.run-the-install-command')}
                <a
                    href={locale.language.startsWith('ru')
                        ? 'https://aaasazan.github.io/gowave-docs/install/node/install/'
                        : 'https://aaasazan.github.io/gowave-docs/en/install/node/install/'}
                    target="_blank"
                    rel="noopener noreferrer"
                    >{t('create-node-step-1-connection.learn-more')}</a
                >
            </p>
            {@render field('Secret Key (SECRET_KEY)', 'TbCertificate', secretField)}
            {#snippet secretField(controlId: string)}
                <input
                    id={controlId}
                    class="secret"
                    readonly
                    value={secret.data?.secretKey ?? ''}
                    onclick={() => copy(false)}
                >
                <span class="copy-key"
                    ><ActionIcon
                        icon={copied === 'secret' ? 'PiCheck' : 'PiCopy'}
                        {...copied === 'secret' ? { tone: 'teal' } : {}}
                        subtle
                        label={t('common.action.copy')}
                        size={28}
                        iconSize={16}
                        disabled={!secret.data}
                        onclick={() => copy(false)}
                    /></span
                >
            {/snippet}
            {@render field(t('base-node-form.internal-name'), 'TbId', nameField, true)}
            {#snippet nameField(controlId: string)}
                <input
                    id={controlId}
                    name="name"
                    bind:value={name}
                    placeholder={t('base-node-form.internal-name-placeholder')}
                    required
                >
            {/snippet}
            <SelectField
                name="countryCode"
                label={t('base-node-form.country')}
                placeholder={t('base-node-form.select-country')}
                leading="TbMapPin"
                options={COUNTRIES}
                clearable={false}
                bind:value={countryCode}
            />
            <div class="connection-address">
                {@render field(
                    t('create-node-step-1-connection.domain-or-ip'),
                    'TbWorld',
                    addressField,
                    true
                )}
                {#snippet addressField(controlId: string)}
                    <input
                        id={controlId}
                        name="address"
                        bind:value={address}
                        placeholder="192.168.1.1"
                        required
                    >
                {/snippet}
                {@render field('Node Port', undefined, portField, true)}
                {#snippet portField(controlId: string)}
                    <input
                        id={controlId}
                        name="port"
                        type="number"
                        min="1"
                        max="65535"
                        step="1"
                        required
                        placeholder="2222"
                        bind:value={port}
                    >
                {/snippet}
            </div>
            <div class="more">
                <button
                    class="mantine-UnstyledButton-root"
                    type="button"
                    aria-expanded={additional}
                    onclick={() => (additional = !additional)}
                >
                    <Icon name="TbSettings" size={16} />
                    {t('create-node-step-1-connection.additional-options')}<Icon
                        name="TbChevronDown"
                        size={14}
                    />
                </button>
            </div>
            <div class="extra" hidden={!additional}>
                <NodeProviderField
                    providers={providers.data?.providers ?? []}
                    value={provider}
                    onchoose={(value) => (provider = value)}
                />
                <SelectField
                    name="activePluginUuid"
                    label={t('node-vitals.card.plugin')}
                    description={t('node-vitals.card.review-documentation-for-more-information')}
                    placeholder={t('node-vitals.card.select-plugin')}
                    emptyText={t('node-vitals.card.nothing-found')}
                    disabled={false}
                    leading="TbPackage"
                    options={(plugins.data?.nodePlugins ?? []).map(({ uuid, name }) => ({
                        value: uuid,
                        label: name
                    }))}
                    bind:value={plugin}
                />
                <label class="node-field">
                    {t('common.field.tags')}
                    <input name="tags" bind:value={tags}>
                </label>
            </div>
            <div class="form-footer">
                {@render installButton()}
                <div class="button-end">
                    <Button type="submit" size="md" variant="light" color="teal">
                        <span class="next-label"
                            >{t('common.action.next')}<Icon name="PiArrowRight" size={18} /></span
                        >
                    </Button>
                </div>
            </div>
        </form>
    {:else if step === 1}
        <form class="profile-step" onsubmit={create}>
            <FormCard
                title={t('base-node-form.core-configuration')}
                icon="SiSecurityscorecard"
                tone="teal"
            >
                <ProfileFields current={undefined} />
            </FormCard>
            {#if save.error}
                <p role="alert">{save.error.message}</p>
            {/if}
            <div class="form-footer">
                {@render installButton()}
                <div class="button-ends">
                    <Button
                        size="md"
                        icon="PiArrowLeft"
                        disabled={save.isPending}
                        onclick={() => (step = 0)}
                        >{t('create-node-modal.widget.back')}</Button
                    >
                    <Button
                        type="submit"
                        variant="light"
                        color="teal"
                        size="md"
                        icon="TbCheck"
                        disabled={save.isPending}
                        >{t('create-node-modal.widget.create-node')}</Button
                    >
                </div>
            </div>
        </form>
    {:else}
        <div class="status-step">
            <div class="status-panel" data-state={connectionState}>
                <Icon
                    name={connectionState === 'connected'
                        ? 'TbCircleCheck'
                        : connectionState === 'error'
                          ? 'TbAlertCircle'
                          : 'TbLink'}
                    size={96}
                />
                <strong
                    >{connectionState === 'connected'
                        ? t('create-node-modal.widget.connection-successful')
                        : connectionState === 'error'
                          ? t('create-node-modal.widget.connection-failed')
                          : 'Connecting...'}</strong
                >
                <p>
                    {t(
                        connectionState === 'connected'
                            ? 'create-node-step-3-status.xray-core-is-up-and-running'
                            : connectionState === 'error'
                              ? 'create-node-step-3-status.gowave-will-try-to-reconnect-shortly'
                              : 'create-node-step-3-status.establishing-mtls-connection'
                    )}
                </p>
            </div>
            {#if node.data?.lastStatusMessage || node.error}
                <p role="alert">{node.data?.lastStatusMessage || node.error?.message}</p>
            {:else}
                <p role="status">{t('create-node-step-3-status.no-errors-so-far')}</p>
            {/if}
            <div class="button-ends">
                <Button size="md" icon="TbX" onclick={close}>{t('common.action.close')}</Button>
                <Button
                    size="md"
                    variant="light"
                    color={connectionState === 'connected' ? 'teal' : 'blue'}
                    icon="TbLink"
                    onclick={() => openNode(uuid)}
                    >{t('create-node-modal.widget.open-node')}</Button
                >
            </div>
        </div>
    {/if}
</Overlay>

<style>
    :global(.node-creation .overlay-header) {
        padding-block: calc(var(--mantine-spacing-sm) + 0.5px);
    }
    .step-bars {
        display: grid;
        grid-template-columns: repeat(3, 1fr);
        gap: var(--mantine-spacing-xs);
        margin-bottom: var(--mantine-spacing-xl);
    }
    .step-bars span {
        height: 8px;
        border-radius: var(--mantine-radius-sm);
        background: light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-4));
    }
    .step-bars .filled {
        background-color: var(--mantine-color-teal-8);
        background-image: repeating-linear-gradient(
            45deg,
            rgb(255 255 255 / 15%) 0 6px,
            transparent 6px 12px
        );
    }
    .connection,
    .profile-step,
    .extra,
    .form-footer,
    .status-step {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    .connection {
        --select-label-weight: 500;
    }
    .connection,
    .profile-step,
    .status-step {
        min-height: 400px;
    }
    .extra[hidden] {
        display: none;
    }
    .instructions {
        margin: 0;
        padding-bottom: var(--mantine-spacing-xs);
        border-bottom: 1px solid var(--mantine-color-default-border);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }
    .instructions a {
        color: var(--mantine-color-anchor);
        text-decoration: none;
        font-weight: 700;
    }
    .instructions a:hover {
        text-decoration: underline;
    }
    .node-field {
        display: grid;
        gap: 0;
        min-width: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }
    .node-field > span:first-child {
        min-height: calc(var(--mantine-font-size-md) * var(--mantine-line-height-md));
    }
    .node-field input {
        width: 100%;
        height: 36px;
        min-height: 36px;
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
        background: var(--mantine-color-dark-6);
    }
    .node-field:first-of-type {
        font-weight: 600;
    }
    .node-field input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    .required {
        color: var(--mantine-color-error);
    }
    .control {
        position: relative;
        min-width: 0;
    }
    .control.with-icon input {
        padding-inline-start: 36px;
    }
    .leading,
    .copy-key {
        position: absolute;
        inset-block: 0;
        display: grid;
        place-items: center;
        width: 36px;
        color: var(--mantine-color-dimmed);
    }
    .leading {
        inset-inline-start: 0;
        pointer-events: none;
    }
    .copy-key {
        inset-inline-end: 0;
        z-index: 1;
    }
    .secret {
        padding-inline-end: 30px;
        font-family: var(--mantine-font-family-monospace);
        font-weight: 400;
        cursor: copy;
    }
    .control:has(.secret)::after {
        position: absolute;
        inset-block: 1px;
        inset-inline-end: 10px;
        width: 80px;
        background: linear-gradient(90deg, transparent, var(--mantine-color-default) 70%);
        pointer-events: none;
        content: "";
    }
    .connection-address {
        display: grid;
        grid-template-columns: minmax(0, 70%) minmax(0, 25%);
        gap: var(--mantine-spacing-xs);
    }
    .more {
        display: flex;
        gap: var(--mantine-spacing-xs);
        align-items: center;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .more::before,
    .more::after {
        content: "";
        height: 1px;
        flex: 1;
        background: var(--mantine-color-default-border);
    }
    .more button {
        display: flex;
        gap: 6px;
        align-items: center;
        padding-inline: 4px;
        color: inherit;
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }
    .form-footer {
        padding-top: var(--mantine-spacing-lg);
        margin-top: auto;
    }
    .button-end {
        display: flex;
        justify-content: end;
    }
    .next-label {
        display: flex;
        align-items: center;
        gap: 3px;
    }
    :global(.install-command[data-variant="filled"]:not(:disabled, [data-copied])) {
        background: light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5));
        color: var(--mantine-color-bright);
    }
    :global(.install-command[data-variant="filled"]:not(:disabled, [data-copied]):hover) {
        background: var(--mantine-color-default-hover);
    }
    .button-ends {
        display: flex;
        justify-content: space-between;
    }
    .feedback {
        margin: 0;
        font-size: var(--mantine-font-size-xs);
    }
    .status-panel {
        display: grid;
        justify-items: center;
        text-align: center;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-xl);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: linear-gradient(
            to top,
            var(--mantine-color-dark-6),
            var(--mantine-color-dark-7)
        );
    }
    .status-panel > strong {
        color: var(--mantine-color-blue-4);
    }
    .status-panel[data-state="connected"] > strong {
        color: var(--mantine-color-teal-4);
    }
    .status-panel[data-state="error"] > strong {
        color: var(--mantine-color-red-4);
    }
    .status-panel p {
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
</style>
