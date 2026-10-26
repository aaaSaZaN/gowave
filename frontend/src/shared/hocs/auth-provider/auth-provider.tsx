import { removeToken, useToken } from '@entities/auth'
import { logoutEvents } from '@shared/emitters'
import { resetAllStores } from '@shared/hocs/store-wrapper'
import {
    createContext,
    type ReactNode,
    useEffect,
    useEffectEvent,
    useMemo,
    useRef,
    useState
} from 'react'

interface AuthContextValues {
    isAuthenticated: boolean
    isInitialized: boolean
    setIsAuthenticated: (isAuthenticated: boolean) => void
}
export const AuthContext = createContext<AuthContextValues | null>(null)

interface AuthProviderProps {
    children: ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
    const token = useToken()
    const [isAuthenticated, setIsAuthenticated] = useState(Boolean(token))
    const [previousToken, setPreviousToken] = useState(token)
    const loggingOut = useRef(false)
    if (previousToken !== token) {
        setPreviousToken(token)
        setIsAuthenticated(Boolean(token))
    }

    const logoutUser = useEffectEvent(() => {
        if (loggingOut.current) return
        loggingOut.current = true
        try {
            setIsAuthenticated(false)
            removeToken()
            resetAllStores()
        } finally {
            loggingOut.current = false
        }
    })

    useEffect(() => logoutEvents.subscribe(logoutUser), [])

    // The token store hydrates synchronously from localStorage.
    const value = useMemo(
        () => ({ isAuthenticated, isInitialized: true, setIsAuthenticated }),
        [isAuthenticated]
    )
    return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
