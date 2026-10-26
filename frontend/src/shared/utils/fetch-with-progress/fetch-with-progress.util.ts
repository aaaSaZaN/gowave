import axios from 'axios'

import { consola } from '@shared/lib/logger'

export const fetchWithProgress = async (url: string, onProgress?: (progress: number) => void) => {
    try {
        const response = await axios.get(url, {
            onDownloadProgress: (progressEvent) => {
                if (progressEvent.total) {
                    // Precompressed responses report the encoded size as the total while
                    // some browsers count decoded bytes as loaded.
                    const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
                    onProgress?.(Math.min(progress, 100))
                } else {
                    onProgress?.(100)
                }
            },
            responseType: 'arraybuffer'
        })

        return response.data
    } catch (error) {
        consola.error('Download failed:', error)
        throw error
    }
}
