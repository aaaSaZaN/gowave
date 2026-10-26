import type { GetSubpageConfigsCommand } from '@remnawave/backend-contract'
import { ROUTES } from '@shared/constants'
import { runAsync } from '@shared/lib/run-async'
import { UniversalSpotlightContentShared } from '@shared/ui/universal-spotlight'
import { TbFile } from 'react-icons/tb'
import { generatePath, useNavigate } from 'react-router'

interface IProps {
    configs: GetSubpageConfigsCommand.Response['response']['configs']
}

export const SubpageConfigsSpotlightWidget = (props: IProps) => {
    const { configs } = props

    const navigate = useNavigate()

    const handleViewSubscriptionPageConfig = (subscriptionPageConfigUuid: string) => {
        runAsync(
            navigate(
                generatePath(ROUTES.DASHBOARD.SUBPAGE_CONFIGS.SUBPAGE_CONFIG_BY_UUID, {
                    uuid: subscriptionPageConfigUuid
                })
            )
        )
    }

    return (
        <UniversalSpotlightContentShared
            actions={configs.map((config) => ({
                label: config.name,
                id: config.uuid,
                leftSection: <TbFile size={24} />,

                onClick: () => handleViewSubscriptionPageConfig(config.uuid)
            }))}
        />
    )
}
