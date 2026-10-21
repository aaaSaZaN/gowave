interface INodeStatusFields {
    isConnected: boolean
    isDisabled: boolean
    lastStatusMessage: null | string
}

export function hasNodeStatusMessage(node: INodeStatusFields): boolean {
    return !!node.lastStatusMessage && node.lastStatusMessage !== 'OK'
}

// Gowave keeps a node connected when it rejects a new config and keeps serving
// the previous one; the message then explains why the config was rejected.
export function isNodeConfigRejected(node: INodeStatusFields): boolean {
    return node.isConnected && !node.isDisabled && hasNodeStatusMessage(node)
}
