export function nextComboboxOption(
    key: 'ArrowDown' | 'ArrowUp',
    current: string | undefined,
    options: readonly { value: string }[]
): string | undefined {
    if (!options.length) return undefined
    const index = options.findIndex(({ value }) => value === current)
    if (key === 'ArrowDown') return options[(index + 1) % options.length]?.value
    return options[index <= 0 ? options.length - 1 : index - 1]?.value
}
