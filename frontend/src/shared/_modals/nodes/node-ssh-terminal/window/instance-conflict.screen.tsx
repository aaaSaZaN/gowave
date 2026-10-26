import { Center } from '@mantine/core'
import { EmptyPageLayout } from '@shared/ui/layouts/empty-page'
import { TbBrowserX } from 'react-icons/tb'

export const InstanceConflictScreen = () => (
    <Center flex={1} p="md">
        <EmptyPageLayout icon={<TbBrowserX size={32} />} title="Already open in another tab" />
    </Center>
)
