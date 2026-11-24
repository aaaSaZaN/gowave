import {
    SubscriptionPageRawConfigSchema,
    type TSubscriptionPageRawConfig
} from '@remnawave/subscription-page-types'

export type PageConfig = TSubscriptionPageRawConfig
export type PlatformKey = keyof PageConfig['platforms']
export type Platform = NonNullable<PageConfig['platforms'][PlatformKey]>
export type ImportMode = 'full' | 'platforms' | 'baseTranslations' | 'svgLibrary'
export type LocalizedText = PageConfig['baseTranslations']['name']

export const platformLabels = {
    android: 'Android',
    androidTV: 'Android TV',
    appleTV: 'Apple TV',
    ios: 'iOS',
    linux: 'Linux',
    macos: 'macOS',
    windows: 'Windows'
} as const satisfies Record<PlatformKey, string>

export function readPageConfig(input: unknown): PageConfig {
    const parsed = SubscriptionPageRawConfigSchema.parse(input)
    // Keep extensions carried by imported configurations and fill only schema defaults.
    return {
        ...parsed,
        ...(input as PageConfig),
        baseSettings: { ...parsed.baseSettings, ...(input as PageConfig).baseSettings }
    }
}

export function mergePageConfig(
    current: PageConfig,
    incoming: PageConfig,
    mode: ImportMode
): PageConfig {
    if (mode === 'full') return incoming
    if (mode === 'baseTranslations') {
        return {
            ...current,
            locales: incoming.locales,
            baseTranslations: incoming.baseTranslations
        }
    }
    const svgLibrary = { ...current.svgLibrary, ...incoming.svgLibrary }
    if (mode === 'svgLibrary') return { ...current, svgLibrary }

    return {
        ...current,
        locales: incoming.locales,
        baseTranslations: incoming.baseTranslations,
        platforms: incoming.platforms,
        svgLibrary
    }
}

export function localized(locales: PageConfig['locales'], value: string): LocalizedText {
    return Object.fromEntries(locales.map((locale) => [locale, value]))
}
