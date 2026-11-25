import { MediaQuery } from 'svelte/reactivity'

// Preserve the theme selected by an existing React dashboard.
const COLOR_SCHEME_KEY = 'mantine-color-scheme-value'
type ColorScheme = 'auto' | 'dark' | 'light'
const systemDark = new MediaQuery('(prefers-color-scheme: dark)')

function restoreTheme(): ColorScheme {
    try {
        const value = localStorage.getItem(COLOR_SCHEME_KEY)
        if (value === 'light' || value === 'auto') return value
    } catch {
        // The dashboard still works when browser storage is disabled.
    }
    return 'dark'
}

let preference = $state(restoreTheme())
const scheme = $derived(
    preference === 'auto' ? (systemDark.current ? 'dark' : 'light') : preference
)

export const theme = {
    get scheme() {
        return scheme
    },
    toggle() {
        preference = scheme === 'dark' ? 'light' : 'dark'
        try {
            localStorage.setItem(COLOR_SCHEME_KEY, preference)
        } catch {
            // The new theme remains active for this page.
        }
    }
}
