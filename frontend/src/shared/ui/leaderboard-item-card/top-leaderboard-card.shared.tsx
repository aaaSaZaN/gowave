import { Card, Center, ScrollArea, Skeleton, Stack, Text } from '@mantine/core'
import { optionalProp } from '@shared/lib/optional-prop'
import type { ReactNode } from 'react'
import { PiEmpty } from 'react-icons/pi'
import { Virtuoso } from 'react-virtuoso'

import { LeaderboardItemCardShared } from './leaderboard-item-card.shared'

const LoadingMoreFooter = () => (
    <Stack gap={6}>
        {Array.from({ length: 3 }, (_, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: Skeletons are stateless placeholder slots.
            <Skeleton height={40} key={`more-${i}`} />
        ))}
    </Stack>
)

export interface ITopLeaderboardItem {
    color: string
    countryCode?: string
    name: string
    total: number
    uuid?: string
}

interface IProps<T extends ITopLeaderboardItem> {
    emptyText: string
    formatValue?: (value: number) => string
    isFetchingMore?: boolean
    isLoading: boolean
    items: T[] | undefined
    maxHeight?: number
    onEndReached?: () => void
    onItemClick?: (item: T) => void
    renderCountryFlag?: (item: T) => ReactNode
    skeletonCount?: number
    wrapper?: (children: ReactNode) => ReactNode
    ordered?: boolean
    virtualized?: boolean
}

export function TopLeaderboardCardShared<T extends ITopLeaderboardItem>(props: IProps<T>) {
    const {
        items,
        onItemClick,
        isLoading,
        emptyText,
        renderCountryFlag,
        maxHeight,
        skeletonCount = 5,
        formatValue,
        wrapper,
        ordered = true,
        onEndReached,
        isFetchingMore,
        virtualized = false
    } = props

    let maxTraffic = 1
    if (items && items.length > 0) {
        if (ordered) {
            maxTraffic = items[0]?.total ?? 0
        } else {
            maxTraffic = items.reduce((max, item) => Math.max(max, item.total), 1)
        }
    }

    const content = (
        <Stack gap={6}>
            {items?.map((item) => (
                <LeaderboardItemCardShared
                    color={item.color}
                    countryFlag={renderCountryFlag?.(item)}
                    {...optionalProp('formatValue', formatValue)}
                    key={item.name}
                    name={item.name}
                    {...optionalProp(
                        'onItemClick',
                        onItemClick ? () => onItemClick(item) : undefined
                    )}
                    total={item.total}
                    {...optionalProp('uuid', item.uuid)}
                    value={maxTraffic}
                />
            ))}
        </Stack>
    )

    const contentMinHeight = skeletonCount * 40 + (skeletonCount - 1) * 6

    const innerContent = (
        <Stack gap="sm" mih={contentMinHeight}>
            <Stack gap="sm" mih={contentMinHeight}>
                {isLoading && (
                    <Stack gap={6}>
                        {Array.from({ length: skeletonCount }, (_, i) => (
                            // biome-ignore lint/suspicious/noArrayIndexKey: Skeletons are stateless placeholder slots.
                            <Skeleton height={40} key={i} />
                        ))}
                    </Stack>
                )}

                {!isLoading && items && items.length > 0 && virtualized && (
                    <Virtuoso
                        components={isFetchingMore ? { Footer: LoadingMoreFooter } : {}}
                        computeItemKey={(_, item) => item.name}
                        data={items}
                        {...optionalProp('endReached', onEndReached)}
                        increaseViewportBy={200}
                        itemContent={(_, item) => (
                            <div style={{ paddingBottom: 6 }}>
                                <LeaderboardItemCardShared
                                    color={item.color}
                                    countryFlag={renderCountryFlag?.(item)}
                                    {...optionalProp('formatValue', formatValue)}
                                    name={item.name}
                                    {...optionalProp(
                                        'onItemClick',
                                        onItemClick ? () => onItemClick(item) : undefined
                                    )}
                                    total={item.total}
                                    {...optionalProp('uuid', item.uuid)}
                                    value={maxTraffic}
                                />
                            </div>
                        )}
                        style={{ height: maxHeight ?? 400 }}
                    />
                )}

                {!isLoading &&
                    items &&
                    items.length > 0 &&
                    !virtualized &&
                    (maxHeight ? (
                        <ScrollArea.Autosize
                            mah={maxHeight}
                            scrollbars="y"
                            styles={{ content: { minWidth: 0 }, scrollbar: { width: '10px' } }}
                            type="hover"
                        >
                            {content}
                        </ScrollArea.Autosize>
                    ) : (
                        content
                    ))}

                {!isLoading && items && items.length === 0 && (
                    <Center flex={1}>
                        <Stack align="center" gap={8}>
                            <PiEmpty size="32px" style={{ opacity: 0.5 }} />
                            <Text c="dimmed" size="sm">
                                {emptyText}
                            </Text>
                        </Stack>
                    </Center>
                )}
            </Stack>
        </Stack>
    )

    const defaultWrapper = (children: ReactNode) => (
        <Card p="md" withBorder>
            {children}
        </Card>
    )

    return wrapper ? wrapper(innerContent) : defaultWrapper(innerContent)
}
