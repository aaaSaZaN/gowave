import { panelUrl } from '@shared/lib/panel-path'
import type { BackendModule, LanguageDetectorModule } from 'i18next'

export const languageDetector: LanguageDetectorModule = {
    type: 'languageDetector',
    detect() {
        let cached: string | null = null
        try {
            cached = localStorage.getItem('i18nextLng')
        } catch {
            // Storage can be disabled independently of the rest of the browser.
        }
        return [
            cached,
            ...navigator.languages,
            navigator.language,
            document.documentElement.lang,
            window.location.pathname.split('/').filter(Boolean)[0],
            window.location.hostname.split('.')[0]
        ]
            .filter((language): language is string => Boolean(language))
            .map((language) => language.split('-')[0] ?? language)
    },
    cacheUserLanguage(language) {
        if (language === 'cimode') return
        try {
            localStorage.setItem('i18nextLng', language)
        } catch {
            // Detection still works when preferences cannot be persisted.
        }
    }
}

export const localeBackend: BackendModule = {
    type: 'backend',
    init() {},
    read(language, namespace, callback) {
        const path = panelUrl(
            `/locales/${encodeURIComponent(language)}/${encodeURIComponent(namespace)}.json`
        )
        fetch(`${path}?v=${encodeURIComponent(__BUILD_ID__)}`).then(
            async (response) => {
                if (!response.ok) {
                    callback(
                        new Error(`Locale request failed: ${response.status}`),
                        response.status >= 500
                    )
                    return
                }
                let resource: unknown
                try {
                    resource = await response.json()
                } catch (error) {
                    callback(error instanceof Error ? error : new Error(String(error)), false)
                    return
                }
                if (typeof resource !== 'object' || resource === null || Array.isArray(resource)) {
                    callback(new Error('Locale resource must be an object'), false)
                    return
                }
                callback(null, resource)
            },
            (error: unknown) =>
                callback(error instanceof Error ? error : new Error(String(error)), true)
        )
    }
}
