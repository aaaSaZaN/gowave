<script lang="ts">
    import type { TOpenEntity } from '@shared/constants/open-entity'
    import { buildOpenEntityUrl } from '@shared/constants/routes'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'

    let { entity, id }: { entity: TOpenEntity; id: string | number } = $props()
    let outcome = $state<'copied' | 'failed'>()
    let timer: ReturnType<typeof setTimeout> | undefined

    function result(next: 'copied' | 'failed'): void {
        outcome = next
        clearTimeout(timer)
        timer = setTimeout(() => (outcome = undefined), 1500)
    }

    function copy(): void {
        void navigator.clipboard.writeText(buildOpenEntityUrl(entity, id)).then(
            () => result('copied'),
            () => result('failed')
        )
    }

    onDestroy(() => clearTimeout(timer))
</script>

<ActionIcon
    icon={outcome === 'copied' ? 'TbCheck' : 'TbLink'}
    tone={outcome === 'copied' ? 'teal' : 'gray'}
    label={locale.t('common.action.copy-link')}
    onclick={copy}
/>
{#if outcome === 'failed'}
    <span role="alert">{locale.t('common.message.error')}</span>
{/if}
