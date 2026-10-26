export const faviconResolver = (faviconLink: null | string) => {
    if (!faviconLink) return null
    try {
        const url = new URL(
            faviconLink.startsWith('//')
                ? `https:${faviconLink}`
                : /^[\w+.-]+:\/\//.test(faviconLink)
                  ? faviconLink
                  : `https://${faviconLink}`
        )
        if (!['http:', 'https:'].includes(url.protocol) || !url.hostname) return null
        return `https://www.google.com/s2/favicons?sz=64&domain_url=${url.protocol}//${url.host}`
    } catch {
        return null
    }
}
