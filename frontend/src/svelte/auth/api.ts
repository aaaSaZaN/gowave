import {
    GetPasskeyAuthenticationOptionsCommand,
    GetStatusCommand,
    LoginCommand,
    OAuth2AuthorizeCommand,
    OAuth2CallbackCommand,
    RegisterCommand,
    VerifyPasskeyAuthenticationCommand
} from '@remnawave/backend-contract/build/backend/commands/auth'
import {
    type PublicKeyCredentialRequestOptionsJSON,
    startAuthentication
} from '@simplewebauthn/browser'
import { callCommand, submitCommand } from '../commands'

export function readAuthStatus(signal: AbortSignal) {
    return callCommand(GetStatusCommand, { signal })
}

export function submitCredentials(values: unknown, register: boolean) {
    return submitCommand(register ? RegisterCommand : LoginCommand, values)
}

export function authorizeOAuth(provider: string) {
    return submitCommand(OAuth2AuthorizeCommand, { provider })
}

export function authenticateOAuth(values: unknown) {
    return submitCommand(OAuth2CallbackCommand, values)
}

export async function authenticatePasskey() {
    const options = await callCommand(GetPasskeyAuthenticationOptionsCommand)
    const response = await startAuthentication({
        optionsJSON: options as PublicKeyCredentialRequestOptionsJSON
    })
    return submitCommand(VerifyPasskeyAuthenticationCommand, { response })
}
