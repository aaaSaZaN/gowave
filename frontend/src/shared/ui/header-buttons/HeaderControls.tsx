import { type BoxProps, Group } from '@mantine/core'
import { optionalProp } from '@shared/lib/optional-prop'

import { GithubControl } from './GithubControl'
import { LanguageControl } from './LanguageControl'
import { LogoutControl } from './LogoutControl'
import { RecapControl } from './RecapControl'
import { VersionControl } from './VersionControl'

interface HeaderControlsProps extends BoxProps {
    githubLink?: string
    isGithubLoading?: boolean
    stars?: number
    withGithub?: boolean
    withLanguage?: boolean
    withLogout?: boolean
    withRecap?: boolean
    withVersion?: boolean
}

export function HeaderControls({
    githubLink,
    withGithub = true,
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
            {withVersion && <VersionControl />}
            {withGithub && githubLink && (
                <GithubControl
                    {...optionalProp('isLoading', isGithubLoading)}
                    link={githubLink}
                    {...optionalProp('stars', stars)}
                />
            )}
            {withRecap && <RecapControl />}
            {withLanguage && <LanguageControl />}
            {withLogout && <LogoutControl />}
        </Group>
    )
}
