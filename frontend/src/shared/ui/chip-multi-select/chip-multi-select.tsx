import { Checkbox, Group, Input, SimpleGrid, Text } from '@mantine/core'
import { optionalProp } from '@shared/lib/optional-prop'
import classes from './ChipMultiSelect.module.css'
import type { IProps } from './interfaces'

export function ChipMultiSelect(props: IProps) {
    const { data, value, defaultValue, onChange, label, description, error } = props

    return (
        <Input.Wrapper description={description} error={error} label={label}>
            <Checkbox.Group
                {...optionalProp('defaultValue', defaultValue)}
                {...optionalProp('onChange', onChange)}
                {...optionalProp('value', value)}
            >
                <SimpleGrid
                    cols={{
                        base: 1,
                        xs: 2,
                        sm: 3
                    }}
                    mt={4}
                    spacing="xs"
                    verticalSpacing="xs"
                >
                    {data.map((item) => (
                        <Checkbox.Card
                            className={classes.card}
                            key={item.value}
                            radius="md"
                            value={item.value}
                        >
                            <Group align="center" gap="xs" justify="space-between" wrap="nowrap">
                                <Group align="center" gap="xs" wrap="nowrap">
                                    <Checkbox.Indicator color="red" size="sm" />
                                    <Text className={classes.label} size="sm">
                                        {item.label}
                                    </Text>
                                </Group>
                                {item.icon}
                            </Group>
                        </Checkbox.Card>
                    ))}
                </SimpleGrid>
            </Checkbox.Group>
        </Input.Wrapper>
    )
}
