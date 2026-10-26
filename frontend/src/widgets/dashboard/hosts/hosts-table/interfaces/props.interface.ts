import type { UseListStateHandlers } from '@mantine/hooks'
import type { GetConfigProfilesCommand, GetHostsCommand } from '@remnawave/backend-contract'
import type { RefObject } from 'react'

export interface IProps {
    configProfiles: GetConfigProfilesCommand.Response['response']['configProfiles'] | undefined
    handlers: UseListStateHandlers<GetHostsCommand.Response['response'][number]>
    hosts: GetHostsCommand.Response['response'] | undefined
    isDraggingRef: RefObject<boolean>
    selectedHosts: string[]
    setSelectedHosts: React.Dispatch<React.SetStateAction<string[]>>
    state: GetHostsCommand.Response['response']
}
