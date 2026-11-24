import type {
    GetSubscriptionTemplateCommand,
    TSubscriptionTemplateType
} from '@remnawave/backend-contract'
import { decode, encode } from '@shared/lib/base64'

export const templateTypes = {
    MIHOMO: { title: 'Mihomo', language: 'yaml' },
    CLASH: { title: 'Clash', language: 'yaml' },
    STASH: { title: 'Stash', language: 'yaml' },
    SINGBOX: { title: 'Singbox', language: 'json' },
    XRAY_JSON: { title: 'Xray JSON', language: 'json' }
} as const satisfies Partial<
    Record<TSubscriptionTemplateType, { title: string; language: 'json' | 'yaml' }>
>
export type TemplateType = keyof typeof templateTypes
export const templateLogos = {
    MIHOMO: 'MihomoLogo',
    CLASH: 'MihomoLogo',
    STASH: 'StashLogo',
    SINGBOX: 'SingboxLogo',
    XRAY_JSON: 'XrayLogo'
} as const satisfies Record<TemplateType, string>
export type Template = GetSubscriptionTemplateCommand.Response['response']

export function templateType(value: string | undefined): TemplateType | undefined {
    return value && Object.hasOwn(templateTypes, value) ? (value as TemplateType) : undefined
}

export function templateText(template: Template, type: TemplateType): string {
    if (templateTypes[type].language === 'json')
        return JSON.stringify(template.templateJson ?? {}, null, 2)
    return template.encodedTemplateYaml
        ? new TextDecoder().decode(decode(template.encodedTemplateYaml))
        : ''
}

export function templateContent(text: string, type: TemplateType) {
    if (templateTypes[type].language === 'yaml') {
        return { encodedTemplateYaml: encode(new TextEncoder().encode(text)) }
    }
    const templateJson: unknown = JSON.parse(text)
    return { templateJson }
}
