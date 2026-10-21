import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import HttpApi from 'i18next-http-backend'
import { initReactI18next } from 'react-i18next'


i18n.use(initReactI18next)
    .use(LanguageDetector)
    .use(HttpApi)
    .init({
        fallbackLng: 'en',
        debug: process.env.NODE_ENV === 'development',
        defaultNS: ['gowave'],
        ns: ['gowave'],
        detection: {
            order: ['localStorage', 'navigator', 'htmlTag', 'path', 'subdomain'],
            convertDetectedLanguage: (lng) => (lng.includes('-') ? lng.split('-')[0] : lng)
        },
        load: 'languageOnly',
        preload: ['en', 'ru', 'fa', 'zh'],
        backend: {
            loadPath: '/locales/{{lng}}/{{ns}}.json',
            // Locale URLs carry no hash, so a new build must not reuse cached files.
            queryStringParams: { v: __BUILD_ID__ }
        },
        interpolation: {
            escapeValue: false
        },
        react: {
            useSuspense: true
        }
    })

export default i18n
