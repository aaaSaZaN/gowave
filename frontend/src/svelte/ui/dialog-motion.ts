export function dismissDialog(dialog: HTMLDialogElement | null | undefined): void {
    if (!dialog?.open || dialog.hasAttribute('data-closing')) return

    dialog.setAttribute('data-closing', '')
    const animations = dialog.getAnimations()
    const finish = () => {
        if (dialog.isConnected) dialog.close()
        dialog.removeAttribute('data-closing')
    }

    if (animations.length === 0) {
        finish()
        return
    }
    void Promise.allSettled(animations.map((animation) => animation.finished)).then(finish)
}

export function cancelDialog(event: Event, blocked = false): void {
    if (event.defaultPrevented) return
    event.preventDefault()
    if (!blocked && event.currentTarget instanceof HTMLDialogElement)
        dismissDialog(event.currentTarget)
}

export function dismissContainingDialog(event: Event): void {
    if (event.currentTarget instanceof Element) dismissDialog(event.currentTarget.closest('dialog'))
}
