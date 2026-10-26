declare global {
    interface Window {
        Go?: new () => {
            importObject: WebAssembly.Imports
            run(instance: WebAssembly.Instance): Promise<void>
        }
        onWasmInitialized?: () => void

        XrayParseConfig?: (config: string) => null | string
    }
}

export {}
