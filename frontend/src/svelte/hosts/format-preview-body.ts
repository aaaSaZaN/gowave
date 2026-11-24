import { applyEdits, format, type ParseError, parseTree } from 'jsonc-parser'

export function formatPreviewBody(body: string, contentType: string): string {
    if (!/^application\/(?:[\w.-]+\+)?json(?:\s*;|$)/i.test(contentType)) return body

    const errors: ParseError[] = []
    const tree = parseTree(body, errors, { disallowComments: true })
    if (!tree || errors.length > 0) return body

    return applyEdits(body, format(body, undefined, { insertSpaces: true, tabSize: 2 }))
}
