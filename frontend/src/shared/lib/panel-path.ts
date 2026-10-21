// The server supplies the base before modules execute; Vite development uses /.
export const panelBasePath = new URL(document.baseURI).pathname

export const panelUrl = (path: string) => `${panelBasePath}${path.replace(/^\//, '')}`
