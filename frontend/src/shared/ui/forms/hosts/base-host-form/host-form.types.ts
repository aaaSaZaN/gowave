import type { UseFormReturnType } from '@mantine/form'
import type {
    CreateHostCommand,
    UpdateHostCommand,
    UpdateManyHostsCommand
} from '@remnawave/backend-contract'

type HostRequest =
    | CreateHostCommand.RequestBody
    | UpdateHostCommand.RequestBody
    | UpdateManyHostsCommand.RequestBody
// Preserve each endpoint's optional value types when the shared form combines them.
type HostField<Key, Request = HostRequest> = Request extends unknown
    ? Key extends keyof Request
        ? Request[Key]
        : never
    : never
export type THostFormValues = {
    [Key in
        | keyof CreateHostCommand.RequestBody
        | keyof UpdateHostCommand.RequestBody
        | keyof UpdateManyHostsCommand.RequestBody]?: HostField<Key>
}

export type THostForm = UseFormReturnType<THostFormValues>
