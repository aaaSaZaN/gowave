export const monacoTheme = {
    base: 'vs-dark',
    inherit: true,
    rules: [
        {
            background: '161b22',
            token: ''
        },
        {
            foreground: '8b949e',
            token: 'comment'
        },
        {
            foreground: '54aeff',
            token: 'constant'
        },
        {
            foreground: 'ffb77c',
            token: 'entity'
        },
        {
            foreground: 'fae17d',
            token: 'keyword'
        },
        {
            foreground: 'fae17d',
            token: 'storage'
        },
        {
            foreground: 'aceebb',
            token: 'string'
        },
        {
            foreground: 'aceebb',
            token: 'meta.verbatim'
        },
        {
            foreground: '54aeff',
            token: 'support'
        },
        {
            foreground: 'fb8f44',
            fontStyle: 'italic',
            token: 'invalid.deprecated'
        },
        {
            foreground: 'c9d1d9',
            background: '7d4e00',
            token: 'invalid.illegal'
        },
        {
            foreground: 'ffb77c',
            fontStyle: 'italic',
            token: 'entity.other.inherited-class'
        },
        {
            foreground: 'ffd8b5',
            token: 'string constant.other.placeholder'
        },
        {
            foreground: '80ccff',
            token: 'meta.function-call.py'
        },
        {
            foreground: '6e7681',
            token: 'meta.tag'
        },
        {
            foreground: '6e7681',
            token: 'meta.tag entity'
        },
        {
            foreground: 'b1bac4',
            token: 'entity.name.section'
        },
        {
            foreground: 'b1bac4',
            token: 'keyword.type.variant'
        },
        {
            foreground: 'c9d1d9',
            token: 'source.ocaml keyword.operator.symbol'
        },
        {
            foreground: '54aeff',
            token: 'source.ocaml keyword.operator.symbol.infix'
        },
        {
            foreground: '54aeff',
            token: 'source.ocaml keyword.operator.symbol.prefix'
        },
        {
            fontStyle: 'underline',
            token: 'source.ocaml keyword.operator.symbol.infix.floating-point'
        },
        {
            fontStyle: 'underline',
            token: 'source.ocaml keyword.operator.symbol.prefix.floating-point'
        },
        {
            fontStyle: 'underline',
            token: 'source.ocaml constant.numeric.floating-point'
        },
        {
            background: 'ffffff08',
            token: 'text.tex.latex meta.function.environment'
        },
        {
            background: '218bff10',
            token: 'text.tex.latex meta.function.environment meta.function.environment'
        },
        {
            foreground: 'fae17d',
            token: 'text.tex.latex support.function'
        },
        {
            foreground: 'c9d1d9',
            token: 'source.plist string.unquoted'
        },
        {
            foreground: 'c9d1d9',
            token: 'source.plist keyword.operator'
        }
    ],
    colors: {
        'editor.foreground': '#c9d1d9',
        'editor.background': '#161b22',
        'editor.selectionBackground': '#0969da55',
        'editor.lineHighlightBackground': '#21262d',
        'editorCursor.foreground': '#54aeff',
        'editorWhitespace.foreground': '#484f58',
        'editorLineNumber.foreground': '#6e7681',
        'editorLineNumber.activeForeground': '#c9d1d9',
        'editorWidget.background': '#1c2128',
        'editorWidget.border': '#30363d',
        'editorWidget.foreground': '#c9d1d9',
        'editorHoverWidget.background': '#1c2128',
        'editorHoverWidget.border': '#30363d',
        'editorHoverWidget.foreground': '#c9d1d9',
        'editorSuggestWidget.background': '#1c2128',
        'editorSuggestWidget.border': '#30363d',
        'editorSuggestWidget.foreground': '#c9d1d9',
        'editorSuggestWidget.selectedBackground': '#21262d',
        'editorSuggestWidget.highlightForeground': '#54aeff',

        'menu.background': '#1c2128',
        'menu.foreground': '#c9d1d9',
        'menu.selectionBackground': '#22b8cf29',
        'menu.selectionForeground': '#e6edf3',
        'menu.separatorBackground': '#30363d',
        'menu.border': '#30363d'
    }
}

export const monacoLightTheme = {
    base: 'vs',
    inherit: true,
    rules: [
        { foreground: '59636e', token: 'comment' },
        { foreground: '0550ae', token: 'constant' },
        { foreground: '8250df', token: 'entity' },
        { foreground: 'cf222e', token: 'keyword' },
        { foreground: 'cf222e', token: 'storage' },
        { foreground: '0a3069', token: 'string' },
        { foreground: '0a3069', token: 'meta.verbatim' },
        { foreground: '0550ae', token: 'support' },
        { foreground: '953800', fontStyle: 'italic', token: 'invalid.deprecated' },
        { foreground: '116329', token: 'string.key.json' },
        { foreground: '0550ae', token: 'number' },
        { foreground: '116329', token: 'type' }
    ],
    colors: {
        'editor.foreground': '#1f2328',
        'editor.background': '#ffffff',
        'editor.selectionBackground': '#0969da33',
        'editor.lineHighlightBackground': '#f6f8fa',
        'editorCursor.foreground': '#0969da',
        'editorWhitespace.foreground': '#d1d9e0',
        'editorLineNumber.foreground': '#818b98',
        'editorLineNumber.activeForeground': '#1f2328',
        'editorWidget.background': '#ffffff',
        'editorWidget.border': '#d1d9e0',
        'editorWidget.foreground': '#1f2328',
        'editorHoverWidget.background': '#ffffff',
        'editorHoverWidget.border': '#d1d9e0',
        'editorHoverWidget.foreground': '#1f2328',
        'editorSuggestWidget.background': '#ffffff',
        'editorSuggestWidget.border': '#d1d9e0',
        'editorSuggestWidget.foreground': '#1f2328',
        'editorSuggestWidget.selectedBackground': '#f6f8fa',
        'editorSuggestWidget.highlightForeground': '#0969da',

        'menu.background': '#ffffff',
        'menu.foreground': '#1f2328',
        'menu.selectionBackground': '#0c85991f',
        'menu.selectionForeground': '#1f2328',
        'menu.separatorBackground': '#d1d9e0',
        'menu.border': '#d1d9e0'
    }
}
