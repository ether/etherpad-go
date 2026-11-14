// adminBase is the path prefix Etherpad is served under: "/etherpad" when the
// admin page is at /etherpad/admin/..., "" at the root.
export const adminBase = (pathname: string = window.location.pathname): string => {
  const m = pathname.match(/^(.*?)\/admin(?:\/|$)/)
  return m ? m[1] : ''
}
