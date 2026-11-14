// adminBase is the path prefix Etherpad is served under: "/etherpad" when the
// admin page is at /etherpad/admin/..., "" at the root. The admin's own
// routes hold no "admin" segment, so the last one is the admin's: the
// prefix may contain one ("/x/admin/y/admin/pads" is under "/x/admin/y").
export const adminBase = (pathname: string = window.location.pathname): string => {
  const m = pathname.match(/^(.*)\/admin(?:\/|$)/)
  return m ? m[1] : ''
}
