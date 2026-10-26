import { AuthContext } from '@shared/hocs/auth-provider'
import invariant from '@shared/lib/invariant'
import { useContext } from 'react'

export function useAuth() {
    const context = useContext(AuthContext)
    invariant(context, 'useAuth must be used within an AuthProvider')
    return context
}
