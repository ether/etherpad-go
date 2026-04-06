import { useEffect, useRef } from 'react'
import { X, CheckCircle, AlertCircle } from 'lucide-react'
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

  const isSuccess = toast.kind === 'success'

  return (
    <div className="fixed top-4 right-4 z-50 animate-[slideIn_0.2s_ease-out]">
      <div
        className={`flex items-center gap-3 rounded-lg px-4 py-3 shadow-lg ${
          isSuccess
            ? 'bg-green-600 text-white'
            : 'bg-red-600 text-white'
        }`}
      >
        {isSuccess ? (
          <CheckCircle className="h-5 w-5 shrink-0" />
        ) : (
          <AlertCircle className="h-5 w-5 shrink-0" />
        )}
        <span className="text-sm font-medium">{toast.message}</span>
        <button
          type="button"
          onClick={() => setToast(null)}
          className="ml-2 rounded p-0.5 hover:bg-white/20 transition-colors"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}
