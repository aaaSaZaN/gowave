import type { HTMLAttributes } from 'react'

interface CountryFlagProps extends HTMLAttributes<HTMLSpanElement> {
    countryCode: string
}

export default function CountryFlag({ countryCode, style, ...props }: CountryFlagProps) {
    if (!/^[a-z]{2}$/i.test(countryCode)) return null
    const code = countryCode.toUpperCase()
    const flag = String.fromCodePoint(...Array.from(code, (char) => 127397 + char.charCodeAt(0)))
    return (
        <span
            role="img"
            aria-label={code}
            {...props}
            style={{ display: 'inline-block', lineHeight: '1em', ...style }}
        >
            {flag}
        </span>
    )
}
