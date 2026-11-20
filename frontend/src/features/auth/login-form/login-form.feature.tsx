import { Button, Container, Paper, PasswordInput, TextInput } from '@mantine/core'
import { useForm } from '@mantine/form'
import { LoginCommand } from '@remnawave/backend-contract'
import { useLogin } from '@shared/api/hooks'
import { handleFormErrors } from '@shared/utils/misc'
import type { SubmitEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { PiSignInDuotone } from 'react-icons/pi'

export const LoginFormFeature = () => {
    const { t } = useTranslation()

    const form = useForm({
        mode: 'uncontrolled',
        initialValues: { username: '', password: '' }
    })

    const { mutate: login, isPending: isLoading } = useLogin()

    const handleSubmit = (event: SubmitEvent<HTMLFormElement>) => {
        event.preventDefault()
        // Password managers can fill these inputs without dispatching a change event.
        const fields = new FormData(event.currentTarget)
        const result = LoginCommand.RequestBodySchema.safeParse(Object.fromEntries(fields))
        if (!result.success) {
            handleFormErrors(form, result.error)
            return
        }
        login({ variables: result.data }, { onError: (error) => handleFormErrors(form, error) })
    }

    return (
        <form onSubmit={handleSubmit}>
            <Container size="100%">
                <Paper>
                    <TextInput
                        autoComplete="username"
                        label={t('common.field.username')}
                        name="username"
                        placeholder={t('common.field.username')}
                        required
                        {...form.getInputProps('username')}
                    />
                    <PasswordInput
                        autoComplete="current-password"
                        label={t('common.field.password')}
                        mt="md"
                        name="password"
                        placeholder={t('login-form.feature.your-password')}
                        required
                        {...form.getInputProps('password')}
                    />
                    <Button
                        fullWidth
                        leftSection={<PiSignInDuotone size="16px" />}
                        loading={isLoading}
                        mt="xl"
                        type="submit"
                        variant="default"
                    >
                        {t('login-form.feature.sign-in')}
                    </Button>
                </Paper>
            </Container>
        </form>
    )
}
