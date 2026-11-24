import { OPEN_ENTITY } from '@shared/constants/open-entity'
import { isValidUuid } from '@shared/utils/misc/is'

export const targets = [
    [OPEN_ENTITY.USER, 'constants.users'],
    [OPEN_ENTITY.HOST, 'constants.hosts'],
    [OPEN_ENTITY.NODE, 'constants.nodes'],
    [OPEN_ENTITY.CONFIG_PROFILE, 'constants.config-profiles'],
    [OPEN_ENTITY.INTERNAL_SQUAD, 'constants.internal-squads'],
    [OPEN_ENTITY.EXTERNAL_SQUAD, 'constants.external-squads'],
    [OPEN_ENTITY.NODE_PLUGIN, 'constants.node-plugins'],
    [OPEN_ENTITY.SUBPAGE_CONFIG, 'constants.subscription-page']
] as const

export type QuickTarget = (typeof targets)[number][0]

export function validEntityId(entity: QuickTarget, id: string): boolean {
    if (entity === OPEN_ENTITY.USER) {
        const number = Number(id)
        return /^\d+$/.test(id) && Number.isSafeInteger(number) && number > 0
    }
    return isValidUuid(id)
}
