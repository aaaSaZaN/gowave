import { sanitizeSvg } from '@shared/utils/misc/sanitize-svg'
import { type HTMLAttributes, useLayoutEffect, useRef } from 'react'

interface SafeSvgProps
    extends Omit<HTMLAttributes<HTMLElement>, 'children' | 'dangerouslySetInnerHTML'> {
    svg: string
    inline?: boolean
}

export function SafeSvg({ svg, inline = false, ...props }: SafeSvgProps) {
    const container = useRef<HTMLDivElement>(null)
    useLayoutEffect(() => {
        const target = container.current
        if (!target) return
        // HTML parsing assigns the SVG namespace even when an uploaded icon omits
        // xmlns. Import only the sanitized SVG root, never arbitrary HTML siblings.
        const parsed = new DOMParser().parseFromString(sanitizeSvg(svg), 'text/html')
        const icon = parsed.body.firstElementChild
        target.replaceChildren()
        if (icon?.localName === 'svg' && icon.namespaceURI === 'http://www.w3.org/2000/svg') {
            target.appendChild(document.importNode(icon, true))
        }
    }, [svg])
    return inline ? <span {...props} ref={container} /> : <div {...props} ref={container} />
}
