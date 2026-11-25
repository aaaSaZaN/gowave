export const OPEN_ENTITY = {
    HOST: 'host',
    CONFIG_PROFILE: 'config-profile',
    EXTERNAL_SQUAD: 'external-squad',
    INTERNAL_SQUAD: 'internal-squad',
    NODE: 'node',
    NODE_PLUGIN: 'node-plugin',
    SUBPAGE_CONFIG: 'subpage-config',
    USER: 'user'
} as const

export type TOpenEntity = (typeof OPEN_ENTITY)[keyof typeof OPEN_ENTITY]
