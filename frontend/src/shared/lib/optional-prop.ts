// React treats an omitted optional prop and an undefined value alike. Third-party
// declarations do not always model both forms; omit only undefined at that boundary.
export function optionalProp<Key extends string, Value>(
    key: Key,
    value: Value
): { [Name in Key]?: Exclude<Value, undefined> } {
    return (value === undefined ? {} : { [key]: value }) as {
        [Name in Key]?: Exclude<Value, undefined>
    }
}
