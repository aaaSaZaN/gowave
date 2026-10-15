import {
    REMNAWAVE_BYPASS_HTTPS_RESTRCTIONS,
    REMNAWAVE_CLIENT_TYPE_BROWSER,
    REMNAWAVE_CLIENT_TYPE_HEADER
} from '@remnawave/backend-contract'
import axios from 'axios'
import consola from 'consola/browser'

import { logoutEvents } from '../emitters/emit-logout'

let authorizationToken = ''

let BASE_DOMAIN = __DOMAIN_BACKEND__
const isDev = __NODE_ENV__ === 'development'
const isDomainOverride = __DOMAIN_OVERRIDE__ === '1'

if (isDev) {
    BASE_DOMAIN = __DOMAIN_BACKEND__
} else {
    BASE_DOMAIN = window.location.origin
}

if (isDomainOverride) {
    BASE_DOMAIN = __DOMAIN_BACKEND__
}

export const getBackendDomain = () => BASE_DOMAIN

export const instance = axios.create({
    baseURL: BASE_DOMAIN,
    headers: {
        'Content-type': 'application/json',
        Accept: 'application/json',
        [REMNAWAVE_CLIENT_TYPE_HEADER]: REMNAWAVE_CLIENT_TYPE_BROWSER,
        ...REMNAWAVE_BYPASS_HTTPS_RESTRCTIONS
    }
})

instance.interceptors.request.use((config) => {
    config.headers.set('Authorization', `Bearer ${authorizationToken}`)
    config.headers.set(REMNAWAVE_CLIENT_TYPE_HEADER, REMNAWAVE_CLIENT_TYPE_BROWSER)
    for (const [key, value] of Object.entries(REMNAWAVE_BYPASS_HTTPS_RESTRCTIONS)) {
        config.headers.set(key, value)
    }
    return config
})

export const setAuthorizationToken = (token: string) => {
    authorizationToken = token
}

export const hasAuthorizationToken = () => authorizationToken !== ''
export const getAuthorizationToken = () => authorizationToken

instance.interceptors.response.use(
    (response) => {
        return response
    },
    (error) => {
        if (error.response?.status === 401) {
            consola.error('Received 401 Unauthorized from backend. Logging out.')
            logoutEvents.emit()
        }
        return Promise.reject(error)
    }
)
