import { createRoot } from 'react-dom/client'

import { App } from './app'

import '@shared/utils/setup-lottie/setup-lottie'

if (typeof window !== 'undefined') {
    if (!navigator.clipboard || !window.isSecureContext) {
        ;(navigator as any).clipboard = {
            writeText: (text: string) => {
                return new Promise<void>((resolve, reject) => {
                    try {
                        const textArea = document.createElement('textarea')
                        textArea.value = text
                        textArea.style.position = 'fixed'
                        textArea.style.left = '-999999px'
                        textArea.style.top = '-999999px'
                        document.body.appendChild(textArea)
                        textArea.focus()
                        textArea.select()
                        const successful = document.execCommand('copy')
                        textArea.remove()
                        if (successful) {
                            resolve()
                        } else {
                            reject(new Error('execCommand copy failed'))
                        }
                    } catch (err) {
                        reject(err)
                    }
                })
            }
        }
    }
}

const root = createRoot(document.getElementById('root')!)
root.render(<App />)
