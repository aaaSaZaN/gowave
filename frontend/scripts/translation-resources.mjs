import { readdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const namespaces = readdirSync(resolve(root, 'public/locales/en'))
    .filter((name) => name.endsWith('.json'))
    .sort()
    .map((name) => name.slice(0, -5))

const imports = namespaces.map(
    (name, index) =>
        `import locale${index} from ${JSON.stringify(`../public/locales/en/${name}.json`)}`
)
const properties = namespaces.map((name, index) => `    ${JSON.stringify(name)}: locale${index}`)
writeFileSync(
    resolve(root, '@types/resources.ts'),
    `${imports.join('\n')}\n\nconst resources = {\n${properties.join(',\n')}\n} as const\n\nexport default resources\n`
)
