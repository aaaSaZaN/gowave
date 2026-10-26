import NiceModal from '@ebay/nice-modal-react'
import { Component, type ComponentType, type ErrorInfo, type ReactNode } from 'react'
import { Outlet } from 'react-router'

import { NiceModalAutoClose } from '@shared/_modals/nice-modal-auto-close'

export interface ErrorBoundaryFallbackProps {
    componentStack: null | string
    error: unknown
    resetErrorBoundary: () => void
}

interface IProps {
    FallbackComponent: ComponentType<ErrorBoundaryFallbackProps>
}

interface BoundaryState {
    failed: boolean
    error: unknown
    componentStack: string | null
}

export class ErrorBoundary extends Component<IProps & { children: ReactNode }, BoundaryState> {
    state: BoundaryState = { failed: false, error: null, componentStack: null }

    static getDerivedStateFromError(error: unknown): Partial<BoundaryState> {
        return { failed: true, error }
    }

    componentDidCatch(_error: unknown, info: ErrorInfo) {
        this.setState({ componentStack: info.componentStack ?? null })
    }

    resetErrorBoundary = () => this.setState({ failed: false, error: null, componentStack: null })

    render() {
        const { FallbackComponent, children } = this.props
        return this.state.failed ? (
            <FallbackComponent
                componentStack={this.state.componentStack}
                error={this.state.error}
                resetErrorBoundary={this.resetErrorBoundary}
            />
        ) : (
            children
        )
    }
}

export const ErrorBoundaryHoc = ({ FallbackComponent }: IProps) => (
    <ErrorBoundary FallbackComponent={FallbackComponent}>
        <NiceModal.Provider>
            <NiceModalAutoClose />
            <Outlet />
        </NiceModal.Provider>
    </ErrorBoundary>
)
