import { githubDarkTheme } from '@json-edit-react/themes'

/**
 * GitHub-styled theme for json-edit-react whose colors are CSS variables
 * declared per color scheme in global.css, so one theme object follows the
 * active Mantine color scheme without re-rendering consumers.
 */
export const jsonViewerTheme = {
    ...githubDarkTheme,
    displayName: 'GoWave',
    fragments: {
        purple: 'var(--gw-json-purple)',
        skyBlue: 'var(--gw-json-string)',
        lime: 'var(--gw-json-bracket)',
        orange: 'var(--gw-json-orange)'
    },
    styles: {
        ...githubDarkTheme.styles,
        container: {
            backgroundColor: 'var(--gw-json-bg)',
            color: 'var(--gw-json-fg)'
        },
        property: 'var(--gw-json-property)',
        itemCount: 'var(--gw-json-count)',
        boolean: {
            color: 'var(--gw-json-boolean)',
            fontSize: '90%',
            fontWeight: 'bold'
        },
        null: [
            'var(--gw-json-null)',
            {
                backgroundColor: 'var(--gw-json-null-bg)',
                padding: '0.05em 0.2em',
                borderRadius: '0.3em'
            }
        ]
    }
} satisfies typeof githubDarkTheme
