import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import { languageDetector, localeBackend } from './plugins'

i18n.use(initReactI18next)
    .use(languageDetector)
    .use(localeBackend)
    .init({
        fallbackLng: 'en',
        debug: import.meta.env.DEV,
        defaultNS: ['gowave'],
        ns: ['gowave'],
        load: 'languageOnly',
        preload: ['en', 'ru', 'fa', 'zh'],
        interpolation: {
            escapeValue: false
        },
        react: {
            useSuspense: true
        }
    })

export default i18n
