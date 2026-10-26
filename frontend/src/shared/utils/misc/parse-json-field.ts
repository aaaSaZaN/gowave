import consola from '@shared/lib/logger'

export const parseJsonField = (value: unknown): null | object | string => {
    if (value === '') return null
    if (typeof value !== 'string') return null

    try {
        const parsed: unknown = JSON.parse(value)
        return typeof parsed === 'string' || (typeof parsed === 'object' && parsed !== null)
            ? parsed
            : null
    } catch (error) {
        consola.error(error)
        return null
    }
}

export const stringifyJsonField = (value: unknown): string => {
    if (typeof value === 'object' && value !== null) {
        return JSON.stringify(value, null, 2)
    }

    return ''
}
