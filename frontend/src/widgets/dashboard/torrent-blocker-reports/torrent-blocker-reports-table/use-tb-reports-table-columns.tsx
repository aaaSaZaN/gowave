import {
    NodeSelectItem,
    type NodeSelectItemProps
} from '@features/dashboard/users/users-table/model/node-select-item'
import type { MRT_ColumnDef } from '@kastov/mantine-react-table-open'
import { Group, Text } from '@mantine/core'
import type { GetNodesCommand, GetTorrentBlockerReportsCommand } from '@remnawave/backend-contract'
import { CountryFlag } from '@shared/ui/get-country-flag'
import { formatTimeUtil } from '@shared/utils/time-utils'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

export const useTbReportsTableColumns = (nodes?: GetNodesCommand.Response['response']) => {
    const { t, i18n } = useTranslation()

    return useMemo<
        MRT_ColumnDef<GetTorrentBlockerReportsCommand.Response['response']['records'][number]>[]
    >(() => {
        const renderCell0: NonNullable<
            MRT_ColumnDef<
                GetTorrentBlockerReportsCommand.Response['response']['records'][number]
            >['Cell']
        > = ({ cell }) => (
            <Group
                align="center"
                gap="sm"
                style={{
                    flex: 1,
                    minWidth: 0
                }}
                wrap="nowrap"
            >
                <CountryFlag countryCode={cell.row.original.node?.countryCode} />
                <Text ff="monospace" fw={500} size="md">
                    {cell.row.original.node?.name || '–'}
                </Text>
            </Group>
        )
        return [
            {
                accessorKey: 'user.username',
                header: 'Username',
                accessorFn: (originalRow) => {
                    if (originalRow.user?.username) {
                        return originalRow.user.username
                    }
                    return '–'
                },
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            },
            {
                accessorKey: 'userId',
                header: 'User ID',
                accessorFn: (originalRow) => {
                    if (originalRow.userId) {
                        return originalRow.userId
                    }
                    return '–'
                },
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            },
            {
                accessorKey: 'node.uuid',
                header: 'Reported Node',
                Cell: renderCell0,
                filterVariant: 'select',
                mantineFilterSelectProps: {
                    comboboxProps: {
                        transitionProps: { transition: 'fade', duration: 200 }
                    },
                    checkIconPosition: 'left',
                    data:
                        nodes?.map((node) => ({
                            label: node.name,
                            value: node.uuid,
                            countryCode: node.countryCode
                        })) ?? [],
                    renderOption: (item) => {
                        const option = item.option as NodeSelectItemProps
                        return <NodeSelectItem {...option} />
                    }
                },
                enableSorting: false,
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            },
            {
                accessorKey: 'report.actionReport.ip',
                header: 'IP',
                accessorFn: (originalRow) => {
                    if (originalRow.report?.actionReport?.ip) {
                        return originalRow.report.actionReport.ip
                    }
                    return '–'
                },

                enableSorting: false,
                enableColumnFilterModes: false,
                enableColumnFilter: true,
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            },
            {
                accessorKey: 'report.xrayReport.inboundTag',
                header: 'Inbound Tag',
                accessorFn: (originalRow) => {
                    if (originalRow.report?.xrayReport?.inboundTag) {
                        return originalRow.report.xrayReport.inboundTag
                    }
                    return '–'
                },

                enableSorting: false,
                enableColumnFilterModes: false,
                enableColumnFilter: true,
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            },

            {
                accessorKey: 'createdAt',
                header: t('common.field.created-at'),
                accessorFn: (originalRow) =>
                    formatTimeUtil({
                        time: originalRow.createdAt,
                        template: 'TIME_FIRST_DATETIME',
                        language: i18n.language
                    }),
                minSize: 250,
                enableColumnFilterModes: false,
                enableColumnFilter: false,

                mantineTableBodyCellProps: {
                    align: 'center',
                    ff: 'monospace'
                }
            },

            {
                accessorKey: 'report.xrayReport.outboundTag',
                header: 'Outbound Tag',
                accessorFn: (originalRow) => {
                    if (originalRow.report?.xrayReport?.outboundTag) {
                        return originalRow.report.xrayReport.outboundTag
                    }
                    return '–'
                },
                size: 300,

                enableSorting: false,
                enableColumnFilterModes: false,
                enableColumnFilter: true,
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            },

            {
                accessorKey: 'report.xrayReport.protocol',
                header: 'Protocol',
                accessorFn: (originalRow) => {
                    if (originalRow.report?.xrayReport?.protocol) {
                        return originalRow.report.xrayReport.protocol
                    }
                    return '–'
                },

                enableSorting: false,
                enableColumnFilterModes: false,
                enableColumnFilter: true,
                mantineTableBodyCellProps: {
                    align: 'center'
                }
            }
        ]
    }, [nodes, t, i18n.language])
}
