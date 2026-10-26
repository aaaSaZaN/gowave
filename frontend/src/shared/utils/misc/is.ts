import { z } from 'zod'

const uuidSchema = z.uuid()

export function isDefined<T>(value: T): value is NonNullable<T> {
    return value !== null && value !== undefined && value !== ''
}

export function isValidUuid(value: string): boolean {
    return uuidSchema.safeParse(value).success
}
