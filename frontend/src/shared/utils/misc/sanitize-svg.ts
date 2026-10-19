import DOMPurify from 'dompurify'

// SVG library entries are user-controlled config data; strip scripts, event
// handlers and foreign content before injecting them into the admin panel.
export const sanitizeSvg = (svg: string): string =>
    DOMPurify.sanitize(svg, {
        USE_PROFILES: { svg: true, svgFilters: true }
    })
