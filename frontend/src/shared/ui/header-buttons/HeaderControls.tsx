import { BoxProps, Group } from '@mantine/core'

import { GithubControl } from './GithubControl'
import { LanguageControl } from './LanguageControl'
import { LogoutControl } from './LogoutControl'
import { RecapControl } from './RecapControl'
import { SupportControl } from './SupportControl'
import { VersionControl } from './VersionControl'

interface HeaderControlsProps extends BoxProps {
    githubLink?: string
    isGithubLoading?: boolean
    stars?: number
    withGithub?: boolean
    withLanguage?: boolean
    withLogout?: boolean
    withRecap?: boolean
    withSupport?: boolean
    withVersion?: boolean
}

export function HeaderControls({
    githubLink,
    withGithub = true,
    withSupport = true,
    withLogout = true,
    withLanguage = true,
    withVersion = true,
    withRecap = false,
    stars,
    isGithubLoading,
    ...others
}: HeaderControlsProps) {
    return (
        <Group gap="xs" {...others}>
            {withSupport && <SupportControl />}

            {withVersion && <VersionControl />}
            {withGithub && (
                <GithubControl isLoading={isGithubLoading} link={githubLink!} stars={stars} />
            )}
            {withRecap && <RecapControl />}
            {withLanguage && <LanguageControl />}
            {withLogout && <LogoutControl />}
        </Group>
    )
}
