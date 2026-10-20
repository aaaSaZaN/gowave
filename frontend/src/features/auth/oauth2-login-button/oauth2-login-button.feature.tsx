import { Button, Stack } from '@mantine/core'
import { TOAuth2ProvidersKeys } from '@remnawave/backend-contract'
import { useState } from 'react'
import { BiLogoGithub, BiLogoTelegram } from 'react-icons/bi'
import { SiKeycloak } from 'react-icons/si'
import { TbKey } from 'react-icons/tb'

import { useOAuth2Authorize } from '@shared/api/hooks'

import { IProps } from './interfaces/props.interface'

export const OAuth2LoginButtonsFeature = (props: IProps) => {
    const { authentication } = props
    const [loadingProvider, setLoadingProvider] = useState<null | TOAuth2ProvidersKeys>(null)

    const { mutate: oauth2Authorize } = useOAuth2Authorize({
        mutationFns: {
            onSuccess: (data) => {
                if (data.authorizationUrl) {
                    window.location.assign(data.authorizationUrl)
                }

                setTimeout(() => {
                    setLoadingProvider(null)
                }, 1000)
            },
            onError: () => {
                setLoadingProvider(null)
            }
        }
    })

    const handleOAuth2Login = (provider: TOAuth2ProvidersKeys) => {
        setLoadingProvider(provider)
        oauth2Authorize({
            variables: {
                provider
            }
        })
    }

    return (
        <Stack>
            {authentication.oauth2.providers.telegram && (
                <Button
                    color="#0088cc"
                    leftSection={<BiLogoTelegram color="white" size={20} />}
                    loaderProps={{ type: 'dots' }}
                    loading={loadingProvider === 'telegram'}
                    onClick={() => handleOAuth2Login('telegram')}
                    variant="filled"
                >
                    Telegram
                </Button>
            )}

            {authentication.oauth2.providers.pocketid && (
                <Button
                    color="dark"
                    leftSection={
                        <svg
                            height={20}
                            viewBox="0 0 1015 1015"
                            width={20}
                            xmlns="http://www.w3.org/2000/svg"
                        >
                            <path
                                d="M506.6,0c209.52,0,379.98,170.45,379.98,379.96,0,82.33-25.9,160.68-74.91,226.54-48.04,64.59-113.78,111.51-190.13,135.71l-21.1,6.7-50.29-248.04,13.91-6.73c45.41-21.95,74.76-68.71,74.76-119.11,0-72.91-59.31-132.23-132.21-132.23s-132.23,59.32-132.23,132.23c0,50.4,29.36,97.16,74.77,119.11l13.65,6.61-81.01,499.24h-226.36V0h351.18Z"
                                fill="white"
                            />
                        </svg>
                    }
                    loaderProps={{ type: 'dots' }}
                    loading={loadingProvider === 'pocketid'}
                    onClick={() => handleOAuth2Login('pocketid')}
                    variant="filled"
                >
                    PocketID
                </Button>
            )}

            {authentication.oauth2.providers.github && (
                <Button
                    color="#24292e"
                    leftSection={<BiLogoGithub color="white" size={20} />}
                    loaderProps={{ type: 'dots' }}
                    loading={loadingProvider === 'github'}
                    onClick={() => handleOAuth2Login('github')}
                    variant="filled"
                >
                    GitHub
                </Button>
            )}

            {authentication.oauth2.providers.keycloak && (
                <Button
                    color="#000000"
                    leftSection={<SiKeycloak size={20} />}
                    loaderProps={{ type: 'dots' }}
                    loading={loadingProvider === 'keycloak'}
                    onClick={() => handleOAuth2Login('keycloak')}
                    variant="filled"
                >
                    Keycloak
                </Button>
            )}

            {authentication.oauth2.providers.generic && (
                <Button
                    color="#000000"
                    leftSection={<TbKey size={20} />}
                    loaderProps={{ type: 'dots' }}
                    loading={loadingProvider === 'generic'}
                    onClick={() => handleOAuth2Login('generic')}
                    variant="filled"
                >
                    OAuth2
                </Button>
            )}
        </Stack>
    )
}
