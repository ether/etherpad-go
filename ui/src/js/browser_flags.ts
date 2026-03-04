const ua = (typeof navigator !== 'undefined' ? navigator.userAgent : '').toLowerCase();

export const browserFlags = {
  opera: /opera|opr\//.test(ua),
  safari: /safari/.test(ua) && !/chrome|crios|android/.test(ua),
  chrome: /chrome|crios/.test(ua),
  firefox: /firefox|iceweasel/.test(ua),
};
