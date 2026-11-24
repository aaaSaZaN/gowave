import { createInstance } from 'i18next'

import { languages } from '../app/i18n/languages'
import { languageDetector, localeBackend } from '../app/i18n/plugins'

const i18n = createInstance()
let language = $state('en')
let ready = $state(false)

i18n.on('languageChanged', (value) => {
    language = value
})

void i18n
    .use(languageDetector)
    .use(localeBackend)
    .init({
        fallbackLng: 'en',
        debug: import.meta.env.DEV,
        defaultNS: 'gowave',
        ns: ['gowave'],
        load: 'languageOnly',
        supportedLngs: languages.map(({ value }) => value),
        preload: languages.map(({ value }) => value),
        interpolation: { escapeValue: false }
    })
    .then(() => {
        ready = true
    })

export const locale = {
    get ready() {
        return ready
    },
    get language() {
        return language
    },
    get direction() {
        return i18n.dir(language)
    },
    get t() {
        return i18n.getFixedT(language)
    },
    setLanguage(value: string) {
        return i18n.changeLanguage(value)
    }
}
