import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { useAdminStore } from '@/store'

export function Toast() {
  const { toast, setToast } = useAdminStore()
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    if (toast && toast.kind === 'success') {
      timerRef.current = setTimeout(() => {
        setToast(null)
      }, 3000)
    }
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [toast, setToast])

  if (!toast) return null

  const isError = toast.kind !== 'success'

  return (
    <div className="fixed bottom-4 right-4 z-50 animate-[slideIn_0.15s_ease-out]">
      <div
        className={`flex items-center gap-3 rounded-lg px-4 py-2.5 text-sm ${
          isError
            ? 'border border-red-200 dark:border-red-800 bg-red-600 text-white'
            : 'bg-black dark:bg-white text-white dark:text-black'
        }`}
      >
        <span className="font-medium">{toast.message}</span>
        <button
          type="button"
          onClick={() => setToast(null)}
          className="ml-1 rounded p-0.5 transition-colors hover:bg-white/20"
        >
          <X className="h-3.5 w-3.5" strokeWidth={1.5} />
        </button>
      </div>
    </div>
  )
}
