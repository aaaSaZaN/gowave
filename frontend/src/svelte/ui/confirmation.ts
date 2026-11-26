import { createContext, onDestroy } from 'svelte'

export interface ConfirmOptions {
    title?: string
    label?: string
    tone?: 'red' | 'orange'
}

export interface Confirmation {
    owner: symbol
    message: string
    options: ConfirmOptions
    resolve: (confirmed: boolean) => void
}

const [get, set] = createContext<{
    ask: (owner: symbol, message: string, options: ConfirmOptions) => Promise<boolean>
    cancel: (owner: symbol) => void
}>()

export const confirmContext = { set }

export function useConfirmation() {
    const confirmation = get()
    const owner = Symbol()
    onDestroy(() => confirmation.cancel(owner))
    return (message: string, options: ConfirmOptions = {}) =>
        confirmation.ask(owner, message, options)
}
