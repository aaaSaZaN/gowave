import { GetNodeCommand, UpdateNodeCommand } from '@remnawave/backend-contract'
import { z } from 'zod'

// trafficTrackingStart is a Gowave extension: how the counter starts when the
// update enables traffic tracking.
export const TRAFFIC_TRACKING_START = ['period', 'zero', 'keep'] as const
export type TTrafficTrackingStart = (typeof TRAFFIC_TRACKING_START)[number]

export const UpdateNodeBodySchema = UpdateNodeCommand.RequestBodySchema.extend({
    trafficTrackingStart: z.enum(TRAFFIC_TRACKING_START).optional()
})

// The stored mode is a Gowave extension of the node response as well.
export const UpdateNodeResponseSchema = UpdateNodeCommand.ResponseSchema.extend({
    response: UpdateNodeCommand.ResponseSchema.shape.response.extend({
        trafficTrackingStart: z.enum(TRAFFIC_TRACKING_START).nullish()
    })
})

export const GetNodeResponseSchema = GetNodeCommand.ResponseSchema.extend({
    response: GetNodeCommand.ResponseSchema.shape.response.extend({
        trafficTrackingStart: z.enum(TRAFFIC_TRACKING_START).nullish(),
        versions: GetNodeCommand.ResponseSchema.shape.response.shape.versions
            .unwrap()
            .extend({ coreSource: z.string().optional() })
            .nullable()
    })
})
