import { readLegacyLayout, writeLegacyLayout } from './view-preferences'

let legacy = $state(readLegacyLayout())

export const layout = {
    get legacy() {
        return legacy
    },
    setLegacy(value: boolean) {
        legacy = value
        writeLegacyLayout(value)
    }
}
