<script lang="ts">
    import { ROUTES } from '@shared/constants/routes'
    import { panelUrl } from '@shared/lib/panel-path'
    import { parseColoredTextUtil } from '@shared/utils/misc/parse-colored-text'
    import { createQuery } from '@tanstack/svelte-query'
    import { readAuthStatus } from '../auth/api'
    import { p } from '../router'

    const fallback = panelUrl(`favicons/gowave.svg?v=${__BUILD_ID__}`)
    const status = createQuery(() => ({
        queryKey: ['auth', 'getAuthStatus'],
        queryFn: ({ signal }) => readAuthStatus(signal),
        staleTime: 30_000
    }))
    const branding = $derived(status.data?.branding)
    const parts = $derived(parseColoredTextUtil(branding?.title || '{cyan}Go{white}wave'))
</script>

<a class="brand" href={p(ROUTES.DASHBOARD.HOME)}>
    <img
        src={branding?.logoUrl || fallback}
        alt=""
        width={branding?.logoUrl ? 30 : 40}
        height={branding?.logoUrl ? 30 : 40}
        onerror={(event) => {
            const image = event.currentTarget
            if (image.getAttribute('src') !== fallback) image.setAttribute('src', fallback)
        }}
    >
    <span>
        {#each parts as part}
            <span
                style:color={part.color === 'white' || !part.color
                    ? 'var(--mantine-color-bright)'
                    : `var(--mantine-color-${part.color}-text, ${part.color})`}
                >{part.text}</span
            >
        {/each}
    </span>
</a>

<style>
    .brand {
        display: inline-flex;
        flex-shrink: 0;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-text);
        font-family: Unbounded, sans-serif;
        font-size: var(--mantine-font-size-lg);
        font-weight: 700;
        text-decoration: none;
        user-select: none;
        white-space: nowrap;
    }

    .brand img {
        display: block;
        flex-shrink: 0;
        object-fit: contain;
    }
</style>
