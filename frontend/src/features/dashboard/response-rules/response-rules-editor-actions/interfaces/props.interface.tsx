import type { Monaco } from '@monaco-editor/react'
import type { ResponseRules } from '@shared/api/schemas/subscription-settings.schema'
import type { editor } from 'monaco-editor'
import type { RefObject } from 'react'

export interface Props {
    editorRef: RefObject<editor.IStandaloneCodeEditor | null>
    hasUnsavedChanges: boolean
    isResponseRulesValid: boolean
    monacoRef: RefObject<Monaco | null>
    originalValue: string
    responseRules: ResponseRules
    setHasUnsavedChanges: (value: boolean) => void
    setIsResponseRulesValid: (value: boolean) => void
    setOriginalValue: (value: string) => void
    setResult: (value: string) => void
    subscriptionSettingsUuid: string
}
