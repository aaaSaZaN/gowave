<script lang="ts">
    import type { Snippet } from 'svelte'
    import ConfirmOverlay from './ConfirmOverlay.svelte'
    import { type Confirmation, confirmContext } from './confirmation'

    let { children }: { children: Snippet } = $props()
    let request = $state<Confirmation>()

    function finish(confirmed: boolean): void {
        const current = request
        request = undefined
        current?.resolve(confirmed)
    }

    confirmContext.set({
        ask(owner, message, options) {
            if (request) return Promise.resolve(false)
            return new Promise((resolve) => {
                request = { owner, message, options, resolve }
            })
        },
        cancel(owner) {
            if (request?.owner === owner) finish(false)
        }
    })
</script>

{@render children()}

{#if request}
    {@render confirmation(request)}
{/if}

{#snippet confirmation(current: Confirmation)}
    {#key current}
        <ConfirmOverlay
            message={current.message}
            {...current.options}
            close={(confirmed) => {
                if (request === current) finish(confirmed)
            }}
        />
    {/key}
{/snippet}
