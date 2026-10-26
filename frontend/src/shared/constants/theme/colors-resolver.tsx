import {
    defaultVariantColorsResolver,
    parseThemeColor,
    rgba,
    type VariantColorsResolver
} from '@mantine/core'

export const variantColorResolver: VariantColorsResolver = (input) => {
    const defaultResolvedColors = defaultVariantColorsResolver(input)

    if (input.variant === 'soft') {
        const parsed = parseThemeColor({
            color: input.color || 'gray',
            theme: input.theme,
            colorScheme: 'dark'
        })
        const palette = input.theme.colors[parsed.color]
        const c1 = palette?.[6] ?? parsed.value
        const c2 = palette?.[7] ?? parsed.value
        return {
            background: `linear-gradient(135deg, ${rgba(c1, 0.15)} 0%, ${rgba(c2, 0.1)} 100%)`,
            border: `1px solid ${rgba(c1, 0.3)}`,
            color: `var(--mantine-color-${parsed.color}-4)`,
            hover: rgba(palette?.[4] ?? parsed.value, 0.1)
        }
    }

    return defaultResolvedColors
}
