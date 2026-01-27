import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import type { BackendModule } from 'i18next'

const LazyImportPlugin: BackendModule = {
    type: 'backend',
    init() {},
    read: async (language, namespace, callback) => {
        try {
            console.log(language, namespace)
            const url = `/admin/ep_admin_pads/${language}` + ".json"

            const res = await fetch(url, { cache: 'force-cache' })

            if (!res.ok) {
                return callback(
                    new Error(`HTTP ${res.status}: ${url}`),
                    null
                )
            }

            const json = await res.json()
            callback(null, json)
        } catch (err) {
            callback(err as Error, null)
        }
    },
}

i18n
    .use(LanguageDetector)
    .use(LazyImportPlugin)
    .use(initReactI18next)
    .init({
        ns: ['translation', 'ep_admin_pads'],
        defaultNS: 'translation',
        fallbackLng: 'en',
        interpolation: {
            escapeValue: false,
        },
    })

export default i18n