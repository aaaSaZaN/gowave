import { runAsync } from '@shared/lib/run-async'
import type { editor } from 'monaco-editor'

export const forceMonacoRetokenize = (instance: editor.IStandaloneCodeEditor) => {
    runAsync(instance.getAction('editor.action.forceRetokenize')?.run())
}
