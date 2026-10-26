const breakpoints = {
    xs: '30em',
    sm: '40em',
    md: '48em',
    lg: '64em',
    xl: '80em',
    '2xl': '96em',
    '3xl': '120em',
    '4xl': '160em'
}

function expandBreakpoints(value) {
    return value.replace(/\$mantine-breakpoint-([\w-]+)/g, (_match, name) => {
        if (!Object.hasOwn(breakpoints, name)) throw new Error(`Unknown breakpoint: ${name}`)
        return breakpoints[name]
    })
}

module.exports = {
    plugins: [
        require('postcss-preset-mantine')({ autoRem: true }),
        // Only named breakpoints need expansion; the preset handles mixins.
        {
            postcssPlugin: 'gowave-breakpoints',
            AtRule(node) {
                node.params = expandBreakpoints(node.params)
            },
            Declaration(node) {
                node.value = expandBreakpoints(node.value)
            }
        }
    ]
}
