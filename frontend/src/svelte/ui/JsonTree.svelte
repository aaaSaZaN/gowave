<script lang="ts">
    import Icon from './Icon.svelte'

    let {
        value,
        indent = 2,
        expandEmptyRoot = false
    }: {
        value: unknown
        indent?: number
        expandEmptyRoot?: boolean
    } = $props()
</script>

{#snippet jsonEntry(value: unknown, key: string | undefined, depth: number)}
    {#if typeof value === 'object' && value !== null && !(value instanceof Date)}
        {@const entries = Object.entries(value)}
        {@const isArray = Array.isArray(value)}
        {@const opening = isArray ? '[' : '{'}
        {@const closing = isArray ? ']' : '}'}
        {#if entries.length || (expandEmptyRoot && depth === 0)}
            <details open={depth < 3}>
                <summary>
                    <span class="branch-arrow"><Icon name="PiCaretRight" size={14} /></span>
                    <span>
                        {#if key !== undefined}
                            <span class="json-property">{key}:</span>
                        {/if}
                        <span class="json-bracket"
                            >{opening}<span class="closed-bracket">…{closing}</span></span
                        >
                    </span>
                    <span class="json-count"
                        >{entries.length} {entries.length === 1 ? 'item' : 'items'}</span
                    >
                </summary>
                <div class="json-entries">
                    {#each entries as [name, entry] (name)}
                        {@render jsonEntry(entry, isArray ? undefined : name, depth + 1)}
                    {/each}
                </div>
                <span class="json-bracket branch-end">{closing}</span>
            </details>
        {:else}
            <div class="json-value">
                {#if key !== undefined}
                    <span class="json-property">{key}:</span>
                {/if}
                <span class="json-bracket">{opening}{closing}</span>
            </div>
        {/if}
    {:else}
        <div class="json-value">
            {#if key !== undefined}
                <span class="json-property">{key}:</span>
            {/if}
            <span class={value === null ? 'null' : value instanceof Date ? 'string' : typeof value}>
                {JSON.stringify(value)}
            </span>
        </div>
    {/if}
{/snippet}

<div class="json-report" dir="ltr" style:--json-indent={`${indent}ch`}>
    {@render jsonEntry(value, undefined, 0)}
</div>

<style>
    .json-report {
        overflow-wrap: anywhere;
        padding: 1em 1em 1em 2em;
        border-radius: 0.5em;
        background: var(--gw-json-bg);
        color: var(--gw-json-fg);
        font: var(--mantine-font-size-md) / 1 var(--mantine-font-family-monospace);
    }
    .json-report summary,
    .json-value {
        display: flex;
        align-items: center;
        min-height: 1.7em;
        gap: 0.3em;
    }
    .json-report summary {
        position: relative;
        flex-wrap: wrap;
        cursor: pointer;
        list-style: none;
    }
    .json-report summary::-webkit-details-marker {
        display: none;
    }
    .json-report .branch-arrow {
        position: absolute;
        inset-inline-end: calc(100% + 0.1em);
        color: var(--gw-json-purple);
        display: inline-flex;
    }
    details[open] > summary .branch-arrow {
        transform: rotate(90deg);
    }
    details[open] > summary .closed-bracket {
        display: none;
    }
    .json-entries {
        padding-inline-start: var(--json-indent);
    }
    .branch-end {
        display: block;
    }
    .json-property {
        color: var(--gw-json-property);
    }
    .json-bracket {
        color: var(--gw-json-bracket);
    }
    .json-count {
        color: var(--gw-json-count);
        font-size: 0.85em;
        font-style: italic;
    }
    .string {
        color: var(--gw-json-string);
    }
    .number {
        color: var(--gw-json-purple);
    }
    .boolean {
        color: var(--gw-json-boolean);
        font-size: 90%;
        font-weight: 700;
    }
    .null {
        padding: 0.05em 0.2em;
        border-radius: 0.3em;
        background: var(--gw-json-null-bg);
        color: var(--gw-json-null);
    }
</style>
