import { SvelteSet } from 'svelte/reactivity'

export const userSelection = new SvelteSet<number>()

export function togglePageSelection(rows: readonly { id: number }[]): void {
    const deselect = rows.every(({ id }) => userSelection.has(id))
    for (const { id } of rows) {
        if (deselect) userSelection.delete(id)
        else userSelection.add(id)
    }
}
