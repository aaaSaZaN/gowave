import type { GetNodesCommand } from '@remnawave/backend-contract'

export interface IProps {
    node: GetNodesCommand.Response['response'][number] | undefined
}
