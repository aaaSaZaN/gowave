type Definition = { queryKey: readonly unknown[] | null }

type QueryEntry<Prefix extends readonly string[], Entry> = Entry extends (
    ...args: infer Args
) => Definition
    ? ((...args: Args) => { queryKey: readonly [...Prefix, ...unknown[]] }) & { _def: Prefix }
    : { queryKey: Prefix; _def: Prefix }

type QueryGroup<Name extends string, Schema> = {
    _def: readonly [Name]
} & {
    [Key in keyof Schema]: QueryEntry<readonly [Name, Key & string], Schema[Key]>
}

export function createQueryKeys<
    const Name extends string,
    Schema extends { [Key in keyof Schema]: Definition | ((...args: never[]) => Definition) }
>(name: Name, schema: Schema): QueryGroup<Name, Schema> {
    const result: Record<string, unknown> = { _def: [name] }
    for (const key of Object.keys(schema)) {
        const entry = schema[key as keyof Schema]
        const prefix = [name, key] as const
        result[key] =
            typeof entry === 'function'
                ? Object.assign(
                      (...args: never[]) => ({
                          queryKey: [...prefix, ...(entry(...args).queryKey ?? [])]
                      }),
                      { _def: prefix }
                  )
                : { queryKey: [...prefix, ...(entry.queryKey ?? [])], _def: prefix }
    }
    return result as QueryGroup<Name, Schema>
}

type Merge<Groups extends readonly unknown[]> = {
    [
        Group in Groups[number] as Group extends { _def: readonly [infer Name extends string] }
            ? Name
            : never
    ]: Group
}

export function mergeQueryKeys<const Groups extends readonly { _def: readonly [string] }[]>(
    ...groups: Groups
): Merge<Groups> {
    return Object.fromEntries(groups.map((group) => [group._def[0], group])) as Merge<Groups>
}
