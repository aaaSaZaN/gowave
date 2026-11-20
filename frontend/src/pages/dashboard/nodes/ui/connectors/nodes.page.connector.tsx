import {
    useGetConfigProfiles,
    useGetNodePlugins,
    useGetNodeSecretKey,
    useGetNodes,
    useGetNodesTags
} from '@shared/api/hooks'

import NodesPageComponent from '../components/nodes.page.component'

export function NodesPageConnector() {
    const { data: nodes, isLoading } = useGetNodes()
    const { data: nodePlugins, isLoading: isNodePluginsLoading } = useGetNodePlugins()
    const { isLoading: isConfigProfilesLoading } = useGetConfigProfiles()

    useGetNodeSecretKey()
    useGetNodePlugins()
    useGetNodesTags()

    return (
        <NodesPageComponent
            isLoading={isLoading || isConfigProfilesLoading || isNodePluginsLoading || !nodePlugins}
            nodes={nodes}
            nodePlugins={nodePlugins}
        />
    )
}
