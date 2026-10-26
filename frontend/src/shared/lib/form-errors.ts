import { isRecord } from './guards'

export function parseFormErrors(value: unknown) {
    const fieldErrors: Record<string, string> = {}
    const formErrors: string[] = []
    if (!isRecord(value)) return { fieldErrors, formErrors }
    const issues = value['issues'] ?? value['errors']
    if (Array.isArray(issues)) {
        for (const issue of issues) {
            if (!isRecord(issue) || typeof issue['message'] !== 'string') continue
            const path = issue['path']
            if (
                Array.isArray(path) &&
                path.length &&
                path.every((part) => typeof part === 'string' || typeof part === 'number')
            ) {
                fieldErrors[path.join('.')] = issue['message']
            } else {
                formErrors.push(issue['message'])
            }
        }
        return { fieldErrors, formErrors }
    }
    if (Array.isArray(value['formErrors'])) {
        formErrors.push(
            ...value['formErrors'].filter(
                (message): message is string => typeof message === 'string'
            )
        )
    }
    if (isRecord(value['fieldErrors'])) {
        for (const [field, messages] of Object.entries(value['fieldErrors'])) {
            if (!Array.isArray(messages)) continue
            fieldErrors[field] = messages
                .filter((message): message is string => typeof message === 'string')
                .join(',')
        }
    }
    return { fieldErrors, formErrors }
}
