import type { GetNodePluginsCommand, GetNodesCommand } from '@remnawave/backend-contract'

export interface IProps {
    nodes: GetNodesCommand.Response['response'] | undefined
    nodePlugins: GetNodePluginsCommand.Response['response'] | undefined
}
