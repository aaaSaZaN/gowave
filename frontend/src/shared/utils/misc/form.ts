import { notifications } from '@mantine/notifications'
import { parseFormErrors } from '@shared/lib/form-errors'
import type { ReactNode } from 'react'

export function handleFormErrors(
    form: { setFieldError: (field: string, message: ReactNode) => void },
    errors: unknown
) {
    const { fieldErrors, formErrors } = parseFormErrors(errors)
    for (const [field, message] of Object.entries(fieldErrors)) form.setFieldError(field, message)
    for (const message of formErrors) notifications.show({ message, color: 'red' })
}
