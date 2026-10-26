import type {
    GetBandwidthStatsCommand,
    GetRemnawaveHealthCommand,
    GetStatsCommand
} from '@remnawave/backend-contract'

export interface IProps {
    bandwidthStats: GetBandwidthStatsCommand.Response['response']
    gowaveHealth: GetRemnawaveHealthCommand.Response['response']
    systemInfo: GetStatsCommand.Response['response']
}
