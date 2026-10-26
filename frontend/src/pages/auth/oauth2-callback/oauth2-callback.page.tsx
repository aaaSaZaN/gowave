import { Center, Loader, Stack, Text, Title, Transition } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import type { TOAuth2ProvidersKeys } from '@remnawave/backend-contract'
import { useOauth2Callback } from '@shared/api/hooks'
import { ROUTES } from '@shared/constants'
import { useAuth } from '@shared/hooks/use-auth'
import { runAsync } from '@shared/lib/run-async'
import { Page } from '@shared/ui/page'
import { consumeReturnTo } from '@shared/utils/return-to.util'
import { type CSSProperties, useEffect } from 'react'
import { TbCheck } from 'react-icons/tb'
import { useNavigate, useParams, useSearchParams } from 'react-router'

export const Oauth2CallbackPage = () => {
    const { provider } = useParams()
    const { setIsAuthenticated } = useAuth()

    const [searchParams] = useSearchParams()
    const navigate = useNavigate()

    const code = searchParams.get('code')
    const state = searchParams.get('state')

    const { mutate: oauth2Callback, isPending } = useOauth2Callback({
        mutationFns: {
            onSuccess: () => {
                setIsAuthenticated(true)

                runAsync(navigate(consumeReturnTo() ?? ROUTES.DASHBOARD.HOME))
            },
            onError: (error) => {
                notifications.show({
                    title: 'OAuth2 Callback',
                    message: error.message,
                    color: 'red'
                })
                setIsAuthenticated(false)

                runAsync(navigate(ROUTES.AUTH.LOGIN))
            }
        }
    })

    useEffect(() => {
        if (code && state && provider) {
            oauth2Callback({
                variables: {
                    provider: provider as TOAuth2ProvidersKeys,
                    code,
                    state
                }
            })
        }
    }, [code, state, provider, oauth2Callback])

    return (
        <Page title="OAuth2 Authentication">
            <Center style={{ minHeight: '60vh' }}>
                <Stack align="center" gap="xl">
                    <Transition
                        duration={300}
                        mounted={true}
                        timingFunction="ease"
                        transition="fade"
                    >
                        {(styles: CSSProperties) => (
                            <div style={styles}>
                                {isPending ? (
                                    <Loader size="xl" variant="dots" />
                                ) : (
                                    <TbCheck color="teal" size={48} />
                                )}
                            </div>
                        )}
                    </Transition>
                    <div>
                        <Title mb="md" order={2} ta="center">
                            Authenticating...
                        </Title>
                        <Text c="dimmed" ta="center">
                            Verifying credentials...
                        </Text>
                    </div>
                </Stack>
            </Center>
        </Page>
    )
}

export default Oauth2CallbackPage
