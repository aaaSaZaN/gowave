import { useEffect, useState } from 'react'

export function useTime(interval = 60_000): number {
    const [now, setNow] = useState(() => Date.now())
    useEffect(() => {
        const timer = setInterval(() => setNow(Date.now()), interval)
        return () => clearInterval(timer)
    }, [interval])
    return now
}
