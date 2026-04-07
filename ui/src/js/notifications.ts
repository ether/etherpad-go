import './components/EpNotification'
import { EpNotification } from './components/EpNotification'

export const notifications = {
  add(args: { title?: string; text: string | Node; class_name?: string; sticky?: boolean; time?: number; position?: 'top' | 'bottom' }): string {
    const type = args.class_name?.includes('error') ? 'error' : 'success'
    const textContent = args.text instanceof Node ? (args.text as HTMLElement).textContent || '' : String(args.text)
    return EpNotification.show({
      text: textContent,
      type,
      duration: args.sticky ? 0 : Number(args.time ?? 3000),
      position: args.position ?? 'top',
    })
  },
  remove(id: string): void {
    const el = document.getElementById(id)
    el?.remove()
  },
  removeAll(): void {
    document.querySelectorAll('ep-notification').forEach(el => el.remove())
  },
}

export default notifications
