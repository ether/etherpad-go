require.define({
    "ep_etherpad-lite/static/js/ace2_common.js": {
        "(module ep_etherpad-lite/static/js/ace2_common.js)": function(require, exports, module) {
            "use strict";
            const isNodeText = e=>e.nodeType === 3
                , getAssoc = (e,o)=>e[`_magicdom_${o}`]
                , setAssoc = (e,o,t)=>{
                    e[`_magicdom_${o}`] = t
                }
                , binarySearch = (e,o)=>{
                    if (e < 1 || o(0))
                        return 0;
                    if (!o(e - 1))
                        return e;
                    let t = 0
                        , r = e - 1;
                    for (; r - t > 1; ) {
                        const s = Math.floor((t + r) / 2);
                        o(s) ? r = s : t = s
                    }
                    return r
                }
                , binarySearchInfinite = (e,o)=>{
                    let t = 0;
                    for (; !o(t); )
                        t += e;
                    return binarySearch(t, o)
                }
                , noop = ()=>{}
            ;
            exports.isNodeText = isNodeText,
                exports.getAssoc = getAssoc,
                exports.setAssoc = setAssoc,
                exports.binarySearch = binarySearch,
                exports.binarySearchInfinite = binarySearchInfinite,
                exports.noop = noop;
        }
    }["(module ep_etherpad-lite/static/js/ace2_common.js)"],
    "ep_etherpad-lite/static/js/vendors/browser.js": {
        "(module ep_etherpad-lite/static/js/vendors/browser.js)": function(require, exports, module) {
            /*!
  * Bowser - a browser detector
  * https://github.com/ded/bowser
  * MIT License | (c) Dustin Diaz 2015
  */
            (function(i, a) {
                    typeof module < "u" && module.exports ? module.exports = a() : typeof define == "function" && define.amd ? define(a) : this[i] = a()
                }
            )("bowser", function() {
                var i = !0;
                function a(s) {
                    function o(c) {
                        var d = s.match(c);
                        return d && d.length > 1 && d[1] || ""
                    }
                    function m(c) {
                        var d = s.match(c);
                        return d && d.length > 1 && d[2] || ""
                    }
                    var t = o(/(ipod|iphone|ipad)/i).toLowerCase(), z = /like android/i.test(s), f = !z && /android/i.test(s), S = /CrOS/.test(s), b = /silk/i.test(s), h = /sailfish/i.test(s), p = /tizen/i.test(s), w = /(web|hpw)os/i.test(s), k = /windows phone/i.test(s), B = !k && /windows/i.test(s), M = !t && !b && /macintosh/i.test(s), P = !f && !h && !p && !w && /linux/i.test(s), v = o(/edge\/(\d+(\.\d+)?)/i), n = o(/version\/(\d+(\.\d+)?)/i), g = /tablet/i.test(s), y = !g && /[^-]mobi/i.test(s), e;
                    /opera|opr/i.test(s) ? e = {
                        name: "Opera",
                        opera: i,
                        version: n || o(/(?:opera|opr)[\s\/](\d+(\.\d+)?)/i)
                    } : /yabrowser/i.test(s) ? e = {
                        name: "Yandex Browser",
                        yandexbrowser: i,
                        version: n || o(/(?:yabrowser)[\s\/](\d+(\.\d+)?)/i)
                    } : k ? (e = {
                        name: "Windows Phone",
                        windowsphone: i
                    },
                        v ? (e.msedge = i,
                            e.version = v) : (e.msie = i,
                            e.version = o(/iemobile\/(\d+(\.\d+)?)/i))) : /msie|trident/i.test(s) ? e = {
                        name: "Internet Explorer",
                        msie: i,
                        version: o(/(?:msie |rv:)(\d+(\.\d+)?)/i)
                    } : S ? e = {
                        name: "Chrome",
                        chromeos: i,
                        chromeBook: i,
                        chrome: i,
                        version: o(/(?:chrome|crios|crmo)\/(\d+(\.\d+)?)/i)
                    } : /chrome.+? edge/i.test(s) ? e = {
                        name: "Microsoft Edge",
                        msedge: i,
                        version: v
                    } : /chrome|crios|crmo/i.test(s) ? e = {
                        name: "Chrome",
                        chrome: i,
                        version: o(/(?:chrome|crios|crmo)\/(\d+(\.\d+)?)/i)
                    } : t ? (e = {
                        name: t == "iphone" ? "iPhone" : t == "ipad" ? "iPad" : "iPod"
                    },
                    n && (e.version = n)) : h ? e = {
                        name: "Sailfish",
                        sailfish: i,
                        version: o(/sailfish\s?browser\/(\d+(\.\d+)?)/i)
                    } : /seamonkey\//i.test(s) ? e = {
                        name: "SeaMonkey",
                        seamonkey: i,
                        version: o(/seamonkey\/(\d+(\.\d+)?)/i)
                    } : /firefox|iceweasel/i.test(s) ? (e = {
                        name: "Firefox",
                        firefox: i,
                        version: o(/(?:firefox|iceweasel)[ \/](\d+(\.\d+)?)/i)
                    },
                    /\((mobile|tablet);[^\)]*rv:[\d\.]+\)/i.test(s) && (e.firefoxos = i)) : b ? e = {
                        name: "Amazon Silk",
                        silk: i,
                        version: o(/silk\/(\d+(\.\d+)?)/i)
                    } : f ? e = {
                        name: "Android",
                        version: n
                    } : /phantom/i.test(s) ? e = {
                        name: "PhantomJS",
                        phantom: i,
                        version: o(/phantomjs\/(\d+(\.\d+)?)/i)
                    } : /blackberry|\bbb\d+/i.test(s) || /rim\stablet/i.test(s) ? e = {
                        name: "BlackBerry",
                        blackberry: i,
                        version: n || o(/blackberry[\d]+\/(\d+(\.\d+)?)/i)
                    } : w ? (e = {
                        name: "WebOS",
                        webos: i,
                        version: n || o(/w(?:eb)?osbrowser\/(\d+(\.\d+)?)/i)
                    },
                    /touchpad\//i.test(s) && (e.touchpad = i)) : /bada/i.test(s) ? e = {
                        name: "Bada",
                        bada: i,
                        version: o(/dolfin\/(\d+(\.\d+)?)/i)
                    } : p ? e = {
                        name: "Tizen",
                        tizen: i,
                        version: o(/(?:tizen\s?)?browser\/(\d+(\.\d+)?)/i) || n
                    } : /safari/i.test(s) ? e = {
                        name: "Safari",
                        safari: i,
                        version: n
                    } : e = {
                        name: o(/^(.*)\/(.*) /),
                        version: m(/^(.*)\/(.*) /)
                    },
                        !e.msedge && /(apple)?webkit/i.test(s) ? (e.name = e.name || "Webkit",
                            e.webkit = i,
                        !e.version && n && (e.version = n)) : !e.opera && /gecko\//i.test(s) && (e.name = e.name || "Gecko",
                            e.gecko = i,
                            e.version = e.version || o(/gecko\/(\d+(\.\d+)?)/i)),
                        !e.msedge && (f || e.silk) ? e.android = i : t ? (e[t] = i,
                            e.ios = i) : B ? e.windows = i : M ? e.mac = i : P && (e.linux = i);
                    var r = "";
                    e.windowsphone ? r = o(/windows phone (?:os)?\s?(\d+(\.\d+)*)/i) : t ? (r = o(/os (\d+([_\s]\d+)*) like mac os x/i),
                        r = r.replace(/[_\s]/g, ".")) : f ? r = o(/android[ \/-](\d+(\.\d+)*)/i) : e.webos ? r = o(/(?:web|hpw)os\/(\d+(\.\d+)*)/i) : e.blackberry ? r = o(/rim\stablet\sos\s(\d+(\.\d+)*)/i) : e.bada ? r = o(/bada\/(\d+(\.\d+)*)/i) : e.tizen && (r = o(/tizen[\/\s](\d+(\.\d+)*)/i)),
                    r && (e.osversion = r);
                    var x = r.split(".")[0];
                    return g || t == "ipad" || f && (x == 3 || x == 4 && !y) || e.silk ? e.tablet = i : (y || t == "iphone" || t == "ipod" || f || e.blackberry || e.webos || e.bada) && (e.mobile = i),
                        e.msedge || e.msie && e.version >= 10 || e.yandexbrowser && e.version >= 15 || e.chrome && e.version >= 20 || e.firefox && e.version >= 20 || e.safari && e.version >= 6 || e.opera && e.version >= 10 || e.ios && e.osversion && e.osversion.split(".")[0] >= 6 || e.blackberry && e.version >= 10.1 ? e.a = i : e.msie && e.version < 10 || e.chrome && e.version < 20 || e.firefox && e.version < 20 || e.safari && e.version < 6 || e.opera && e.version < 10 || e.ios && e.osversion && e.osversion.split(".")[0] < 6 ? e.c = i : e.x = i,
                        e
                }
                var l = a(typeof navigator < "u" ? navigator.userAgent : "");
                return l.test = function(s) {
                    for (var o = 0; o < s.length; ++o) {
                        var m = s[o];
                        if (typeof m == "string" && m in l)
                            return !0
                    }
                    return !1
                }
                    ,
                    l._detect = a,
                    l
            });
        }
    }["(module ep_etherpad-lite/static/js/vendors/browser.js)"],
    "ep_etherpad-lite/static/js/vendors/jquery.js": {
        "(module ep_etherpad-lite/static/js/vendors/jquery.js)": function(require, exports, module) {
            /*!
 * jQuery JavaScript Library v3.7.0
 * https://jquery.com/
 *
 * Copyright OpenJS Foundation and other contributors
 * Released under the MIT license
 * https://jquery.org/license
 *
 * Date: 2023-05-11T18:29Z
 */
            (function(k, Ie) {
                    "use strict";
                    typeof module == "object" && typeof module.exports == "object" ? module.exports = k.document ? Ie(k, !0) : function(z) {
                            if (!z.document)
                                throw new Error("jQuery requires a window with a document");
                            return Ie(z)
                        }
                        : Ie(k)
                }
            )(typeof window < "u" ? window : this, function(k, Ie) {
                "use strict";
                var z = []
                    , Mt = Object.getPrototypeOf
                    , Z = z.slice
                    , Rt = z.flat ? function(e) {
                        return z.flat.call(e)
                    }
                    : function(e) {
                        return z.concat.apply([], e)
                    }
                    , Je = z.push
                    , de = z.indexOf
                    , Ke = {}
                    , It = Ke.toString
                    , _e = Ke.hasOwnProperty
                    , _t = _e.toString
                    , kn = _t.call(Object)
                    , q = {}
                    , L = function(t) {
                    return typeof t == "function" && typeof t.nodeType != "number" && typeof t.item != "function"
                }
                    , Ae = function(t) {
                    return t != null && t === t.window
                }
                    , j = k.document
                    , jn = {
                    type: !0,
                    src: !0,
                    nonce: !0,
                    noModule: !0
                };
                function Wt(e, t, n) {
                    n = n || j;
                    var r, o, u = n.createElement("script");
                    if (u.text = e,
                        t)
                        for (r in jn)
                            o = t[r] || t.getAttribute && t.getAttribute(r),
                            o && u.setAttribute(r, o);
                    n.head.appendChild(u).parentNode.removeChild(u)
                }
                function De(e) {
                    return e == null ? e + "" : typeof e == "object" || typeof e == "function" ? Ke[It.call(e)] || "object" : typeof e
                }
                var Ft = "3.7.0"
                    , qn = /HTML$/i
                    , i = function(e, t) {
                    return new i.fn.init(e,t)
                };
                i.fn = i.prototype = {
                    jquery: Ft,
                    constructor: i,
                    length: 0,
                    toArray: function() {
                        return Z.call(this)
                    },
                    get: function(e) {
                        return e == null ? Z.call(this) : e < 0 ? this[e + this.length] : this[e]
                    },
                    pushStack: function(e) {
                        var t = i.merge(this.constructor(), e);
                        return t.prevObject = this,
                            t
                    },
                    each: function(e) {
                        return i.each(this, e)
                    },
                    map: function(e) {
                        return this.pushStack(i.map(this, function(t, n) {
                            return e.call(t, n, t)
                        }))
                    },
                    slice: function() {
                        return this.pushStack(Z.apply(this, arguments))
                    },
                    first: function() {
                        return this.eq(0)
                    },
                    last: function() {
                        return this.eq(-1)
                    },
                    even: function() {
                        return this.pushStack(i.grep(this, function(e, t) {
                            return (t + 1) % 2
                        }))
                    },
                    odd: function() {
                        return this.pushStack(i.grep(this, function(e, t) {
                            return t % 2
                        }))
                    },
                    eq: function(e) {
                        var t = this.length
                            , n = +e + (e < 0 ? t : 0);
                        return this.pushStack(n >= 0 && n < t ? [this[n]] : [])
                    },
                    end: function() {
                        return this.prevObject || this.constructor()
                    },
                    push: Je,
                    sort: z.sort,
                    splice: z.splice
                },
                    i.extend = i.fn.extend = function() {
                        var e, t, n, r, o, u, a = arguments[0] || {}, c = 1, f = arguments.length, d = !1;
                        for (typeof a == "boolean" && (d = a,
                            a = arguments[c] || {},
                            c++),
                             typeof a != "object" && !L(a) && (a = {}),
                             c === f && (a = this,
                                 c--); c < f; c++)
                            if ((e = arguments[c]) != null)
                                for (t in e)
                                    r = e[t],
                                    !(t === "__proto__" || a === r) && (d && r && (i.isPlainObject(r) || (o = Array.isArray(r))) ? (n = a[t],
                                        o && !Array.isArray(n) ? u = [] : !o && !i.isPlainObject(n) ? u = {} : u = n,
                                        o = !1,
                                        a[t] = i.extend(d, u, r)) : r !== void 0 && (a[t] = r));
                        return a
                    }
                    ,
                    i.extend({
                        expando: "jQuery" + (Ft + Math.random()).replace(/\D/g, ""),
                        isReady: !0,
                        error: function(e) {
                            throw new Error(e)
                        },
                        noop: function() {},
                        isPlainObject: function(e) {
                            var t, n;
                            return !e || It.call(e) !== "[object Object]" ? !1 : (t = Mt(e),
                                t ? (n = _e.call(t, "constructor") && t.constructor,
                                typeof n == "function" && _t.call(n) === kn) : !0)
                        },
                        isEmptyObject: function(e) {
                            var t;
                            for (t in e)
                                return !1;
                            return !0
                        },
                        globalEval: function(e, t, n) {
                            Wt(e, {
                                nonce: t && t.nonce
                            }, n)
                        },
                        each: function(e, t) {
                            var n, r = 0;
                            if (lt(e))
                                for (n = e.length; r < n && t.call(e[r], r, e[r]) !== !1; r++)
                                    ;
                            else
                                for (r in e)
                                    if (t.call(e[r], r, e[r]) === !1)
                                        break;
                            return e
                        },
                        text: function(e) {
                            var t, n = "", r = 0, o = e.nodeType;
                            if (o) {
                                if (o === 1 || o === 9 || o === 11)
                                    return e.textContent;
                                if (o === 3 || o === 4)
                                    return e.nodeValue
                            } else
                                for (; t = e[r++]; )
                                    n += i.text(t);
                            return n
                        },
                        makeArray: function(e, t) {
                            var n = t || [];
                            return e != null && (lt(Object(e)) ? i.merge(n, typeof e == "string" ? [e] : e) : Je.call(n, e)),
                                n
                        },
                        inArray: function(e, t, n) {
                            return t == null ? -1 : de.call(t, e, n)
                        },
                        isXMLDoc: function(e) {
                            var t = e && e.namespaceURI
                                , n = e && (e.ownerDocument || e).documentElement;
                            return !qn.test(t || n && n.nodeName || "HTML")
                        },
                        merge: function(e, t) {
                            for (var n = +t.length, r = 0, o = e.length; r < n; r++)
                                e[o++] = t[r];
                            return e.length = o,
                                e
                        },
                        grep: function(e, t, n) {
                            for (var r, o = [], u = 0, a = e.length, c = !n; u < a; u++)
                                r = !t(e[u], u),
                                r !== c && o.push(e[u]);
                            return o
                        },
                        map: function(e, t, n) {
                            var r, o, u = 0, a = [];
                            if (lt(e))
                                for (r = e.length; u < r; u++)
                                    o = t(e[u], u, n),
                                    o != null && a.push(o);
                            else
                                for (u in e)
                                    o = t(e[u], u, n),
                                    o != null && a.push(o);
                            return Rt(a)
                        },
                        guid: 1,
                        support: q
                    }),
                typeof Symbol == "function" && (i.fn[Symbol.iterator] = z[Symbol.iterator]),
                    i.each("Boolean Number String Function Array Date RegExp Object Error Symbol".split(" "), function(e, t) {
                        Ke["[object " + t + "]"] = t.toLowerCase()
                    });
                function lt(e) {
                    var t = !!e && "length"in e && e.length
                        , n = De(e);
                    return L(e) || Ae(e) ? !1 : n === "array" || t === 0 || typeof t == "number" && t > 0 && t - 1 in e
                }
                function $(e, t) {
                    return e.nodeName && e.nodeName.toLowerCase() === t.toLowerCase()
                }
                var Ln = z.pop
                    , Hn = z.sort
                    , On = z.splice
                    , _ = "[\\x20\\t\\r\\n\\f]"
                    , We = new RegExp("^" + _ + "+|((?:^|[^\\\\])(?:\\\\.)*)" + _ + "+$","g");
                i.contains = function(e, t) {
                    var n = t && t.parentNode;
                    return e === n || !!(n && n.nodeType === 1 && (e.contains ? e.contains(n) : e.compareDocumentPosition && e.compareDocumentPosition(n) & 16))
                }
                ;
                var Pn = /([\0-\x1f\x7f]|^-?\d)|^-$|[^\x80-\uFFFF\w-]/g;
                function Mn(e, t) {
                    return t ? e === "\0" ? "\uFFFD" : e.slice(0, -1) + "\\" + e.charCodeAt(e.length - 1).toString(16) + " " : "\\" + e
                }
                i.escapeSelector = function(e) {
                    return (e + "").replace(Pn, Mn)
                }
                ;
                var pe = j
                    , dt = Je;
                (function() {
                        var e, t, n, r, o, u = dt, a, c, f, d, y, b = i.expando, h = 0, x = 0, D = ut(), R = ut(), H = ut(), V = ut(), U = function(s, l) {
                            return s === l && (o = !0),
                                0
                        }, ae = "checked|selected|async|autofocus|autoplay|controls|defer|disabled|hidden|ismap|loop|multiple|open|readonly|required|scoped", se = "(?:\\\\[\\da-fA-F]{1,6}" + _ + "?|\\\\[^\\r\\n\\f]|[\\w-]|[^\0-\\x7f])+", M = "\\[" + _ + "*(" + se + ")(?:" + _ + "*([*^$|!~]?=)" + _ + `*(?:'((?:\\\\.|[^\\\\'])*)'|"((?:\\\\.|[^\\\\"])*)"|(` + se + "))|)" + _ + "*\\]", Se = ":(" + se + `)(?:\\((('((?:\\\\.|[^\\\\'])*)'|"((?:\\\\.|[^\\\\"])*)")|((?:\\\\.|[^\\\\()[\\]]|` + M + ")*)|.*)\\)|)", I = new RegExp(_ + "+","g"), B = new RegExp("^" + _ + "*," + _ + "*"), Qe = new RegExp("^" + _ + "*([>+~]|" + _ + ")" + _ + "*"), kt = new RegExp(_ + "|>"), fe = new RegExp(Se), Ye = new RegExp("^" + se + "$"), ce = {
                            ID: new RegExp("^#(" + se + ")"),
                            CLASS: new RegExp("^\\.(" + se + ")"),
                            TAG: new RegExp("^(" + se + "|[*])"),
                            ATTR: new RegExp("^" + M),
                            PSEUDO: new RegExp("^" + Se),
                            CHILD: new RegExp("^:(only|first|last|nth|nth-last)-(child|of-type)(?:\\(" + _ + "*(even|odd|(([+-]|)(\\d*)n|)" + _ + "*(?:([+-]|)" + _ + "*(\\d+)|))" + _ + "*\\)|)","i"),
                            bool: new RegExp("^(?:" + ae + ")$","i"),
                            needsContext: new RegExp("^" + _ + "*[>+~]|:(even|odd|eq|gt|lt|nth|first|last)(?:\\(" + _ + "*((?:-\\d)?\\d*)" + _ + "*\\)|)(?=[^-]|$)","i")
                        }, be = /^(?:input|select|textarea|button)$/i, xe = /^h\d$/i, te = /^(?:#([\w-]+)|(\w+)|\.([\w-]+))$/, jt = /[+~]/, ye = new RegExp("\\\\[\\da-fA-F]{1,6}" + _ + "?|\\\\([^\\r\\n\\f])","g"), ve = function(s, l) {
                            var p = "0x" + s.slice(1) - 65536;
                            return l || (p < 0 ? String.fromCharCode(p + 65536) : String.fromCharCode(p >> 10 | 55296, p & 1023 | 56320))
                        }, qr = function() {
                            me()
                        }, Lr = ft(function(s) {
                            return s.disabled === !0 && $(s, "fieldset")
                        }, {
                            dir: "parentNode",
                            next: "legend"
                        });
                        function Hr() {
                            try {
                                return a.activeElement
                            } catch {}
                        }
                        try {
                            u.apply(z = Z.call(pe.childNodes), pe.childNodes),
                                z[pe.childNodes.length].nodeType
                        } catch {
                            u = {
                                apply: function(l, p) {
                                    dt.apply(l, Z.call(p))
                                },
                                call: function(l) {
                                    dt.apply(l, Z.call(arguments, 1))
                                }
                            }
                        }
                        function W(s, l, p, g) {
                            var v, m, T, w, C, O, A, N = l && l.ownerDocument, P = l ? l.nodeType : 9;
                            if (p = p || [],
                            typeof s != "string" || !s || P !== 1 && P !== 9 && P !== 11)
                                return p;
                            if (!g && (me(l),
                                l = l || a,
                                f)) {
                                if (P !== 11 && (C = te.exec(s)))
                                    if (v = C[1]) {
                                        if (P === 9)
                                            if (T = l.getElementById(v)) {
                                                if (T.id === v)
                                                    return u.call(p, T),
                                                        p
                                            } else
                                                return p;
                                        else if (N && (T = N.getElementById(v)) && W.contains(l, T) && T.id === v)
                                            return u.call(p, T),
                                                p
                                    } else {
                                        if (C[2])
                                            return u.apply(p, l.getElementsByTagName(s)),
                                                p;
                                        if ((v = C[3]) && l.getElementsByClassName)
                                            return u.apply(p, l.getElementsByClassName(v)),
                                                p
                                    }
                                if (!V[s + " "] && (!d || !d.test(s))) {
                                    if (A = s,
                                        N = l,
                                    P === 1 && (kt.test(s) || Qe.test(s))) {
                                        for (N = jt.test(s) && qt(l.parentNode) || l,
                                             (N != l || !q.scope) && ((w = l.getAttribute("id")) ? w = i.escapeSelector(w) : l.setAttribute("id", w = b)),
                                                 O = at(s),
                                                 m = O.length; m--; )
                                            O[m] = (w ? "#" + w : ":scope") + " " + st(O[m]);
                                        A = O.join(",")
                                    }
                                    try {
                                        return u.apply(p, N.querySelectorAll(A)),
                                            p
                                    } catch {
                                        V(s, !0)
                                    } finally {
                                        w === b && l.removeAttribute("id")
                                    }
                                }
                            }
                            return Nn(s.replace(We, "$1"), l, p, g)
                        }
                        function ut() {
                            var s = [];
                            function l(p, g) {
                                return s.push(p + " ") > t.cacheLength && delete l[s.shift()],
                                    l[p + " "] = g
                            }
                            return l
                        }
                        function ie(s) {
                            return s[b] = !0,
                                s
                        }
                        function Me(s) {
                            var l = a.createElement("fieldset");
                            try {
                                return !!s(l)
                            } catch {
                                return !1
                            } finally {
                                l.parentNode && l.parentNode.removeChild(l),
                                    l = null
                            }
                        }
                        function Or(s) {
                            return function(l) {
                                return $(l, "input") && l.type === s
                            }
                        }
                        function Pr(s) {
                            return function(l) {
                                return ($(l, "input") || $(l, "button")) && l.type === s
                            }
                        }
                        function An(s) {
                            return function(l) {
                                return "form"in l ? l.parentNode && l.disabled === !1 ? "label"in l ? "label"in l.parentNode ? l.parentNode.disabled === s : l.disabled === s : l.isDisabled === s || l.isDisabled !== !s && Lr(l) === s : l.disabled === s : "label"in l ? l.disabled === s : !1
                            }
                        }
                        function Ee(s) {
                            return ie(function(l) {
                                return l = +l,
                                    ie(function(p, g) {
                                        for (var v, m = s([], p.length, l), T = m.length; T--; )
                                            p[v = m[T]] && (p[v] = !(g[v] = p[v]))
                                    })
                            })
                        }
                        function qt(s) {
                            return s && typeof s.getElementsByTagName < "u" && s
                        }
                        function me(s) {
                            var l, p = s ? s.ownerDocument || s : pe;
                            return p == a || p.nodeType !== 9 || !p.documentElement || (a = p,
                                    c = a.documentElement,
                                    f = !i.isXMLDoc(a),
                                    y = c.matches || c.webkitMatchesSelector || c.msMatchesSelector,
                                pe != a && (l = a.defaultView) && l.top !== l && l.addEventListener("unload", qr),
                                    q.getById = Me(function(g) {
                                        return c.appendChild(g).id = i.expando,
                                        !a.getElementsByName || !a.getElementsByName(i.expando).length
                                    }),
                                    q.disconnectedMatch = Me(function(g) {
                                        return y.call(g, "*")
                                    }),
                                    q.scope = Me(function() {
                                        return a.querySelectorAll(":scope")
                                    }),
                                    q.cssHas = Me(function() {
                                        try {
                                            return a.querySelector(":has(*,:jqfake)"),
                                                !1
                                        } catch {
                                            return !0
                                        }
                                    }),
                                    q.getById ? (t.filter.ID = function(g) {
                                            var v = g.replace(ye, ve);
                                            return function(m) {
                                                return m.getAttribute("id") === v
                                            }
                                        }
                                            ,
                                            t.find.ID = function(g, v) {
                                                if (typeof v.getElementById < "u" && f) {
                                                    var m = v.getElementById(g);
                                                    return m ? [m] : []
                                                }
                                            }
                                    ) : (t.filter.ID = function(g) {
                                            var v = g.replace(ye, ve);
                                            return function(m) {
                                                var T = typeof m.getAttributeNode < "u" && m.getAttributeNode("id");
                                                return T && T.value === v
                                            }
                                        }
                                            ,
                                            t.find.ID = function(g, v) {
                                                if (typeof v.getElementById < "u" && f) {
                                                    var m, T, w, C = v.getElementById(g);
                                                    if (C) {
                                                        if (m = C.getAttributeNode("id"),
                                                        m && m.value === g)
                                                            return [C];
                                                        for (w = v.getElementsByName(g),
                                                                 T = 0; C = w[T++]; )
                                                            if (m = C.getAttributeNode("id"),
                                                            m && m.value === g)
                                                                return [C]
                                                    }
                                                    return []
                                                }
                                            }
                                    ),
                                    t.find.TAG = function(g, v) {
                                        return typeof v.getElementsByTagName < "u" ? v.getElementsByTagName(g) : v.querySelectorAll(g)
                                    }
                                    ,
                                    t.find.CLASS = function(g, v) {
                                        if (typeof v.getElementsByClassName < "u" && f)
                                            return v.getElementsByClassName(g)
                                    }
                                    ,
                                    d = [],
                                    Me(function(g) {
                                        var v;
                                        c.appendChild(g).innerHTML = "<a id='" + b + "' href='' disabled='disabled'></a><select id='" + b + "-\r\\' disabled='disabled'><option selected=''></option></select>",
                                        g.querySelectorAll("[selected]").length || d.push("\\[" + _ + "*(?:value|" + ae + ")"),
                                        g.querySelectorAll("[id~=" + b + "-]").length || d.push("~="),
                                        g.querySelectorAll("a#" + b + "+*").length || d.push(".#.+[+~]"),
                                        g.querySelectorAll(":checked").length || d.push(":checked"),
                                            v = a.createElement("input"),
                                            v.setAttribute("type", "hidden"),
                                            g.appendChild(v).setAttribute("name", "D"),
                                            c.appendChild(g).disabled = !0,
                                        g.querySelectorAll(":disabled").length !== 2 && d.push(":enabled", ":disabled"),
                                            v = a.createElement("input"),
                                            v.setAttribute("name", ""),
                                            g.appendChild(v),
                                        g.querySelectorAll("[name='']").length || d.push("\\[" + _ + "*name" + _ + "*=" + _ + `*(?:''|"")`)
                                    }),
                                q.cssHas || d.push(":has"),
                                    d = d.length && new RegExp(d.join("|")),
                                    U = function(g, v) {
                                        if (g === v)
                                            return o = !0,
                                                0;
                                        var m = !g.compareDocumentPosition - !v.compareDocumentPosition;
                                        return m || (m = (g.ownerDocument || g) == (v.ownerDocument || v) ? g.compareDocumentPosition(v) : 1,
                                            m & 1 || !q.sortDetached && v.compareDocumentPosition(g) === m ? g === a || g.ownerDocument == pe && W.contains(pe, g) ? -1 : v === a || v.ownerDocument == pe && W.contains(pe, v) ? 1 : r ? de.call(r, g) - de.call(r, v) : 0 : m & 4 ? -1 : 1)
                                    }
                            ),
                                a
                        }
                        W.matches = function(s, l) {
                            return W(s, null, null, l)
                        }
                            ,
                            W.matchesSelector = function(s, l) {
                                if (me(s),
                                f && !V[l + " "] && (!d || !d.test(l)))
                                    try {
                                        var p = y.call(s, l);
                                        if (p || q.disconnectedMatch || s.document && s.document.nodeType !== 11)
                                            return p
                                    } catch {
                                        V(l, !0)
                                    }
                                return W(l, a, null, [s]).length > 0
                            }
                            ,
                            W.contains = function(s, l) {
                                return (s.ownerDocument || s) != a && me(s),
                                    i.contains(s, l)
                            }
                            ,
                            W.attr = function(s, l) {
                                (s.ownerDocument || s) != a && me(s);
                                var p = t.attrHandle[l.toLowerCase()]
                                    , g = p && _e.call(t.attrHandle, l.toLowerCase()) ? p(s, l, !f) : void 0;
                                return g !== void 0 ? g : s.getAttribute(l)
                            }
                            ,
                            W.error = function(s) {
                                throw new Error("Syntax error, unrecognized expression: " + s)
                            }
                            ,
                            i.uniqueSort = function(s) {
                                var l, p = [], g = 0, v = 0;
                                if (o = !q.sortStable,
                                    r = !q.sortStable && Z.call(s, 0),
                                    Hn.call(s, U),
                                    o) {
                                    for (; l = s[v++]; )
                                        l === s[v] && (g = p.push(v));
                                    for (; g--; )
                                        On.call(s, p[g], 1)
                                }
                                return r = null,
                                    s
                            }
                            ,
                            i.fn.uniqueSort = function() {
                                return this.pushStack(i.uniqueSort(Z.apply(this)))
                            }
                            ,
                            t = i.expr = {
                                cacheLength: 50,
                                createPseudo: ie,
                                match: ce,
                                attrHandle: {},
                                find: {},
                                relative: {
                                    ">": {
                                        dir: "parentNode",
                                        first: !0
                                    },
                                    " ": {
                                        dir: "parentNode"
                                    },
                                    "+": {
                                        dir: "previousSibling",
                                        first: !0
                                    },
                                    "~": {
                                        dir: "previousSibling"
                                    }
                                },
                                preFilter: {
                                    ATTR: function(s) {
                                        return s[1] = s[1].replace(ye, ve),
                                            s[3] = (s[3] || s[4] || s[5] || "").replace(ye, ve),
                                        s[2] === "~=" && (s[3] = " " + s[3] + " "),
                                            s.slice(0, 4)
                                    },
                                    CHILD: function(s) {
                                        return s[1] = s[1].toLowerCase(),
                                            s[1].slice(0, 3) === "nth" ? (s[3] || W.error(s[0]),
                                                s[4] = +(s[4] ? s[5] + (s[6] || 1) : 2 * (s[3] === "even" || s[3] === "odd")),
                                                s[5] = +(s[7] + s[8] || s[3] === "odd")) : s[3] && W.error(s[0]),
                                            s
                                    },
                                    PSEUDO: function(s) {
                                        var l, p = !s[6] && s[2];
                                        return ce.CHILD.test(s[0]) ? null : (s[3] ? s[2] = s[4] || s[5] || "" : p && fe.test(p) && (l = at(p, !0)) && (l = p.indexOf(")", p.length - l) - p.length) && (s[0] = s[0].slice(0, l),
                                            s[2] = p.slice(0, l)),
                                            s.slice(0, 3))
                                    }
                                },
                                filter: {
                                    TAG: function(s) {
                                        var l = s.replace(ye, ve).toLowerCase();
                                        return s === "*" ? function() {
                                                return !0
                                            }
                                            : function(p) {
                                                return $(p, l)
                                            }
                                    },
                                    CLASS: function(s) {
                                        var l = D[s + " "];
                                        return l || (l = new RegExp("(^|" + _ + ")" + s + "(" + _ + "|$)")) && D(s, function(p) {
                                            return l.test(typeof p.className == "string" && p.className || typeof p.getAttribute < "u" && p.getAttribute("class") || "")
                                        })
                                    },
                                    ATTR: function(s, l, p) {
                                        return function(g) {
                                            var v = W.attr(g, s);
                                            return v == null ? l === "!=" : l ? (v += "",
                                                l === "=" ? v === p : l === "!=" ? v !== p : l === "^=" ? p && v.indexOf(p) === 0 : l === "*=" ? p && v.indexOf(p) > -1 : l === "$=" ? p && v.slice(-p.length) === p : l === "~=" ? (" " + v.replace(I, " ") + " ").indexOf(p) > -1 : l === "|=" ? v === p || v.slice(0, p.length + 1) === p + "-" : !1) : !0
                                        }
                                    },
                                    CHILD: function(s, l, p, g, v) {
                                        var m = s.slice(0, 3) !== "nth"
                                            , T = s.slice(-4) !== "last"
                                            , w = l === "of-type";
                                        return g === 1 && v === 0 ? function(C) {
                                                return !!C.parentNode
                                            }
                                            : function(C, O, A) {
                                                var N, P, E, F, K, X = m !== T ? "nextSibling" : "previousSibling", ne = C.parentNode, le = w && C.nodeName.toLowerCase(), Re = !A && !w, G = !1;
                                                if (ne) {
                                                    if (m) {
                                                        for (; X; ) {
                                                            for (E = C; E = E[X]; )
                                                                if (w ? $(E, le) : E.nodeType === 1)
                                                                    return !1;
                                                            K = X = s === "only" && !K && "nextSibling"
                                                        }
                                                        return !0
                                                    }
                                                    if (K = [T ? ne.firstChild : ne.lastChild],
                                                    T && Re) {
                                                        for (P = ne[b] || (ne[b] = {}),
                                                                 N = P[s] || [],
                                                                 F = N[0] === h && N[1],
                                                                 G = F && N[2],
                                                                 E = F && ne.childNodes[F]; E = ++F && E && E[X] || (G = F = 0) || K.pop(); )
                                                            if (E.nodeType === 1 && ++G && E === C) {
                                                                P[s] = [h, F, G];
                                                                break
                                                            }
                                                    } else if (Re && (P = C[b] || (C[b] = {}),
                                                        N = P[s] || [],
                                                        F = N[0] === h && N[1],
                                                        G = F),
                                                    G === !1)
                                                        for (; (E = ++F && E && E[X] || (G = F = 0) || K.pop()) && !((w ? $(E, le) : E.nodeType === 1) && ++G && (Re && (P = E[b] || (E[b] = {}),
                                                            P[s] = [h, G]),
                                                        E === C)); )
                                                            ;
                                                    return G -= v,
                                                    G === g || G % g === 0 && G / g >= 0
                                                }
                                            }
                                    },
                                    PSEUDO: function(s, l) {
                                        var p, g = t.pseudos[s] || t.setFilters[s.toLowerCase()] || W.error("unsupported pseudo: " + s);
                                        return g[b] ? g(l) : g.length > 1 ? (p = [s, s, "", l],
                                                t.setFilters.hasOwnProperty(s.toLowerCase()) ? ie(function(v, m) {
                                                    for (var T, w = g(v, l), C = w.length; C--; )
                                                        T = de.call(v, w[C]),
                                                            v[T] = !(m[T] = w[C])
                                                }) : function(v) {
                                                    return g(v, 0, p)
                                                }
                                        ) : g
                                    }
                                },
                                pseudos: {
                                    not: ie(function(s) {
                                        var l = []
                                            , p = []
                                            , g = Pt(s.replace(We, "$1"));
                                        return g[b] ? ie(function(v, m, T, w) {
                                            for (var C, O = g(v, null, w, []), A = v.length; A--; )
                                                (C = O[A]) && (v[A] = !(m[A] = C))
                                        }) : function(v, m, T) {
                                            return l[0] = v,
                                                g(l, null, T, p),
                                                l[0] = null,
                                                !p.pop()
                                        }
                                    }),
                                    has: ie(function(s) {
                                        return function(l) {
                                            return W(s, l).length > 0
                                        }
                                    }),
                                    contains: ie(function(s) {
                                        return s = s.replace(ye, ve),
                                            function(l) {
                                                return (l.textContent || i.text(l)).indexOf(s) > -1
                                            }
                                    }),
                                    lang: ie(function(s) {
                                        return Ye.test(s || "") || W.error("unsupported lang: " + s),
                                            s = s.replace(ye, ve).toLowerCase(),
                                            function(l) {
                                                var p;
                                                do
                                                    if (p = f ? l.lang : l.getAttribute("xml:lang") || l.getAttribute("lang"))
                                                        return p = p.toLowerCase(),
                                                        p === s || p.indexOf(s + "-") === 0;
                                                while ((l = l.parentNode) && l.nodeType === 1);
                                                return !1
                                            }
                                    }),
                                    target: function(s) {
                                        var l = k.location && k.location.hash;
                                        return l && l.slice(1) === s.id
                                    },
                                    root: function(s) {
                                        return s === c
                                    },
                                    focus: function(s) {
                                        return s === Hr() && a.hasFocus() && !!(s.type || s.href || ~s.tabIndex)
                                    },
                                    enabled: An(!1),
                                    disabled: An(!0),
                                    checked: function(s) {
                                        return $(s, "input") && !!s.checked || $(s, "option") && !!s.selected
                                    },
                                    selected: function(s) {
                                        return s.parentNode && s.parentNode.selectedIndex,
                                        s.selected === !0
                                    },
                                    empty: function(s) {
                                        for (s = s.firstChild; s; s = s.nextSibling)
                                            if (s.nodeType < 6)
                                                return !1;
                                        return !0
                                    },
                                    parent: function(s) {
                                        return !t.pseudos.empty(s)
                                    },
                                    header: function(s) {
                                        return xe.test(s.nodeName)
                                    },
                                    input: function(s) {
                                        return be.test(s.nodeName)
                                    },
                                    button: function(s) {
                                        return $(s, "input") && s.type === "button" || $(s, "button")
                                    },
                                    text: function(s) {
                                        var l;
                                        return $(s, "input") && s.type === "text" && ((l = s.getAttribute("type")) == null || l.toLowerCase() === "text")
                                    },
                                    first: Ee(function() {
                                        return [0]
                                    }),
                                    last: Ee(function(s, l) {
                                        return [l - 1]
                                    }),
                                    eq: Ee(function(s, l, p) {
                                        return [p < 0 ? p + l : p]
                                    }),
                                    even: Ee(function(s, l) {
                                        for (var p = 0; p < l; p += 2)
                                            s.push(p);
                                        return s
                                    }),
                                    odd: Ee(function(s, l) {
                                        for (var p = 1; p < l; p += 2)
                                            s.push(p);
                                        return s
                                    }),
                                    lt: Ee(function(s, l, p) {
                                        var g;
                                        for (p < 0 ? g = p + l : p > l ? g = l : g = p; --g >= 0; )
                                            s.push(g);
                                        return s
                                    }),
                                    gt: Ee(function(s, l, p) {
                                        for (var g = p < 0 ? p + l : p; ++g < l; )
                                            s.push(g);
                                        return s
                                    })
                                }
                            },
                            t.pseudos.nth = t.pseudos.eq;
                        for (e in {
                            radio: !0,
                            checkbox: !0,
                            file: !0,
                            password: !0,
                            image: !0
                        })
                            t.pseudos[e] = Or(e);
                        for (e in {
                            submit: !0,
                            reset: !0
                        })
                            t.pseudos[e] = Pr(e);
                        function Dn() {}
                        Dn.prototype = t.filters = t.pseudos,
                            t.setFilters = new Dn;
                        function at(s, l) {
                            var p, g, v, m, T, w, C, O = R[s + " "];
                            if (O)
                                return l ? 0 : O.slice(0);
                            for (T = s,
                                     w = [],
                                     C = t.preFilter; T; ) {
                                (!p || (g = B.exec(T))) && (g && (T = T.slice(g[0].length) || T),
                                    w.push(v = [])),
                                    p = !1,
                                (g = Qe.exec(T)) && (p = g.shift(),
                                    v.push({
                                        value: p,
                                        type: g[0].replace(We, " ")
                                    }),
                                    T = T.slice(p.length));
                                for (m in t.filter)
                                    (g = ce[m].exec(T)) && (!C[m] || (g = C[m](g))) && (p = g.shift(),
                                        v.push({
                                            value: p,
                                            type: m,
                                            matches: g
                                        }),
                                        T = T.slice(p.length));
                                if (!p)
                                    break
                            }
                            return l ? T.length : T ? W.error(s) : R(s, w).slice(0)
                        }
                        function st(s) {
                            for (var l = 0, p = s.length, g = ""; l < p; l++)
                                g += s[l].value;
                            return g
                        }
                        function ft(s, l, p) {
                            var g = l.dir
                                , v = l.next
                                , m = v || g
                                , T = p && m === "parentNode"
                                , w = x++;
                            return l.first ? function(C, O, A) {
                                    for (; C = C[g]; )
                                        if (C.nodeType === 1 || T)
                                            return s(C, O, A);
                                    return !1
                                }
                                : function(C, O, A) {
                                    var N, P, E = [h, w];
                                    if (A) {
                                        for (; C = C[g]; )
                                            if ((C.nodeType === 1 || T) && s(C, O, A))
                                                return !0
                                    } else
                                        for (; C = C[g]; )
                                            if (C.nodeType === 1 || T)
                                                if (P = C[b] || (C[b] = {}),
                                                v && $(C, v))
                                                    C = C[g] || C;
                                                else {
                                                    if ((N = P[m]) && N[0] === h && N[1] === w)
                                                        return E[2] = N[2];
                                                    if (P[m] = E,
                                                        E[2] = s(C, O, A))
                                                        return !0
                                                }
                                    return !1
                                }
                        }
                        function Lt(s) {
                            return s.length > 1 ? function(l, p, g) {
                                    for (var v = s.length; v--; )
                                        if (!s[v](l, p, g))
                                            return !1;
                                    return !0
                                }
                                : s[0]
                        }
                        function Mr(s, l, p) {
                            for (var g = 0, v = l.length; g < v; g++)
                                W(s, l[g], p);
                            return p
                        }
                        function ct(s, l, p, g, v) {
                            for (var m, T = [], w = 0, C = s.length, O = l != null; w < C; w++)
                                (m = s[w]) && (!p || p(m, g, v)) && (T.push(m),
                                O && l.push(w));
                            return T
                        }
                        function Ht(s, l, p, g, v, m) {
                            return g && !g[b] && (g = Ht(g)),
                            v && !v[b] && (v = Ht(v, m)),
                                ie(function(T, w, C, O) {
                                    var A, N, P, E, F = [], K = [], X = w.length, ne = T || Mr(l || "*", C.nodeType ? [C] : C, []), le = s && (T || !l) ? ct(ne, F, s, C, O) : ne;
                                    if (p ? (E = v || (T ? s : X || g) ? [] : w,
                                        p(le, E, C, O)) : E = le,
                                        g)
                                        for (A = ct(E, K),
                                                 g(A, [], C, O),
                                                 N = A.length; N--; )
                                            (P = A[N]) && (E[K[N]] = !(le[K[N]] = P));
                                    if (T) {
                                        if (v || s) {
                                            if (v) {
                                                for (A = [],
                                                         N = E.length; N--; )
                                                    (P = E[N]) && A.push(le[N] = P);
                                                v(null, E = [], A, O)
                                            }
                                            for (N = E.length; N--; )
                                                (P = E[N]) && (A = v ? de.call(T, P) : F[N]) > -1 && (T[A] = !(w[A] = P))
                                        }
                                    } else
                                        E = ct(E === w ? E.splice(X, E.length) : E),
                                            v ? v(null, w, E, O) : u.apply(w, E)
                                })
                        }
                        function Ot(s) {
                            for (var l, p, g, v = s.length, m = t.relative[s[0].type], T = m || t.relative[" "], w = m ? 1 : 0, C = ft(function(N) {
                                return N === l
                            }, T, !0), O = ft(function(N) {
                                return de.call(l, N) > -1
                            }, T, !0), A = [function(N, P, E) {
                                var F = !m && (E || P != n) || ((l = P).nodeType ? C(N, P, E) : O(N, P, E));
                                return l = null,
                                    F
                            }
                            ]; w < v; w++)
                                if (p = t.relative[s[w].type])
                                    A = [ft(Lt(A), p)];
                                else {
                                    if (p = t.filter[s[w].type].apply(null, s[w].matches),
                                        p[b]) {
                                        for (g = ++w; g < v && !t.relative[s[g].type]; g++)
                                            ;
                                        return Ht(w > 1 && Lt(A), w > 1 && st(s.slice(0, w - 1).concat({
                                            value: s[w - 2].type === " " ? "*" : ""
                                        })).replace(We, "$1"), p, w < g && Ot(s.slice(w, g)), g < v && Ot(s = s.slice(g)), g < v && st(s))
                                    }
                                    A.push(p)
                                }
                            return Lt(A)
                        }
                        function Rr(s, l) {
                            var p = l.length > 0
                                , g = s.length > 0
                                , v = function(m, T, w, C, O) {
                                var A, N, P, E = 0, F = "0", K = m && [], X = [], ne = n, le = m || g && t.find.TAG("*", O), Re = h += ne == null ? 1 : Math.random() || .1, G = le.length;
                                for (O && (n = T == a || T || O); F !== G && (A = le[F]) != null; F++) {
                                    if (g && A) {
                                        for (N = 0,
                                             !T && A.ownerDocument != a && (me(A),
                                                 w = !f); P = s[N++]; )
                                            if (P(A, T || a, w)) {
                                                u.call(C, A);
                                                break
                                            }
                                        O && (h = Re)
                                    }
                                    p && ((A = !P && A) && E--,
                                    m && K.push(A))
                                }
                                if (E += F,
                                p && F !== E) {
                                    for (N = 0; P = l[N++]; )
                                        P(K, X, T, w);
                                    if (m) {
                                        if (E > 0)
                                            for (; F--; )
                                                K[F] || X[F] || (X[F] = Ln.call(C));
                                        X = ct(X)
                                    }
                                    u.apply(C, X),
                                    O && !m && X.length > 0 && E + l.length > 1 && i.uniqueSort(C)
                                }
                                return O && (h = Re,
                                    n = ne),
                                    K
                            };
                            return p ? ie(v) : v
                        }
                        function Pt(s, l) {
                            var p, g = [], v = [], m = H[s + " "];
                            if (!m) {
                                for (l || (l = at(s)),
                                         p = l.length; p--; )
                                    m = Ot(l[p]),
                                        m[b] ? g.push(m) : v.push(m);
                                m = H(s, Rr(v, g)),
                                    m.selector = s
                            }
                            return m
                        }
                        function Nn(s, l, p, g) {
                            var v, m, T, w, C, O = typeof s == "function" && s, A = !g && at(s = O.selector || s);
                            if (p = p || [],
                            A.length === 1) {
                                if (m = A[0] = A[0].slice(0),
                                m.length > 2 && (T = m[0]).type === "ID" && l.nodeType === 9 && f && t.relative[m[1].type]) {
                                    if (l = (t.find.ID(T.matches[0].replace(ye, ve), l) || [])[0],
                                        l)
                                        O && (l = l.parentNode);
                                    else
                                        return p;
                                    s = s.slice(m.shift().value.length)
                                }
                                for (v = ce.needsContext.test(s) ? 0 : m.length; v-- && (T = m[v],
                                    !t.relative[w = T.type]); )
                                    if ((C = t.find[w]) && (g = C(T.matches[0].replace(ye, ve), jt.test(m[0].type) && qt(l.parentNode) || l))) {
                                        if (m.splice(v, 1),
                                            s = g.length && st(m),
                                            !s)
                                            return u.apply(p, g),
                                                p;
                                        break
                                    }
                            }
                            return (O || Pt(s, A))(g, l, !f, p, !l || jt.test(s) && qt(l.parentNode) || l),
                                p
                        }
                        q.sortStable = b.split("").sort(U).join("") === b,
                            me(),
                            q.sortDetached = Me(function(s) {
                                return s.compareDocumentPosition(a.createElement("fieldset")) & 1
                            }),
                            i.find = W,
                            i.expr[":"] = i.expr.pseudos,
                            i.unique = i.uniqueSort,
                            W.compile = Pt,
                            W.select = Nn,
                            W.setDocument = me,
                            W.escape = i.escapeSelector,
                            W.getText = i.text,
                            W.isXML = i.isXMLDoc,
                            W.selectors = i.expr,
                            W.support = i.support,
                            W.uniqueSort = i.uniqueSort
                    }
                )();
                var Ne = function(e, t, n) {
                    for (var r = [], o = n !== void 0; (e = e[t]) && e.nodeType !== 9; )
                        if (e.nodeType === 1) {
                            if (o && i(e).is(n))
                                break;
                            r.push(e)
                        }
                    return r
                }
                    , $t = function(e, t) {
                    for (var n = []; e; e = e.nextSibling)
                        e.nodeType === 1 && e !== t && n.push(e);
                    return n
                }
                    , Bt = i.expr.match.needsContext
                    , Ut = /^<([a-z][^\/\0>:\x20\t\r\n\f]*)[\x20\t\r\n\f]*\/?>(?:<\/\1>|)$/i;
                function pt(e, t, n) {
                    return L(t) ? i.grep(e, function(r, o) {
                        return !!t.call(r, o, r) !== n
                    }) : t.nodeType ? i.grep(e, function(r) {
                        return r === t !== n
                    }) : typeof t != "string" ? i.grep(e, function(r) {
                        return de.call(t, r) > -1 !== n
                    }) : i.filter(t, e, n)
                }
                i.filter = function(e, t, n) {
                    var r = t[0];
                    return n && (e = ":not(" + e + ")"),
                        t.length === 1 && r.nodeType === 1 ? i.find.matchesSelector(r, e) ? [r] : [] : i.find.matches(e, i.grep(t, function(o) {
                            return o.nodeType === 1
                        }))
                }
                    ,
                    i.fn.extend({
                        find: function(e) {
                            var t, n, r = this.length, o = this;
                            if (typeof e != "string")
                                return this.pushStack(i(e).filter(function() {
                                    for (t = 0; t < r; t++)
                                        if (i.contains(o[t], this))
                                            return !0
                                }));
                            for (n = this.pushStack([]),
                                     t = 0; t < r; t++)
                                i.find(e, o[t], n);
                            return r > 1 ? i.uniqueSort(n) : n
                        },
                        filter: function(e) {
                            return this.pushStack(pt(this, e || [], !1))
                        },
                        not: function(e) {
                            return this.pushStack(pt(this, e || [], !0))
                        },
                        is: function(e) {
                            return !!pt(this, typeof e == "string" && Bt.test(e) ? i(e) : e || [], !1).length
                        }
                    });
                var zt, Rn = /^(?:\s*(<[\w\W]+>)[^>]*|#([\w-]+))$/, In = i.fn.init = function(e, t, n) {
                        var r, o;
                        if (!e)
                            return this;
                        if (n = n || zt,
                        typeof e == "string")
                            if (e[0] === "<" && e[e.length - 1] === ">" && e.length >= 3 ? r = [null, e, null] : r = Rn.exec(e),
                            r && (r[1] || !t))
                                if (r[1]) {
                                    if (t = t instanceof i ? t[0] : t,
                                        i.merge(this, i.parseHTML(r[1], t && t.nodeType ? t.ownerDocument || t : j, !0)),
                                    Ut.test(r[1]) && i.isPlainObject(t))
                                        for (r in t)
                                            L(this[r]) ? this[r](t[r]) : this.attr(r, t[r]);
                                    return this
                                } else
                                    return o = j.getElementById(r[2]),
                                    o && (this[0] = o,
                                        this.length = 1),
                                        this;
                            else
                                return !t || t.jquery ? (t || n).find(e) : this.constructor(t).find(e);
                        else {
                            if (e.nodeType)
                                return this[0] = e,
                                    this.length = 1,
                                    this;
                            if (L(e))
                                return n.ready !== void 0 ? n.ready(e) : e(i)
                        }
                        return i.makeArray(e, this)
                    }
                ;
                In.prototype = i.fn,
                    zt = i(j);
                var _n = /^(?:parents|prev(?:Until|All))/
                    , Wn = {
                    children: !0,
                    contents: !0,
                    next: !0,
                    prev: !0
                };
                i.fn.extend({
                    has: function(e) {
                        var t = i(e, this)
                            , n = t.length;
                        return this.filter(function() {
                            for (var r = 0; r < n; r++)
                                if (i.contains(this, t[r]))
                                    return !0
                        })
                    },
                    closest: function(e, t) {
                        var n, r = 0, o = this.length, u = [], a = typeof e != "string" && i(e);
                        if (!Bt.test(e)) {
                            for (; r < o; r++)
                                for (n = this[r]; n && n !== t; n = n.parentNode)
                                    if (n.nodeType < 11 && (a ? a.index(n) > -1 : n.nodeType === 1 && i.find.matchesSelector(n, e))) {
                                        u.push(n);
                                        break
                                    }
                        }
                        return this.pushStack(u.length > 1 ? i.uniqueSort(u) : u)
                    },
                    index: function(e) {
                        return e ? typeof e == "string" ? de.call(i(e), this[0]) : de.call(this, e.jquery ? e[0] : e) : this[0] && this[0].parentNode ? this.first().prevAll().length : -1
                    },
                    add: function(e, t) {
                        return this.pushStack(i.uniqueSort(i.merge(this.get(), i(e, t))))
                    },
                    addBack: function(e) {
                        return this.add(e == null ? this.prevObject : this.prevObject.filter(e))
                    }
                });
                function Vt(e, t) {
                    for (; (e = e[t]) && e.nodeType !== 1; )
                        ;
                    return e
                }
                i.each({
                    parent: function(e) {
                        var t = e.parentNode;
                        return t && t.nodeType !== 11 ? t : null
                    },
                    parents: function(e) {
                        return Ne(e, "parentNode")
                    },
                    parentsUntil: function(e, t, n) {
                        return Ne(e, "parentNode", n)
                    },
                    next: function(e) {
                        return Vt(e, "nextSibling")
                    },
                    prev: function(e) {
                        return Vt(e, "previousSibling")
                    },
                    nextAll: function(e) {
                        return Ne(e, "nextSibling")
                    },
                    prevAll: function(e) {
                        return Ne(e, "previousSibling")
                    },
                    nextUntil: function(e, t, n) {
                        return Ne(e, "nextSibling", n)
                    },
                    prevUntil: function(e, t, n) {
                        return Ne(e, "previousSibling", n)
                    },
                    siblings: function(e) {
                        return $t((e.parentNode || {}).firstChild, e)
                    },
                    children: function(e) {
                        return $t(e.firstChild)
                    },
                    contents: function(e) {
                        return e.contentDocument != null && Mt(e.contentDocument) ? e.contentDocument : ($(e, "template") && (e = e.content || e),
                            i.merge([], e.childNodes))
                    }
                }, function(e, t) {
                    i.fn[e] = function(n, r) {
                        var o = i.map(this, t, n);
                        return e.slice(-5) !== "Until" && (r = n),
                        r && typeof r == "string" && (o = i.filter(r, o)),
                        this.length > 1 && (Wn[e] || i.uniqueSort(o),
                        _n.test(e) && o.reverse()),
                            this.pushStack(o)
                    }
                });
                var oe = /[^\x20\t\r\n\f]+/g;
                function Fn(e) {
                    var t = {};
                    return i.each(e.match(oe) || [], function(n, r) {
                        t[r] = !0
                    }),
                        t
                }
                i.Callbacks = function(e) {
                    e = typeof e == "string" ? Fn(e) : i.extend({}, e);
                    var t, n, r, o, u = [], a = [], c = -1, f = function() {
                        for (o = o || e.once,
                                 r = t = !0; a.length; c = -1)
                            for (n = a.shift(); ++c < u.length; )
                                u[c].apply(n[0], n[1]) === !1 && e.stopOnFalse && (c = u.length,
                                    n = !1);
                        e.memory || (n = !1),
                            t = !1,
                        o && (n ? u = [] : u = "")
                    }, d = {
                        add: function() {
                            return u && (n && !t && (c = u.length - 1,
                                a.push(n)),
                                function y(b) {
                                    i.each(b, function(h, x) {
                                        L(x) ? (!e.unique || !d.has(x)) && u.push(x) : x && x.length && De(x) !== "string" && y(x)
                                    })
                                }(arguments),
                            n && !t && f()),
                                this
                        },
                        remove: function() {
                            return i.each(arguments, function(y, b) {
                                for (var h; (h = i.inArray(b, u, h)) > -1; )
                                    u.splice(h, 1),
                                    h <= c && c--
                            }),
                                this
                        },
                        has: function(y) {
                            return y ? i.inArray(y, u) > -1 : u.length > 0
                        },
                        empty: function() {
                            return u && (u = []),
                                this
                        },
                        disable: function() {
                            return o = a = [],
                                u = n = "",
                                this
                        },
                        disabled: function() {
                            return !u
                        },
                        lock: function() {
                            return o = a = [],
                            !n && !t && (u = n = ""),
                                this
                        },
                        locked: function() {
                            return !!o
                        },
                        fireWith: function(y, b) {
                            return o || (b = b || [],
                                b = [y, b.slice ? b.slice() : b],
                                a.push(b),
                            t || f()),
                                this
                        },
                        fire: function() {
                            return d.fireWith(this, arguments),
                                this
                        },
                        fired: function() {
                            return !!r
                        }
                    };
                    return d
                }
                ;
                function ke(e) {
                    return e
                }
                function Ze(e) {
                    throw e
                }
                function Xt(e, t, n, r) {
                    var o;
                    try {
                        e && L(o = e.promise) ? o.call(e).done(t).fail(n) : e && L(o = e.then) ? o.call(e, t, n) : t.apply(void 0, [e].slice(r))
                    } catch (u) {
                        n.apply(void 0, [u])
                    }
                }
                i.extend({
                    Deferred: function(e) {
                        var t = [["notify", "progress", i.Callbacks("memory"), i.Callbacks("memory"), 2], ["resolve", "done", i.Callbacks("once memory"), i.Callbacks("once memory"), 0, "resolved"], ["reject", "fail", i.Callbacks("once memory"), i.Callbacks("once memory"), 1, "rejected"]]
                            , n = "pending"
                            , r = {
                            state: function() {
                                return n
                            },
                            always: function() {
                                return o.done(arguments).fail(arguments),
                                    this
                            },
                            catch: function(u) {
                                return r.then(null, u)
                            },
                            pipe: function() {
                                var u = arguments;
                                return i.Deferred(function(a) {
                                    i.each(t, function(c, f) {
                                        var d = L(u[f[4]]) && u[f[4]];
                                        o[f[1]](function() {
                                            var y = d && d.apply(this, arguments);
                                            y && L(y.promise) ? y.promise().progress(a.notify).done(a.resolve).fail(a.reject) : a[f[0] + "With"](this, d ? [y] : arguments)
                                        })
                                    }),
                                        u = null
                                }).promise()
                            },
                            then: function(u, a, c) {
                                var f = 0;
                                function d(y, b, h, x) {
                                    return function() {
                                        var D = this
                                            , R = arguments
                                            , H = function() {
                                                var U, ae;
                                                if (!(y < f)) {
                                                    if (U = h.apply(D, R),
                                                    U === b.promise())
                                                        throw new TypeError("Thenable self-resolution");
                                                    ae = U && (typeof U == "object" || typeof U == "function") && U.then,
                                                        L(ae) ? x ? ae.call(U, d(f, b, ke, x), d(f, b, Ze, x)) : (f++,
                                                            ae.call(U, d(f, b, ke, x), d(f, b, Ze, x), d(f, b, ke, b.notifyWith))) : (h !== ke && (D = void 0,
                                                            R = [U]),
                                                            (x || b.resolveWith)(D, R))
                                                }
                                            }
                                            , V = x ? H : function() {
                                                try {
                                                    H()
                                                } catch (U) {
                                                    i.Deferred.exceptionHook && i.Deferred.exceptionHook(U, V.error),
                                                    y + 1 >= f && (h !== Ze && (D = void 0,
                                                        R = [U]),
                                                        b.rejectWith(D, R))
                                                }
                                            }
                                        ;
                                        y ? V() : (i.Deferred.getErrorHook ? V.error = i.Deferred.getErrorHook() : i.Deferred.getStackHook && (V.error = i.Deferred.getStackHook()),
                                            k.setTimeout(V))
                                    }
                                }
                                return i.Deferred(function(y) {
                                    t[0][3].add(d(0, y, L(c) ? c : ke, y.notifyWith)),
                                        t[1][3].add(d(0, y, L(u) ? u : ke)),
                                        t[2][3].add(d(0, y, L(a) ? a : Ze))
                                }).promise()
                            },
                            promise: function(u) {
                                return u != null ? i.extend(u, r) : r
                            }
                        }
                            , o = {};
                        return i.each(t, function(u, a) {
                            var c = a[2]
                                , f = a[5];
                            r[a[1]] = c.add,
                            f && c.add(function() {
                                n = f
                            }, t[3 - u][2].disable, t[3 - u][3].disable, t[0][2].lock, t[0][3].lock),
                                c.add(a[3].fire),
                                o[a[0]] = function() {
                                    return o[a[0] + "With"](this === o ? void 0 : this, arguments),
                                        this
                                }
                                ,
                                o[a[0] + "With"] = c.fireWith
                        }),
                            r.promise(o),
                        e && e.call(o, o),
                            o
                    },
                    when: function(e) {
                        var t = arguments.length
                            , n = t
                            , r = Array(n)
                            , o = Z.call(arguments)
                            , u = i.Deferred()
                            , a = function(c) {
                            return function(f) {
                                r[c] = this,
                                    o[c] = arguments.length > 1 ? Z.call(arguments) : f,
                                --t || u.resolveWith(r, o)
                            }
                        };
                        if (t <= 1 && (Xt(e, u.done(a(n)).resolve, u.reject, !t),
                        u.state() === "pending" || L(o[n] && o[n].then)))
                            return u.then();
                        for (; n--; )
                            Xt(o[n], a(n), u.reject);
                        return u.promise()
                    }
                });
                var $n = /^(Eval|Internal|Range|Reference|Syntax|Type|URI)Error$/;
                i.Deferred.exceptionHook = function(e, t) {
                    k.console && k.console.warn && e && $n.test(e.name) && k.console.warn("jQuery.Deferred exception: " + e.message, e.stack, t)
                }
                    ,
                    i.readyException = function(e) {
                        k.setTimeout(function() {
                            throw e
                        })
                    }
                ;
                var ht = i.Deferred();
                i.fn.ready = function(e) {
                    return ht.then(e).catch(function(t) {
                        i.readyException(t)
                    }),
                        this
                }
                    ,
                    i.extend({
                        isReady: !1,
                        readyWait: 1,
                        ready: function(e) {
                            (e === !0 ? --i.readyWait : i.isReady) || (i.isReady = !0,
                            !(e !== !0 && --i.readyWait > 0) && ht.resolveWith(j, [i]))
                        }
                    }),
                    i.ready.then = ht.then;
                function et() {
                    j.removeEventListener("DOMContentLoaded", et),
                        k.removeEventListener("load", et),
                        i.ready()
                }
                j.readyState === "complete" || j.readyState !== "loading" && !j.documentElement.doScroll ? k.setTimeout(i.ready) : (j.addEventListener("DOMContentLoaded", et),
                    k.addEventListener("load", et));
                var he = function(e, t, n, r, o, u, a) {
                    var c = 0
                        , f = e.length
                        , d = n == null;
                    if (De(n) === "object") {
                        o = !0;
                        for (c in n)
                            he(e, t, c, n[c], !0, u, a)
                    } else if (r !== void 0 && (o = !0,
                    L(r) || (a = !0),
                    d && (a ? (t.call(e, r),
                        t = null) : (d = t,
                            t = function(y, b, h) {
                                return d.call(i(y), h)
                            }
                    )),
                        t))
                        for (; c < f; c++)
                            t(e[c], n, a ? r : r.call(e[c], c, t(e[c], n)));
                    return o ? e : d ? t.call(e) : f ? t(e[0], n) : u
                }
                    , Bn = /^-ms-/
                    , Un = /-([a-z])/g;
                function zn(e, t) {
                    return t.toUpperCase()
                }
                function ue(e) {
                    return e.replace(Bn, "ms-").replace(Un, zn)
                }
                var Fe = function(e) {
                    return e.nodeType === 1 || e.nodeType === 9 || !+e.nodeType
                };
                function $e() {
                    this.expando = i.expando + $e.uid++
                }
                $e.uid = 1,
                    $e.prototype = {
                        cache: function(e) {
                            var t = e[this.expando];
                            return t || (t = {},
                            Fe(e) && (e.nodeType ? e[this.expando] = t : Object.defineProperty(e, this.expando, {
                                value: t,
                                configurable: !0
                            }))),
                                t
                        },
                        set: function(e, t, n) {
                            var r, o = this.cache(e);
                            if (typeof t == "string")
                                o[ue(t)] = n;
                            else
                                for (r in t)
                                    o[ue(r)] = t[r];
                            return o
                        },
                        get: function(e, t) {
                            return t === void 0 ? this.cache(e) : e[this.expando] && e[this.expando][ue(t)]
                        },
                        access: function(e, t, n) {
                            return t === void 0 || t && typeof t == "string" && n === void 0 ? this.get(e, t) : (this.set(e, t, n),
                                n !== void 0 ? n : t)
                        },
                        remove: function(e, t) {
                            var n, r = e[this.expando];
                            if (r !== void 0) {
                                if (t !== void 0)
                                    for (Array.isArray(t) ? t = t.map(ue) : (t = ue(t),
                                        t = t in r ? [t] : t.match(oe) || []),
                                             n = t.length; n--; )
                                        delete r[t[n]];
                                (t === void 0 || i.isEmptyObject(r)) && (e.nodeType ? e[this.expando] = void 0 : delete e[this.expando])
                            }
                        },
                        hasData: function(e) {
                            var t = e[this.expando];
                            return t !== void 0 && !i.isEmptyObject(t)
                        }
                    };
                var S = new $e
                    , Q = new $e
                    , Vn = /^(?:\{[\w\W]*\}|\[[\w\W]*\])$/
                    , Xn = /[A-Z]/g;
                function Gn(e) {
                    return e === "true" ? !0 : e === "false" ? !1 : e === "null" ? null : e === +e + "" ? +e : Vn.test(e) ? JSON.parse(e) : e
                }
                function Gt(e, t, n) {
                    var r;
                    if (n === void 0 && e.nodeType === 1)
                        if (r = "data-" + t.replace(Xn, "-$&").toLowerCase(),
                            n = e.getAttribute(r),
                        typeof n == "string") {
                            try {
                                n = Gn(n)
                            } catch {}
                            Q.set(e, t, n)
                        } else
                            n = void 0;
                    return n
                }
                i.extend({
                    hasData: function(e) {
                        return Q.hasData(e) || S.hasData(e)
                    },
                    data: function(e, t, n) {
                        return Q.access(e, t, n)
                    },
                    removeData: function(e, t) {
                        Q.remove(e, t)
                    },
                    _data: function(e, t, n) {
                        return S.access(e, t, n)
                    },
                    _removeData: function(e, t) {
                        S.remove(e, t)
                    }
                }),
                    i.fn.extend({
                        data: function(e, t) {
                            var n, r, o, u = this[0], a = u && u.attributes;
                            if (e === void 0) {
                                if (this.length && (o = Q.get(u),
                                u.nodeType === 1 && !S.get(u, "hasDataAttrs"))) {
                                    for (n = a.length; n--; )
                                        a[n] && (r = a[n].name,
                                        r.indexOf("data-") === 0 && (r = ue(r.slice(5)),
                                            Gt(u, r, o[r])));
                                    S.set(u, "hasDataAttrs", !0)
                                }
                                return o
                            }
                            return typeof e == "object" ? this.each(function() {
                                Q.set(this, e)
                            }) : he(this, function(c) {
                                var f;
                                if (u && c === void 0)
                                    return f = Q.get(u, e),
                                        f !== void 0 || (f = Gt(u, e),
                                        f !== void 0) ? f : void 0;
                                this.each(function() {
                                    Q.set(this, e, c)
                                })
                            }, null, t, arguments.length > 1, null, !0)
                        },
                        removeData: function(e) {
                            return this.each(function() {
                                Q.remove(this, e)
                            })
                        }
                    }),
                    i.extend({
                        queue: function(e, t, n) {
                            var r;
                            if (e)
                                return t = (t || "fx") + "queue",
                                    r = S.get(e, t),
                                n && (!r || Array.isArray(n) ? r = S.access(e, t, i.makeArray(n)) : r.push(n)),
                                r || []
                        },
                        dequeue: function(e, t) {
                            t = t || "fx";
                            var n = i.queue(e, t)
                                , r = n.length
                                , o = n.shift()
                                , u = i._queueHooks(e, t)
                                , a = function() {
                                i.dequeue(e, t)
                            };
                            o === "inprogress" && (o = n.shift(),
                                r--),
                            o && (t === "fx" && n.unshift("inprogress"),
                                delete u.stop,
                                o.call(e, a, u)),
                            !r && u && u.empty.fire()
                        },
                        _queueHooks: function(e, t) {
                            var n = t + "queueHooks";
                            return S.get(e, n) || S.access(e, n, {
                                empty: i.Callbacks("once memory").add(function() {
                                    S.remove(e, [t + "queue", n])
                                })
                            })
                        }
                    }),
                    i.fn.extend({
                        queue: function(e, t) {
                            var n = 2;
                            return typeof e != "string" && (t = e,
                                e = "fx",
                                n--),
                                arguments.length < n ? i.queue(this[0], e) : t === void 0 ? this : this.each(function() {
                                    var r = i.queue(this, e, t);
                                    i._queueHooks(this, e),
                                    e === "fx" && r[0] !== "inprogress" && i.dequeue(this, e)
                                })
                        },
                        dequeue: function(e) {
                            return this.each(function() {
                                i.dequeue(this, e)
                            })
                        },
                        clearQueue: function(e) {
                            return this.queue(e || "fx", [])
                        },
                        promise: function(e, t) {
                            var n, r = 1, o = i.Deferred(), u = this, a = this.length, c = function() {
                                --r || o.resolveWith(u, [u])
                            };
                            for (typeof e != "string" && (t = e,
                                e = void 0),
                                     e = e || "fx"; a--; )
                                n = S.get(u[a], e + "queueHooks"),
                                n && n.empty && (r++,
                                    n.empty.add(c));
                            return c(),
                                o.promise(t)
                        }
                    });
                var Qt = /[+-]?(?:\d*\.|)\d+(?:[eE][+-]?\d+|)/.source
                    , Be = new RegExp("^(?:([+-])=|)(" + Qt + ")([a-z%]*)$","i")
                    , ge = ["Top", "Right", "Bottom", "Left"]
                    , Te = j.documentElement
                    , je = function(e) {
                    return i.contains(e.ownerDocument, e)
                }
                    , Qn = {
                    composed: !0
                };
                Te.getRootNode && (je = function(e) {
                        return i.contains(e.ownerDocument, e) || e.getRootNode(Qn) === e.ownerDocument
                    }
                );
                var tt = function(e, t) {
                    return e = t || e,
                    e.style.display === "none" || e.style.display === "" && je(e) && i.css(e, "display") === "none"
                };
                function Yt(e, t, n, r) {
                    var o, u, a = 20, c = r ? function() {
                            return r.cur()
                        }
                        : function() {
                            return i.css(e, t, "")
                        }
                        , f = c(), d = n && n[3] || (i.cssNumber[t] ? "" : "px"), y = e.nodeType && (i.cssNumber[t] || d !== "px" && +f) && Be.exec(i.css(e, t));
                    if (y && y[3] !== d) {
                        for (f = f / 2,
                                 d = d || y[3],
                                 y = +f || 1; a--; )
                            i.style(e, t, y + d),
                            (1 - u) * (1 - (u = c() / f || .5)) <= 0 && (a = 0),
                                y = y / u;
                        y = y * 2,
                            i.style(e, t, y + d),
                            n = n || []
                    }
                    return n && (y = +y || +f || 0,
                        o = n[1] ? y + (n[1] + 1) * n[2] : +n[2],
                    r && (r.unit = d,
                        r.start = y,
                        r.end = o)),
                        o
                }
                var Jt = {};
                function Yn(e) {
                    var t, n = e.ownerDocument, r = e.nodeName, o = Jt[r];
                    return o || (t = n.body.appendChild(n.createElement(r)),
                        o = i.css(t, "display"),
                        t.parentNode.removeChild(t),
                    o === "none" && (o = "block"),
                        Jt[r] = o,
                        o)
                }
                function qe(e, t) {
                    for (var n, r, o = [], u = 0, a = e.length; u < a; u++)
                        r = e[u],
                        r.style && (n = r.style.display,
                            t ? (n === "none" && (o[u] = S.get(r, "display") || null,
                            o[u] || (r.style.display = "")),
                            r.style.display === "" && tt(r) && (o[u] = Yn(r))) : n !== "none" && (o[u] = "none",
                                S.set(r, "display", n)));
                    for (u = 0; u < a; u++)
                        o[u] != null && (e[u].style.display = o[u]);
                    return e
                }
                i.fn.extend({
                    show: function() {
                        return qe(this, !0)
                    },
                    hide: function() {
                        return qe(this)
                    },
                    toggle: function(e) {
                        return typeof e == "boolean" ? e ? this.show() : this.hide() : this.each(function() {
                            tt(this) ? i(this).show() : i(this).hide()
                        })
                    }
                });
                var Ue = /^(?:checkbox|radio)$/i
                    , Kt = /<([a-z][^\/\0>\x20\t\r\n\f]*)/i
                    , Zt = /^$|^module$|\/(?:java|ecma)script/i;
                (function() {
                        var e = j.createDocumentFragment()
                            , t = e.appendChild(j.createElement("div"))
                            , n = j.createElement("input");
                        n.setAttribute("type", "radio"),
                            n.setAttribute("checked", "checked"),
                            n.setAttribute("name", "t"),
                            t.appendChild(n),
                            q.checkClone = t.cloneNode(!0).cloneNode(!0).lastChild.checked,
                            t.innerHTML = "<textarea>x</textarea>",
                            q.noCloneChecked = !!t.cloneNode(!0).lastChild.defaultValue,
                            t.innerHTML = "<option></option>",
                            q.option = !!t.lastChild
                    }
                )();
                var ee = {
                    thead: [1, "<table>", "</table>"],
                    col: [2, "<table><colgroup>", "</colgroup></table>"],
                    tr: [2, "<table><tbody>", "</tbody></table>"],
                    td: [3, "<table><tbody><tr>", "</tr></tbody></table>"],
                    _default: [0, "", ""]
                };
                ee.tbody = ee.tfoot = ee.colgroup = ee.caption = ee.thead,
                    ee.th = ee.td,
                q.option || (ee.optgroup = ee.option = [1, "<select multiple='multiple'>", "</select>"]);
                function Y(e, t) {
                    var n;
                    return typeof e.getElementsByTagName < "u" ? n = e.getElementsByTagName(t || "*") : typeof e.querySelectorAll < "u" ? n = e.querySelectorAll(t || "*") : n = [],
                        t === void 0 || t && $(e, t) ? i.merge([e], n) : n
                }
                function gt(e, t) {
                    for (var n = 0, r = e.length; n < r; n++)
                        S.set(e[n], "globalEval", !t || S.get(t[n], "globalEval"))
                }
                var Jn = /<|&#?\w+;/;
                function en(e, t, n, r, o) {
                    for (var u, a, c, f, d, y, b = t.createDocumentFragment(), h = [], x = 0, D = e.length; x < D; x++)
                        if (u = e[x],
                        u || u === 0)
                            if (De(u) === "object")
                                i.merge(h, u.nodeType ? [u] : u);
                            else if (!Jn.test(u))
                                h.push(t.createTextNode(u));
                            else {
                                for (a = a || b.appendChild(t.createElement("div")),
                                         c = (Kt.exec(u) || ["", ""])[1].toLowerCase(),
                                         f = ee[c] || ee._default,
                                         a.innerHTML = f[1] + i.htmlPrefilter(u) + f[2],
                                         y = f[0]; y--; )
                                    a = a.lastChild;
                                i.merge(h, a.childNodes),
                                    a = b.firstChild,
                                    a.textContent = ""
                            }
                    for (b.textContent = "",
                             x = 0; u = h[x++]; ) {
                        if (r && i.inArray(u, r) > -1) {
                            o && o.push(u);
                            continue
                        }
                        if (d = je(u),
                            a = Y(b.appendChild(u), "script"),
                        d && gt(a),
                            n)
                            for (y = 0; u = a[y++]; )
                                Zt.test(u.type || "") && n.push(u)
                    }
                    return b
                }
                var tn = /^([^.]*)(?:\.(.+)|)/;
                function Le() {
                    return !0
                }
                function He() {
                    return !1
                }
                function yt(e, t, n, r, o, u) {
                    var a, c;
                    if (typeof t == "object") {
                        typeof n != "string" && (r = r || n,
                            n = void 0);
                        for (c in t)
                            yt(e, c, n, r, t[c], u);
                        return e
                    }
                    if (r == null && o == null ? (o = n,
                        r = n = void 0) : o == null && (typeof n == "string" ? (o = r,
                        r = void 0) : (o = r,
                        r = n,
                        n = void 0)),
                    o === !1)
                        o = He;
                    else if (!o)
                        return e;
                    return u === 1 && (a = o,
                        o = function(f) {
                            return i().off(f),
                                a.apply(this, arguments)
                        }
                        ,
                        o.guid = a.guid || (a.guid = i.guid++)),
                        e.each(function() {
                            i.event.add(this, t, o, r, n)
                        })
                }
                i.event = {
                    global: {},
                    add: function(e, t, n, r, o) {
                        var u, a, c, f, d, y, b, h, x, D, R, H = S.get(e);
                        if (Fe(e))
                            for (n.handler && (u = n,
                                n = u.handler,
                                o = u.selector),
                                 o && i.find.matchesSelector(Te, o),
                                 n.guid || (n.guid = i.guid++),
                                 (f = H.events) || (f = H.events = Object.create(null)),
                                 (a = H.handle) || (a = H.handle = function(V) {
                                         return typeof i < "u" && i.event.triggered !== V.type ? i.event.dispatch.apply(e, arguments) : void 0
                                     }
                                 ),
                                     t = (t || "").match(oe) || [""],
                                     d = t.length; d--; )
                                c = tn.exec(t[d]) || [],
                                    x = R = c[1],
                                    D = (c[2] || "").split(".").sort(),
                                x && (b = i.event.special[x] || {},
                                    x = (o ? b.delegateType : b.bindType) || x,
                                    b = i.event.special[x] || {},
                                    y = i.extend({
                                        type: x,
                                        origType: R,
                                        data: r,
                                        handler: n,
                                        guid: n.guid,
                                        selector: o,
                                        needsContext: o && i.expr.match.needsContext.test(o),
                                        namespace: D.join(".")
                                    }, u),
                                (h = f[x]) || (h = f[x] = [],
                                    h.delegateCount = 0,
                                (!b.setup || b.setup.call(e, r, D, a) === !1) && e.addEventListener && e.addEventListener(x, a)),
                                b.add && (b.add.call(e, y),
                                y.handler.guid || (y.handler.guid = n.guid)),
                                    o ? h.splice(h.delegateCount++, 0, y) : h.push(y),
                                    i.event.global[x] = !0)
                    },
                    remove: function(e, t, n, r, o) {
                        var u, a, c, f, d, y, b, h, x, D, R, H = S.hasData(e) && S.get(e);
                        if (!(!H || !(f = H.events))) {
                            for (t = (t || "").match(oe) || [""],
                                     d = t.length; d--; ) {
                                if (c = tn.exec(t[d]) || [],
                                    x = R = c[1],
                                    D = (c[2] || "").split(".").sort(),
                                    !x) {
                                    for (x in f)
                                        i.event.remove(e, x + t[d], n, r, !0);
                                    continue
                                }
                                for (b = i.event.special[x] || {},
                                         x = (r ? b.delegateType : b.bindType) || x,
                                         h = f[x] || [],
                                         c = c[2] && new RegExp("(^|\\.)" + D.join("\\.(?:.*\\.|)") + "(\\.|$)"),
                                         a = u = h.length; u--; )
                                    y = h[u],
                                    (o || R === y.origType) && (!n || n.guid === y.guid) && (!c || c.test(y.namespace)) && (!r || r === y.selector || r === "**" && y.selector) && (h.splice(u, 1),
                                    y.selector && h.delegateCount--,
                                    b.remove && b.remove.call(e, y));
                                a && !h.length && ((!b.teardown || b.teardown.call(e, D, H.handle) === !1) && i.removeEvent(e, x, H.handle),
                                    delete f[x])
                            }
                            i.isEmptyObject(f) && S.remove(e, "handle events")
                        }
                    },
                    dispatch: function(e) {
                        var t, n, r, o, u, a, c = new Array(arguments.length), f = i.event.fix(e), d = (S.get(this, "events") || Object.create(null))[f.type] || [], y = i.event.special[f.type] || {};
                        for (c[0] = f,
                                 t = 1; t < arguments.length; t++)
                            c[t] = arguments[t];
                        if (f.delegateTarget = this,
                            !(y.preDispatch && y.preDispatch.call(this, f) === !1)) {
                            for (a = i.event.handlers.call(this, f, d),
                                     t = 0; (o = a[t++]) && !f.isPropagationStopped(); )
                                for (f.currentTarget = o.elem,
                                         n = 0; (u = o.handlers[n++]) && !f.isImmediatePropagationStopped(); )
                                    (!f.rnamespace || u.namespace === !1 || f.rnamespace.test(u.namespace)) && (f.handleObj = u,
                                        f.data = u.data,
                                        r = ((i.event.special[u.origType] || {}).handle || u.handler).apply(o.elem, c),
                                    r !== void 0 && (f.result = r) === !1 && (f.preventDefault(),
                                        f.stopPropagation()));
                            return y.postDispatch && y.postDispatch.call(this, f),
                                f.result
                        }
                    },
                    handlers: function(e, t) {
                        var n, r, o, u, a, c = [], f = t.delegateCount, d = e.target;
                        if (f && d.nodeType && !(e.type === "click" && e.button >= 1)) {
                            for (; d !== this; d = d.parentNode || this)
                                if (d.nodeType === 1 && !(e.type === "click" && d.disabled === !0)) {
                                    for (u = [],
                                             a = {},
                                             n = 0; n < f; n++)
                                        r = t[n],
                                            o = r.selector + " ",
                                        a[o] === void 0 && (a[o] = r.needsContext ? i(o, this).index(d) > -1 : i.find(o, this, null, [d]).length),
                                        a[o] && u.push(r);
                                    u.length && c.push({
                                        elem: d,
                                        handlers: u
                                    })
                                }
                        }
                        return d = this,
                        f < t.length && c.push({
                            elem: d,
                            handlers: t.slice(f)
                        }),
                            c
                    },
                    addProp: function(e, t) {
                        Object.defineProperty(i.Event.prototype, e, {
                            enumerable: !0,
                            configurable: !0,
                            get: L(t) ? function() {
                                    if (this.originalEvent)
                                        return t(this.originalEvent)
                                }
                                : function() {
                                    if (this.originalEvent)
                                        return this.originalEvent[e]
                                }
                            ,
                            set: function(n) {
                                Object.defineProperty(this, e, {
                                    enumerable: !0,
                                    configurable: !0,
                                    writable: !0,
                                    value: n
                                })
                            }
                        })
                    },
                    fix: function(e) {
                        return e[i.expando] ? e : new i.Event(e)
                    },
                    special: {
                        load: {
                            noBubble: !0
                        },
                        click: {
                            setup: function(e) {
                                var t = this || e;
                                return Ue.test(t.type) && t.click && $(t, "input") && nt(t, "click", !0),
                                    !1
                            },
                            trigger: function(e) {
                                var t = this || e;
                                return Ue.test(t.type) && t.click && $(t, "input") && nt(t, "click"),
                                    !0
                            },
                            _default: function(e) {
                                var t = e.target;
                                return Ue.test(t.type) && t.click && $(t, "input") && S.get(t, "click") || $(t, "a")
                            }
                        },
                        beforeunload: {
                            postDispatch: function(e) {
                                e.result !== void 0 && e.originalEvent && (e.originalEvent.returnValue = e.result)
                            }
                        }
                    }
                };
                function nt(e, t, n) {
                    if (!n) {
                        S.get(e, t) === void 0 && i.event.add(e, t, Le);
                        return
                    }
                    S.set(e, t, !1),
                        i.event.add(e, t, {
                            namespace: !1,
                            handler: function(r) {
                                var o, u = S.get(this, t);
                                if (r.isTrigger & 1 && this[t]) {
                                    if (u)
                                        (i.event.special[t] || {}).delegateType && r.stopPropagation();
                                    else if (u = Z.call(arguments),
                                        S.set(this, t, u),
                                        this[t](),
                                        o = S.get(this, t),
                                        S.set(this, t, !1),
                                    u !== o)
                                        return r.stopImmediatePropagation(),
                                            r.preventDefault(),
                                            o
                                } else
                                    u && (S.set(this, t, i.event.trigger(u[0], u.slice(1), this)),
                                        r.stopPropagation(),
                                        r.isImmediatePropagationStopped = Le)
                            }
                        })
                }
                i.removeEvent = function(e, t, n) {
                    e.removeEventListener && e.removeEventListener(t, n)
                }
                    ,
                    i.Event = function(e, t) {
                        if (!(this instanceof i.Event))
                            return new i.Event(e,t);
                        e && e.type ? (this.originalEvent = e,
                            this.type = e.type,
                            this.isDefaultPrevented = e.defaultPrevented || e.defaultPrevented === void 0 && e.returnValue === !1 ? Le : He,
                            this.target = e.target && e.target.nodeType === 3 ? e.target.parentNode : e.target,
                            this.currentTarget = e.currentTarget,
                            this.relatedTarget = e.relatedTarget) : this.type = e,
                        t && i.extend(this, t),
                            this.timeStamp = e && e.timeStamp || Date.now(),
                            this[i.expando] = !0
                    }
                    ,
                    i.Event.prototype = {
                        constructor: i.Event,
                        isDefaultPrevented: He,
                        isPropagationStopped: He,
                        isImmediatePropagationStopped: He,
                        isSimulated: !1,
                        preventDefault: function() {
                            var e = this.originalEvent;
                            this.isDefaultPrevented = Le,
                            e && !this.isSimulated && e.preventDefault()
                        },
                        stopPropagation: function() {
                            var e = this.originalEvent;
                            this.isPropagationStopped = Le,
                            e && !this.isSimulated && e.stopPropagation()
                        },
                        stopImmediatePropagation: function() {
                            var e = this.originalEvent;
                            this.isImmediatePropagationStopped = Le,
                            e && !this.isSimulated && e.stopImmediatePropagation(),
                                this.stopPropagation()
                        }
                    },
                    i.each({
                        altKey: !0,
                        bubbles: !0,
                        cancelable: !0,
                        changedTouches: !0,
                        ctrlKey: !0,
                        detail: !0,
                        eventPhase: !0,
                        metaKey: !0,
                        pageX: !0,
                        pageY: !0,
                        shiftKey: !0,
                        view: !0,
                        char: !0,
                        code: !0,
                        charCode: !0,
                        key: !0,
                        keyCode: !0,
                        button: !0,
                        buttons: !0,
                        clientX: !0,
                        clientY: !0,
                        offsetX: !0,
                        offsetY: !0,
                        pointerId: !0,
                        pointerType: !0,
                        screenX: !0,
                        screenY: !0,
                        targetTouches: !0,
                        toElement: !0,
                        touches: !0,
                        which: !0
                    }, i.event.addProp),
                    i.each({
                        focus: "focusin",
                        blur: "focusout"
                    }, function(e, t) {
                        function n(r) {
                            if (j.documentMode) {
                                var o = S.get(this, "handle")
                                    , u = i.event.fix(r);
                                u.type = r.type === "focusin" ? "focus" : "blur",
                                    u.isSimulated = !0,
                                    o(r),
                                u.target === u.currentTarget && o(u)
                            } else
                                i.event.simulate(t, r.target, i.event.fix(r))
                        }
                        i.event.special[e] = {
                            setup: function() {
                                var r;
                                if (nt(this, e, !0),
                                    j.documentMode)
                                    r = S.get(this, t),
                                    r || this.addEventListener(t, n),
                                        S.set(this, t, (r || 0) + 1);
                                else
                                    return !1
                            },
                            trigger: function() {
                                return nt(this, e),
                                    !0
                            },
                            teardown: function() {
                                var r;
                                if (j.documentMode)
                                    r = S.get(this, t) - 1,
                                        r ? S.set(this, t, r) : (this.removeEventListener(t, n),
                                            S.remove(this, t));
                                else
                                    return !1
                            },
                            _default: function(r) {
                                return S.get(r.target, e)
                            },
                            delegateType: t
                        },
                            i.event.special[t] = {
                                setup: function() {
                                    var r = this.ownerDocument || this.document || this
                                        , o = j.documentMode ? this : r
                                        , u = S.get(o, t);
                                    u || (j.documentMode ? this.addEventListener(t, n) : r.addEventListener(e, n, !0)),
                                        S.set(o, t, (u || 0) + 1)
                                },
                                teardown: function() {
                                    var r = this.ownerDocument || this.document || this
                                        , o = j.documentMode ? this : r
                                        , u = S.get(o, t) - 1;
                                    u ? S.set(o, t, u) : (j.documentMode ? this.removeEventListener(t, n) : r.removeEventListener(e, n, !0),
                                        S.remove(o, t))
                                }
                            }
                    }),
                    i.each({
                        mouseenter: "mouseover",
                        mouseleave: "mouseout",
                        pointerenter: "pointerover",
                        pointerleave: "pointerout"
                    }, function(e, t) {
                        i.event.special[e] = {
                            delegateType: t,
                            bindType: t,
                            handle: function(n) {
                                var r, o = this, u = n.relatedTarget, a = n.handleObj;
                                return (!u || u !== o && !i.contains(o, u)) && (n.type = a.origType,
                                    r = a.handler.apply(this, arguments),
                                    n.type = t),
                                    r
                            }
                        }
                    }),
                    i.fn.extend({
                        on: function(e, t, n, r) {
                            return yt(this, e, t, n, r)
                        },
                        one: function(e, t, n, r) {
                            return yt(this, e, t, n, r, 1)
                        },
                        off: function(e, t, n) {
                            var r, o;
                            if (e && e.preventDefault && e.handleObj)
                                return r = e.handleObj,
                                    i(e.delegateTarget).off(r.namespace ? r.origType + "." + r.namespace : r.origType, r.selector, r.handler),
                                    this;
                            if (typeof e == "object") {
                                for (o in e)
                                    this.off(o, t, e[o]);
                                return this
                            }
                            return (t === !1 || typeof t == "function") && (n = t,
                                t = void 0),
                            n === !1 && (n = He),
                                this.each(function() {
                                    i.event.remove(this, e, n, t)
                                })
                        }
                    });
                var Kn = /<script|<style|<link/i
                    , Zn = /checked\s*(?:[^=]|=\s*.checked.)/i
                    , er = /^\s*<!\[CDATA\[|\]\]>\s*$/g;
                function nn(e, t) {
                    return $(e, "table") && $(t.nodeType !== 11 ? t : t.firstChild, "tr") && i(e).children("tbody")[0] || e
                }
                function tr(e) {
                    return e.type = (e.getAttribute("type") !== null) + "/" + e.type,
                        e
                }
                function nr(e) {
                    return (e.type || "").slice(0, 5) === "true/" ? e.type = e.type.slice(5) : e.removeAttribute("type"),
                        e
                }
                function rn(e, t) {
                    var n, r, o, u, a, c, f;
                    if (t.nodeType === 1) {
                        if (S.hasData(e) && (u = S.get(e),
                            f = u.events,
                            f)) {
                            S.remove(t, "handle events");
                            for (o in f)
                                for (n = 0,
                                         r = f[o].length; n < r; n++)
                                    i.event.add(t, o, f[o][n])
                        }
                        Q.hasData(e) && (a = Q.access(e),
                            c = i.extend({}, a),
                            Q.set(t, c))
                    }
                }
                function rr(e, t) {
                    var n = t.nodeName.toLowerCase();
                    n === "input" && Ue.test(e.type) ? t.checked = e.checked : (n === "input" || n === "textarea") && (t.defaultValue = e.defaultValue)
                }
                function Oe(e, t, n, r) {
                    t = Rt(t);
                    var o, u, a, c, f, d, y = 0, b = e.length, h = b - 1, x = t[0], D = L(x);
                    if (D || b > 1 && typeof x == "string" && !q.checkClone && Zn.test(x))
                        return e.each(function(R) {
                            var H = e.eq(R);
                            D && (t[0] = x.call(this, R, H.html())),
                                Oe(H, t, n, r)
                        });
                    if (b && (o = en(t, e[0].ownerDocument, !1, e, r),
                        u = o.firstChild,
                    o.childNodes.length === 1 && (o = u),
                    u || r)) {
                        for (a = i.map(Y(o, "script"), tr),
                                 c = a.length; y < b; y++)
                            f = o,
                            y !== h && (f = i.clone(f, !0, !0),
                            c && i.merge(a, Y(f, "script"))),
                                n.call(e[y], f, y);
                        if (c)
                            for (d = a[a.length - 1].ownerDocument,
                                     i.map(a, nr),
                                     y = 0; y < c; y++)
                                f = a[y],
                                Zt.test(f.type || "") && !S.access(f, "globalEval") && i.contains(d, f) && (f.src && (f.type || "").toLowerCase() !== "module" ? i._evalUrl && !f.noModule && i._evalUrl(f.src, {
                                    nonce: f.nonce || f.getAttribute("nonce")
                                }, d) : Wt(f.textContent.replace(er, ""), f, d))
                    }
                    return e
                }
                function on(e, t, n) {
                    for (var r, o = t ? i.filter(t, e) : e, u = 0; (r = o[u]) != null; u++)
                        !n && r.nodeType === 1 && i.cleanData(Y(r)),
                        r.parentNode && (n && je(r) && gt(Y(r, "script")),
                            r.parentNode.removeChild(r));
                    return e
                }
                i.extend({
                    htmlPrefilter: function(e) {
                        return e
                    },
                    clone: function(e, t, n) {
                        var r, o, u, a, c = e.cloneNode(!0), f = je(e);
                        if (!q.noCloneChecked && (e.nodeType === 1 || e.nodeType === 11) && !i.isXMLDoc(e))
                            for (a = Y(c),
                                     u = Y(e),
                                     r = 0,
                                     o = u.length; r < o; r++)
                                rr(u[r], a[r]);
                        if (t)
                            if (n)
                                for (u = u || Y(e),
                                         a = a || Y(c),
                                         r = 0,
                                         o = u.length; r < o; r++)
                                    rn(u[r], a[r]);
                            else
                                rn(e, c);
                        return a = Y(c, "script"),
                        a.length > 0 && gt(a, !f && Y(e, "script")),
                            c
                    },
                    cleanData: function(e) {
                        for (var t, n, r, o = i.event.special, u = 0; (n = e[u]) !== void 0; u++)
                            if (Fe(n)) {
                                if (t = n[S.expando]) {
                                    if (t.events)
                                        for (r in t.events)
                                            o[r] ? i.event.remove(n, r) : i.removeEvent(n, r, t.handle);
                                    n[S.expando] = void 0
                                }
                                n[Q.expando] && (n[Q.expando] = void 0)
                            }
                    }
                }),
                    i.fn.extend({
                        detach: function(e) {
                            return on(this, e, !0)
                        },
                        remove: function(e) {
                            return on(this, e)
                        },
                        text: function(e) {
                            return he(this, function(t) {
                                return t === void 0 ? i.text(this) : this.empty().each(function() {
                                    (this.nodeType === 1 || this.nodeType === 11 || this.nodeType === 9) && (this.textContent = t)
                                })
                            }, null, e, arguments.length)
                        },
                        append: function() {
                            return Oe(this, arguments, function(e) {
                                if (this.nodeType === 1 || this.nodeType === 11 || this.nodeType === 9) {
                                    var t = nn(this, e);
                                    t.appendChild(e)
                                }
                            })
                        },
                        prepend: function() {
                            return Oe(this, arguments, function(e) {
                                if (this.nodeType === 1 || this.nodeType === 11 || this.nodeType === 9) {
                                    var t = nn(this, e);
                                    t.insertBefore(e, t.firstChild)
                                }
                            })
                        },
                        before: function() {
                            return Oe(this, arguments, function(e) {
                                this.parentNode && this.parentNode.insertBefore(e, this)
                            })
                        },
                        after: function() {
                            return Oe(this, arguments, function(e) {
                                this.parentNode && this.parentNode.insertBefore(e, this.nextSibling)
                            })
                        },
                        empty: function() {
                            for (var e, t = 0; (e = this[t]) != null; t++)
                                e.nodeType === 1 && (i.cleanData(Y(e, !1)),
                                    e.textContent = "");
                            return this
                        },
                        clone: function(e, t) {
                            return e = e ?? !1,
                                t = t ?? e,
                                this.map(function() {
                                    return i.clone(this, e, t)
                                })
                        },
                        html: function(e) {
                            return he(this, function(t) {
                                var n = this[0] || {}
                                    , r = 0
                                    , o = this.length;
                                if (t === void 0 && n.nodeType === 1)
                                    return n.innerHTML;
                                if (typeof t == "string" && !Kn.test(t) && !ee[(Kt.exec(t) || ["", ""])[1].toLowerCase()]) {
                                    t = i.htmlPrefilter(t);
                                    try {
                                        for (; r < o; r++)
                                            n = this[r] || {},
                                            n.nodeType === 1 && (i.cleanData(Y(n, !1)),
                                                n.innerHTML = t);
                                        n = 0
                                    } catch {}
                                }
                                n && this.empty().append(t)
                            }, null, e, arguments.length)
                        },
                        replaceWith: function() {
                            var e = [];
                            return Oe(this, arguments, function(t) {
                                var n = this.parentNode;
                                i.inArray(this, e) < 0 && (i.cleanData(Y(this)),
                                n && n.replaceChild(t, this))
                            }, e)
                        }
                    }),
                    i.each({
                        appendTo: "append",
                        prependTo: "prepend",
                        insertBefore: "before",
                        insertAfter: "after",
                        replaceAll: "replaceWith"
                    }, function(e, t) {
                        i.fn[e] = function(n) {
                            for (var r, o = [], u = i(n), a = u.length - 1, c = 0; c <= a; c++)
                                r = c === a ? this : this.clone(!0),
                                    i(u[c])[t](r),
                                    Je.apply(o, r.get());
                            return this.pushStack(o)
                        }
                    });
                var vt = new RegExp("^(" + Qt + ")(?!px)[a-z%]+$","i")
                    , bt = /^--/
                    , rt = function(e) {
                    var t = e.ownerDocument.defaultView;
                    return (!t || !t.opener) && (t = k),
                        t.getComputedStyle(e)
                }
                    , un = function(e, t, n) {
                    var r, o, u = {};
                    for (o in t)
                        u[o] = e.style[o],
                            e.style[o] = t[o];
                    r = n.call(e);
                    for (o in t)
                        e.style[o] = u[o];
                    return r
                }
                    , ir = new RegExp(ge.join("|"),"i");
                (function() {
                        function e() {
                            if (d) {
                                f.style.cssText = "position:absolute;left:-11111px;width:60px;margin-top:1px;padding:0;border:0",
                                    d.style.cssText = "position:relative;display:block;box-sizing:border-box;overflow:scroll;margin:auto;border:1px;padding:1px;width:60%;top:1%",
                                    Te.appendChild(f).appendChild(d);
                                var y = k.getComputedStyle(d);
                                n = y.top !== "1%",
                                    c = t(y.marginLeft) === 12,
                                    d.style.right = "60%",
                                    u = t(y.right) === 36,
                                    r = t(y.width) === 36,
                                    d.style.position = "absolute",
                                    o = t(d.offsetWidth / 3) === 12,
                                    Te.removeChild(f),
                                    d = null
                            }
                        }
                        function t(y) {
                            return Math.round(parseFloat(y))
                        }
                        var n, r, o, u, a, c, f = j.createElement("div"), d = j.createElement("div");
                        d.style && (d.style.backgroundClip = "content-box",
                            d.cloneNode(!0).style.backgroundClip = "",
                            q.clearCloneStyle = d.style.backgroundClip === "content-box",
                            i.extend(q, {
                                boxSizingReliable: function() {
                                    return e(),
                                        r
                                },
                                pixelBoxStyles: function() {
                                    return e(),
                                        u
                                },
                                pixelPosition: function() {
                                    return e(),
                                        n
                                },
                                reliableMarginLeft: function() {
                                    return e(),
                                        c
                                },
                                scrollboxSize: function() {
                                    return e(),
                                        o
                                },
                                reliableTrDimensions: function() {
                                    var y, b, h, x;
                                    return a == null && (y = j.createElement("table"),
                                        b = j.createElement("tr"),
                                        h = j.createElement("div"),
                                        y.style.cssText = "position:absolute;left:-11111px;border-collapse:separate",
                                        b.style.cssText = "border:1px solid",
                                        b.style.height = "1px",
                                        h.style.height = "9px",
                                        h.style.display = "block",
                                        Te.appendChild(y).appendChild(b).appendChild(h),
                                        x = k.getComputedStyle(b),
                                        a = parseInt(x.height, 10) + parseInt(x.borderTopWidth, 10) + parseInt(x.borderBottomWidth, 10) === b.offsetHeight,
                                        Te.removeChild(y)),
                                        a
                                }
                            }))
                    }
                )();
                function ze(e, t, n) {
                    var r, o, u, a, c = bt.test(t), f = e.style;
                    return n = n || rt(e),
                    n && (a = n.getPropertyValue(t) || n[t],
                    c && a && (a = a.replace(We, "$1") || void 0),
                    a === "" && !je(e) && (a = i.style(e, t)),
                    !q.pixelBoxStyles() && vt.test(a) && ir.test(t) && (r = f.width,
                        o = f.minWidth,
                        u = f.maxWidth,
                        f.minWidth = f.maxWidth = f.width = a,
                        a = n.width,
                        f.width = r,
                        f.minWidth = o,
                        f.maxWidth = u)),
                        a !== void 0 ? a + "" : a
                }
                function an(e, t) {
                    return {
                        get: function() {
                            if (e()) {
                                delete this.get;
                                return
                            }
                            return (this.get = t).apply(this, arguments)
                        }
                    }
                }
                var sn = ["Webkit", "Moz", "ms"]
                    , fn = j.createElement("div").style
                    , cn = {};
                function or(e) {
                    for (var t = e[0].toUpperCase() + e.slice(1), n = sn.length; n--; )
                        if (e = sn[n] + t,
                        e in fn)
                            return e
                }
                function xt(e) {
                    var t = i.cssProps[e] || cn[e];
                    return t || (e in fn ? e : cn[e] = or(e) || e)
                }
                var ur = /^(none|table(?!-c[ea]).+)/
                    , ar = {
                    position: "absolute",
                    visibility: "hidden",
                    display: "block"
                }
                    , ln = {
                    letterSpacing: "0",
                    fontWeight: "400"
                };
                function dn(e, t, n) {
                    var r = Be.exec(t);
                    return r ? Math.max(0, r[2] - (n || 0)) + (r[3] || "px") : t
                }
                function mt(e, t, n, r, o, u) {
                    var a = t === "width" ? 1 : 0
                        , c = 0
                        , f = 0
                        , d = 0;
                    if (n === (r ? "border" : "content"))
                        return 0;
                    for (; a < 4; a += 2)
                        n === "margin" && (d += i.css(e, n + ge[a], !0, o)),
                            r ? (n === "content" && (f -= i.css(e, "padding" + ge[a], !0, o)),
                            n !== "margin" && (f -= i.css(e, "border" + ge[a] + "Width", !0, o))) : (f += i.css(e, "padding" + ge[a], !0, o),
                                n !== "padding" ? f += i.css(e, "border" + ge[a] + "Width", !0, o) : c += i.css(e, "border" + ge[a] + "Width", !0, o));
                    return !r && u >= 0 && (f += Math.max(0, Math.ceil(e["offset" + t[0].toUpperCase() + t.slice(1)] - u - f - c - .5)) || 0),
                    f + d
                }
                function pn(e, t, n) {
                    var r = rt(e)
                        , o = !q.boxSizingReliable() || n
                        , u = o && i.css(e, "boxSizing", !1, r) === "border-box"
                        , a = u
                        , c = ze(e, t, r)
                        , f = "offset" + t[0].toUpperCase() + t.slice(1);
                    if (vt.test(c)) {
                        if (!n)
                            return c;
                        c = "auto"
                    }
                    return (!q.boxSizingReliable() && u || !q.reliableTrDimensions() && $(e, "tr") || c === "auto" || !parseFloat(c) && i.css(e, "display", !1, r) === "inline") && e.getClientRects().length && (u = i.css(e, "boxSizing", !1, r) === "border-box",
                        a = f in e,
                    a && (c = e[f])),
                        c = parseFloat(c) || 0,
                    c + mt(e, t, n || (u ? "border" : "content"), a, r, c) + "px"
                }
                i.extend({
                    cssHooks: {
                        opacity: {
                            get: function(e, t) {
                                if (t) {
                                    var n = ze(e, "opacity");
                                    return n === "" ? "1" : n
                                }
                            }
                        }
                    },
                    cssNumber: {
                        animationIterationCount: !0,
                        aspectRatio: !0,
                        borderImageSlice: !0,
                        columnCount: !0,
                        flexGrow: !0,
                        flexShrink: !0,
                        fontWeight: !0,
                        gridArea: !0,
                        gridColumn: !0,
                        gridColumnEnd: !0,
                        gridColumnStart: !0,
                        gridRow: !0,
                        gridRowEnd: !0,
                        gridRowStart: !0,
                        lineHeight: !0,
                        opacity: !0,
                        order: !0,
                        orphans: !0,
                        scale: !0,
                        widows: !0,
                        zIndex: !0,
                        zoom: !0,
                        fillOpacity: !0,
                        floodOpacity: !0,
                        stopOpacity: !0,
                        strokeMiterlimit: !0,
                        strokeOpacity: !0
                    },
                    cssProps: {},
                    style: function(e, t, n, r) {
                        if (!(!e || e.nodeType === 3 || e.nodeType === 8 || !e.style)) {
                            var o, u, a, c = ue(t), f = bt.test(t), d = e.style;
                            if (f || (t = xt(c)),
                                a = i.cssHooks[t] || i.cssHooks[c],
                            n !== void 0) {
                                if (u = typeof n,
                                u === "string" && (o = Be.exec(n)) && o[1] && (n = Yt(e, t, o),
                                    u = "number"),
                                n == null || n !== n)
                                    return;
                                u === "number" && !f && (n += o && o[3] || (i.cssNumber[c] ? "" : "px")),
                                !q.clearCloneStyle && n === "" && t.indexOf("background") === 0 && (d[t] = "inherit"),
                                (!a || !("set"in a) || (n = a.set(e, n, r)) !== void 0) && (f ? d.setProperty(t, n) : d[t] = n)
                            } else
                                return a && "get"in a && (o = a.get(e, !1, r)) !== void 0 ? o : d[t]
                        }
                    },
                    css: function(e, t, n, r) {
                        var o, u, a, c = ue(t), f = bt.test(t);
                        return f || (t = xt(c)),
                            a = i.cssHooks[t] || i.cssHooks[c],
                        a && "get"in a && (o = a.get(e, !0, n)),
                        o === void 0 && (o = ze(e, t, r)),
                        o === "normal" && t in ln && (o = ln[t]),
                            n === "" || n ? (u = parseFloat(o),
                                n === !0 || isFinite(u) ? u || 0 : o) : o
                    }
                }),
                    i.each(["height", "width"], function(e, t) {
                        i.cssHooks[t] = {
                            get: function(n, r, o) {
                                if (r)
                                    return ur.test(i.css(n, "display")) && (!n.getClientRects().length || !n.getBoundingClientRect().width) ? un(n, ar, function() {
                                        return pn(n, t, o)
                                    }) : pn(n, t, o)
                            },
                            set: function(n, r, o) {
                                var u, a = rt(n), c = !q.scrollboxSize() && a.position === "absolute", f = c || o, d = f && i.css(n, "boxSizing", !1, a) === "border-box", y = o ? mt(n, t, o, d, a) : 0;
                                return d && c && (y -= Math.ceil(n["offset" + t[0].toUpperCase() + t.slice(1)] - parseFloat(a[t]) - mt(n, t, "border", !1, a) - .5)),
                                y && (u = Be.exec(r)) && (u[3] || "px") !== "px" && (n.style[t] = r,
                                    r = i.css(n, t)),
                                    dn(n, r, y)
                            }
                        }
                    }),
                    i.cssHooks.marginLeft = an(q.reliableMarginLeft, function(e, t) {
                        if (t)
                            return (parseFloat(ze(e, "marginLeft")) || e.getBoundingClientRect().left - un(e, {
                                marginLeft: 0
                            }, function() {
                                return e.getBoundingClientRect().left
                            })) + "px"
                    }),
                    i.each({
                        margin: "",
                        padding: "",
                        border: "Width"
                    }, function(e, t) {
                        i.cssHooks[e + t] = {
                            expand: function(n) {
                                for (var r = 0, o = {}, u = typeof n == "string" ? n.split(" ") : [n]; r < 4; r++)
                                    o[e + ge[r] + t] = u[r] || u[r - 2] || u[0];
                                return o
                            }
                        },
                        e !== "margin" && (i.cssHooks[e + t].set = dn)
                    }),
                    i.fn.extend({
                        css: function(e, t) {
                            return he(this, function(n, r, o) {
                                var u, a, c = {}, f = 0;
                                if (Array.isArray(r)) {
                                    for (u = rt(n),
                                             a = r.length; f < a; f++)
                                        c[r[f]] = i.css(n, r[f], !1, u);
                                    return c
                                }
                                return o !== void 0 ? i.style(n, r, o) : i.css(n, r)
                            }, e, t, arguments.length > 1)
                        }
                    });
                function J(e, t, n, r, o) {
                    return new J.prototype.init(e,t,n,r,o)
                }
                i.Tween = J,
                    J.prototype = {
                        constructor: J,
                        init: function(e, t, n, r, o, u) {
                            this.elem = e,
                                this.prop = n,
                                this.easing = o || i.easing._default,
                                this.options = t,
                                this.start = this.now = this.cur(),
                                this.end = r,
                                this.unit = u || (i.cssNumber[n] ? "" : "px")
                        },
                        cur: function() {
                            var e = J.propHooks[this.prop];
                            return e && e.get ? e.get(this) : J.propHooks._default.get(this)
                        },
                        run: function(e) {
                            var t, n = J.propHooks[this.prop];
                            return this.options.duration ? this.pos = t = i.easing[this.easing](e, this.options.duration * e, 0, 1, this.options.duration) : this.pos = t = e,
                                this.now = (this.end - this.start) * t + this.start,
                            this.options.step && this.options.step.call(this.elem, this.now, this),
                                n && n.set ? n.set(this) : J.propHooks._default.set(this),
                                this
                        }
                    },
                    J.prototype.init.prototype = J.prototype,
                    J.propHooks = {
                        _default: {
                            get: function(e) {
                                var t;
                                return e.elem.nodeType !== 1 || e.elem[e.prop] != null && e.elem.style[e.prop] == null ? e.elem[e.prop] : (t = i.css(e.elem, e.prop, ""),
                                    !t || t === "auto" ? 0 : t)
                            },
                            set: function(e) {
                                i.fx.step[e.prop] ? i.fx.step[e.prop](e) : e.elem.nodeType === 1 && (i.cssHooks[e.prop] || e.elem.style[xt(e.prop)] != null) ? i.style(e.elem, e.prop, e.now + e.unit) : e.elem[e.prop] = e.now
                            }
                        }
                    },
                    J.propHooks.scrollTop = J.propHooks.scrollLeft = {
                        set: function(e) {
                            e.elem.nodeType && e.elem.parentNode && (e.elem[e.prop] = e.now)
                        }
                    },
                    i.easing = {
                        linear: function(e) {
                            return e
                        },
                        swing: function(e) {
                            return .5 - Math.cos(e * Math.PI) / 2
                        },
                        _default: "swing"
                    },
                    i.fx = J.prototype.init,
                    i.fx.step = {};
                var Pe, it, sr = /^(?:toggle|show|hide)$/, fr = /queueHooks$/;
                function Tt() {
                    it && (j.hidden === !1 && k.requestAnimationFrame ? k.requestAnimationFrame(Tt) : k.setTimeout(Tt, i.fx.interval),
                        i.fx.tick())
                }
                function hn() {
                    return k.setTimeout(function() {
                        Pe = void 0
                    }),
                        Pe = Date.now()
                }
                function ot(e, t) {
                    var n, r = 0, o = {
                        height: e
                    };
                    for (t = t ? 1 : 0; r < 4; r += 2 - t)
                        n = ge[r],
                            o["margin" + n] = o["padding" + n] = e;
                    return t && (o.opacity = o.width = e),
                        o
                }
                function gn(e, t, n) {
                    for (var r, o = (re.tweeners[t] || []).concat(re.tweeners["*"]), u = 0, a = o.length; u < a; u++)
                        if (r = o[u].call(n, t, e))
                            return r
                }
                function cr(e, t, n) {
                    var r, o, u, a, c, f, d, y, b = "width"in t || "height"in t, h = this, x = {}, D = e.style, R = e.nodeType && tt(e), H = S.get(e, "fxshow");
                    n.queue || (a = i._queueHooks(e, "fx"),
                    a.unqueued == null && (a.unqueued = 0,
                            c = a.empty.fire,
                            a.empty.fire = function() {
                                a.unqueued || c()
                            }
                    ),
                        a.unqueued++,
                        h.always(function() {
                            h.always(function() {
                                a.unqueued--,
                                i.queue(e, "fx").length || a.empty.fire()
                            })
                        }));
                    for (r in t)
                        if (o = t[r],
                            sr.test(o)) {
                            if (delete t[r],
                                u = u || o === "toggle",
                            o === (R ? "hide" : "show"))
                                if (o === "show" && H && H[r] !== void 0)
                                    R = !0;
                                else
                                    continue;
                            x[r] = H && H[r] || i.style(e, r)
                        }
                    if (f = !i.isEmptyObject(t),
                        !(!f && i.isEmptyObject(x))) {
                        b && e.nodeType === 1 && (n.overflow = [D.overflow, D.overflowX, D.overflowY],
                            d = H && H.display,
                        d == null && (d = S.get(e, "display")),
                            y = i.css(e, "display"),
                        y === "none" && (d ? y = d : (qe([e], !0),
                            d = e.style.display || d,
                            y = i.css(e, "display"),
                            qe([e]))),
                        (y === "inline" || y === "inline-block" && d != null) && i.css(e, "float") === "none" && (f || (h.done(function() {
                            D.display = d
                        }),
                        d == null && (y = D.display,
                            d = y === "none" ? "" : y)),
                            D.display = "inline-block")),
                        n.overflow && (D.overflow = "hidden",
                            h.always(function() {
                                D.overflow = n.overflow[0],
                                    D.overflowX = n.overflow[1],
                                    D.overflowY = n.overflow[2]
                            })),
                            f = !1;
                        for (r in x)
                            f || (H ? "hidden"in H && (R = H.hidden) : H = S.access(e, "fxshow", {
                                display: d
                            }),
                            u && (H.hidden = !R),
                            R && qe([e], !0),
                                h.done(function() {
                                    R || qe([e]),
                                        S.remove(e, "fxshow");
                                    for (r in x)
                                        i.style(e, r, x[r])
                                })),
                                f = gn(R ? H[r] : 0, r, h),
                            r in H || (H[r] = f.start,
                            R && (f.end = f.start,
                                f.start = 0))
                    }
                }
                function lr(e, t) {
                    var n, r, o, u, a;
                    for (n in e)
                        if (r = ue(n),
                            o = t[r],
                            u = e[n],
                        Array.isArray(u) && (o = u[1],
                            u = e[n] = u[0]),
                        n !== r && (e[r] = u,
                            delete e[n]),
                            a = i.cssHooks[r],
                        a && "expand"in a) {
                            u = a.expand(u),
                                delete e[r];
                            for (n in u)
                                n in e || (e[n] = u[n],
                                    t[n] = o)
                        } else
                            t[r] = o
                }
                function re(e, t, n) {
                    var r, o, u = 0, a = re.prefilters.length, c = i.Deferred().always(function() {
                        delete f.elem
                    }), f = function() {
                        if (o)
                            return !1;
                        for (var b = Pe || hn(), h = Math.max(0, d.startTime + d.duration - b), x = h / d.duration || 0, D = 1 - x, R = 0, H = d.tweens.length; R < H; R++)
                            d.tweens[R].run(D);
                        return c.notifyWith(e, [d, D, h]),
                            D < 1 && H ? h : (H || c.notifyWith(e, [d, 1, 0]),
                                c.resolveWith(e, [d]),
                                !1)
                    }, d = c.promise({
                        elem: e,
                        props: i.extend({}, t),
                        opts: i.extend(!0, {
                            specialEasing: {},
                            easing: i.easing._default
                        }, n),
                        originalProperties: t,
                        originalOptions: n,
                        startTime: Pe || hn(),
                        duration: n.duration,
                        tweens: [],
                        createTween: function(b, h) {
                            var x = i.Tween(e, d.opts, b, h, d.opts.specialEasing[b] || d.opts.easing);
                            return d.tweens.push(x),
                                x
                        },
                        stop: function(b) {
                            var h = 0
                                , x = b ? d.tweens.length : 0;
                            if (o)
                                return this;
                            for (o = !0; h < x; h++)
                                d.tweens[h].run(1);
                            return b ? (c.notifyWith(e, [d, 1, 0]),
                                c.resolveWith(e, [d, b])) : c.rejectWith(e, [d, b]),
                                this
                        }
                    }), y = d.props;
                    for (lr(y, d.opts.specialEasing); u < a; u++)
                        if (r = re.prefilters[u].call(d, e, y, d.opts),
                            r)
                            return L(r.stop) && (i._queueHooks(d.elem, d.opts.queue).stop = r.stop.bind(r)),
                                r;
                    return i.map(y, gn, d),
                    L(d.opts.start) && d.opts.start.call(e, d),
                        d.progress(d.opts.progress).done(d.opts.done, d.opts.complete).fail(d.opts.fail).always(d.opts.always),
                        i.fx.timer(i.extend(f, {
                            elem: e,
                            anim: d,
                            queue: d.opts.queue
                        })),
                        d
                }
                i.Animation = i.extend(re, {
                    tweeners: {
                        "*": [function(e, t) {
                            var n = this.createTween(e, t);
                            return Yt(n.elem, e, Be.exec(t), n),
                                n
                        }
                        ]
                    },
                    tweener: function(e, t) {
                        L(e) ? (t = e,
                            e = ["*"]) : e = e.match(oe);
                        for (var n, r = 0, o = e.length; r < o; r++)
                            n = e[r],
                                re.tweeners[n] = re.tweeners[n] || [],
                                re.tweeners[n].unshift(t)
                    },
                    prefilters: [cr],
                    prefilter: function(e, t) {
                        t ? re.prefilters.unshift(e) : re.prefilters.push(e)
                    }
                }),
                    i.speed = function(e, t, n) {
                        var r = e && typeof e == "object" ? i.extend({}, e) : {
                            complete: n || !n && t || L(e) && e,
                            duration: e,
                            easing: n && t || t && !L(t) && t
                        };
                        return i.fx.off ? r.duration = 0 : typeof r.duration != "number" && (r.duration in i.fx.speeds ? r.duration = i.fx.speeds[r.duration] : r.duration = i.fx.speeds._default),
                        (r.queue == null || r.queue === !0) && (r.queue = "fx"),
                            r.old = r.complete,
                            r.complete = function() {
                                L(r.old) && r.old.call(this),
                                r.queue && i.dequeue(this, r.queue)
                            }
                            ,
                            r
                    }
                    ,
                    i.fn.extend({
                        fadeTo: function(e, t, n, r) {
                            return this.filter(tt).css("opacity", 0).show().end().animate({
                                opacity: t
                            }, e, n, r)
                        },
                        animate: function(e, t, n, r) {
                            var o = i.isEmptyObject(e)
                                , u = i.speed(t, n, r)
                                , a = function() {
                                var c = re(this, i.extend({}, e), u);
                                (o || S.get(this, "finish")) && c.stop(!0)
                            };
                            return a.finish = a,
                                o || u.queue === !1 ? this.each(a) : this.queue(u.queue, a)
                        },
                        stop: function(e, t, n) {
                            var r = function(o) {
                                var u = o.stop;
                                delete o.stop,
                                    u(n)
                            };
                            return typeof e != "string" && (n = t,
                                t = e,
                                e = void 0),
                            t && this.queue(e || "fx", []),
                                this.each(function() {
                                    var o = !0
                                        , u = e != null && e + "queueHooks"
                                        , a = i.timers
                                        , c = S.get(this);
                                    if (u)
                                        c[u] && c[u].stop && r(c[u]);
                                    else
                                        for (u in c)
                                            c[u] && c[u].stop && fr.test(u) && r(c[u]);
                                    for (u = a.length; u--; )
                                        a[u].elem === this && (e == null || a[u].queue === e) && (a[u].anim.stop(n),
                                            o = !1,
                                            a.splice(u, 1));
                                    (o || !n) && i.dequeue(this, e)
                                })
                        },
                        finish: function(e) {
                            return e !== !1 && (e = e || "fx"),
                                this.each(function() {
                                    var t, n = S.get(this), r = n[e + "queue"], o = n[e + "queueHooks"], u = i.timers, a = r ? r.length : 0;
                                    for (n.finish = !0,
                                             i.queue(this, e, []),
                                         o && o.stop && o.stop.call(this, !0),
                                             t = u.length; t--; )
                                        u[t].elem === this && u[t].queue === e && (u[t].anim.stop(!0),
                                            u.splice(t, 1));
                                    for (t = 0; t < a; t++)
                                        r[t] && r[t].finish && r[t].finish.call(this);
                                    delete n.finish
                                })
                        }
                    }),
                    i.each(["toggle", "show", "hide"], function(e, t) {
                        var n = i.fn[t];
                        i.fn[t] = function(r, o, u) {
                            return r == null || typeof r == "boolean" ? n.apply(this, arguments) : this.animate(ot(t, !0), r, o, u)
                        }
                    }),
                    i.each({
                        slideDown: ot("show"),
                        slideUp: ot("hide"),
                        slideToggle: ot("toggle"),
                        fadeIn: {
                            opacity: "show"
                        },
                        fadeOut: {
                            opacity: "hide"
                        },
                        fadeToggle: {
                            opacity: "toggle"
                        }
                    }, function(e, t) {
                        i.fn[e] = function(n, r, o) {
                            return this.animate(t, n, r, o)
                        }
                    }),
                    i.timers = [],
                    i.fx.tick = function() {
                        var e, t = 0, n = i.timers;
                        for (Pe = Date.now(); t < n.length; t++)
                            e = n[t],
                            !e() && n[t] === e && n.splice(t--, 1);
                        n.length || i.fx.stop(),
                            Pe = void 0
                    }
                    ,
                    i.fx.timer = function(e) {
                        i.timers.push(e),
                            i.fx.start()
                    }
                    ,
                    i.fx.interval = 13,
                    i.fx.start = function() {
                        it || (it = !0,
                            Tt())
                    }
                    ,
                    i.fx.stop = function() {
                        it = null
                    }
                    ,
                    i.fx.speeds = {
                        slow: 600,
                        fast: 200,
                        _default: 400
                    },
                    i.fn.delay = function(e, t) {
                        return e = i.fx && i.fx.speeds[e] || e,
                            t = t || "fx",
                            this.queue(t, function(n, r) {
                                var o = k.setTimeout(n, e);
                                r.stop = function() {
                                    k.clearTimeout(o)
                                }
                            })
                    }
                    ,
                    function() {
                        var e = j.createElement("input")
                            , t = j.createElement("select")
                            , n = t.appendChild(j.createElement("option"));
                        e.type = "checkbox",
                            q.checkOn = e.value !== "",
                            q.optSelected = n.selected,
                            e = j.createElement("input"),
                            e.value = "t",
                            e.type = "radio",
                            q.radioValue = e.value === "t"
                    }();
                var yn, Ve = i.expr.attrHandle;
                i.fn.extend({
                    attr: function(e, t) {
                        return he(this, i.attr, e, t, arguments.length > 1)
                    },
                    removeAttr: function(e) {
                        return this.each(function() {
                            i.removeAttr(this, e)
                        })
                    }
                }),
                    i.extend({
                        attr: function(e, t, n) {
                            var r, o, u = e.nodeType;
                            if (!(u === 3 || u === 8 || u === 2)) {
                                if (typeof e.getAttribute > "u")
                                    return i.prop(e, t, n);
                                if ((u !== 1 || !i.isXMLDoc(e)) && (o = i.attrHooks[t.toLowerCase()] || (i.expr.match.bool.test(t) ? yn : void 0)),
                                n !== void 0) {
                                    if (n === null) {
                                        i.removeAttr(e, t);
                                        return
                                    }
                                    return o && "set"in o && (r = o.set(e, n, t)) !== void 0 ? r : (e.setAttribute(t, n + ""),
                                        n)
                                }
                                return o && "get"in o && (r = o.get(e, t)) !== null ? r : (r = i.find.attr(e, t),
                                r ?? void 0)
                            }
                        },
                        attrHooks: {
                            type: {
                                set: function(e, t) {
                                    if (!q.radioValue && t === "radio" && $(e, "input")) {
                                        var n = e.value;
                                        return e.setAttribute("type", t),
                                        n && (e.value = n),
                                            t
                                    }
                                }
                            }
                        },
                        removeAttr: function(e, t) {
                            var n, r = 0, o = t && t.match(oe);
                            if (o && e.nodeType === 1)
                                for (; n = o[r++]; )
                                    e.removeAttribute(n)
                        }
                    }),
                    yn = {
                        set: function(e, t, n) {
                            return t === !1 ? i.removeAttr(e, n) : e.setAttribute(n, n),
                                n
                        }
                    },
                    i.each(i.expr.match.bool.source.match(/\w+/g), function(e, t) {
                        var n = Ve[t] || i.find.attr;
                        Ve[t] = function(r, o, u) {
                            var a, c, f = o.toLowerCase();
                            return u || (c = Ve[f],
                                Ve[f] = a,
                                a = n(r, o, u) != null ? f : null,
                                Ve[f] = c),
                                a
                        }
                    });
                var dr = /^(?:input|select|textarea|button)$/i
                    , pr = /^(?:a|area)$/i;
                i.fn.extend({
                    prop: function(e, t) {
                        return he(this, i.prop, e, t, arguments.length > 1)
                    },
                    removeProp: function(e) {
                        return this.each(function() {
                            delete this[i.propFix[e] || e]
                        })
                    }
                }),
                    i.extend({
                        prop: function(e, t, n) {
                            var r, o, u = e.nodeType;
                            if (!(u === 3 || u === 8 || u === 2))
                                return (u !== 1 || !i.isXMLDoc(e)) && (t = i.propFix[t] || t,
                                    o = i.propHooks[t]),
                                    n !== void 0 ? o && "set"in o && (r = o.set(e, n, t)) !== void 0 ? r : e[t] = n : o && "get"in o && (r = o.get(e, t)) !== null ? r : e[t]
                        },
                        propHooks: {
                            tabIndex: {
                                get: function(e) {
                                    var t = i.find.attr(e, "tabindex");
                                    return t ? parseInt(t, 10) : dr.test(e.nodeName) || pr.test(e.nodeName) && e.href ? 0 : -1
                                }
                            }
                        },
                        propFix: {
                            for: "htmlFor",
                            class: "className"
                        }
                    }),
                q.optSelected || (i.propHooks.selected = {
                    get: function(e) {
                        var t = e.parentNode;
                        return t && t.parentNode && t.parentNode.selectedIndex,
                            null
                    },
                    set: function(e) {
                        var t = e.parentNode;
                        t && (t.selectedIndex,
                        t.parentNode && t.parentNode.selectedIndex)
                    }
                }),
                    i.each(["tabIndex", "readOnly", "maxLength", "cellSpacing", "cellPadding", "rowSpan", "colSpan", "useMap", "frameBorder", "contentEditable"], function() {
                        i.propFix[this.toLowerCase()] = this
                    });
                function Ce(e) {
                    var t = e.match(oe) || [];
                    return t.join(" ")
                }
                function we(e) {
                    return e.getAttribute && e.getAttribute("class") || ""
                }
                function Ct(e) {
                    return Array.isArray(e) ? e : typeof e == "string" ? e.match(oe) || [] : []
                }
                i.fn.extend({
                    addClass: function(e) {
                        var t, n, r, o, u, a;
                        return L(e) ? this.each(function(c) {
                            i(this).addClass(e.call(this, c, we(this)))
                        }) : (t = Ct(e),
                            t.length ? this.each(function() {
                                if (r = we(this),
                                    n = this.nodeType === 1 && " " + Ce(r) + " ",
                                    n) {
                                    for (u = 0; u < t.length; u++)
                                        o = t[u],
                                        n.indexOf(" " + o + " ") < 0 && (n += o + " ");
                                    a = Ce(n),
                                    r !== a && this.setAttribute("class", a)
                                }
                            }) : this)
                    },
                    removeClass: function(e) {
                        var t, n, r, o, u, a;
                        return L(e) ? this.each(function(c) {
                            i(this).removeClass(e.call(this, c, we(this)))
                        }) : arguments.length ? (t = Ct(e),
                            t.length ? this.each(function() {
                                if (r = we(this),
                                    n = this.nodeType === 1 && " " + Ce(r) + " ",
                                    n) {
                                    for (u = 0; u < t.length; u++)
                                        for (o = t[u]; n.indexOf(" " + o + " ") > -1; )
                                            n = n.replace(" " + o + " ", " ");
                                    a = Ce(n),
                                    r !== a && this.setAttribute("class", a)
                                }
                            }) : this) : this.attr("class", "")
                    },
                    toggleClass: function(e, t) {
                        var n, r, o, u, a = typeof e, c = a === "string" || Array.isArray(e);
                        return L(e) ? this.each(function(f) {
                            i(this).toggleClass(e.call(this, f, we(this), t), t)
                        }) : typeof t == "boolean" && c ? t ? this.addClass(e) : this.removeClass(e) : (n = Ct(e),
                            this.each(function() {
                                if (c)
                                    for (u = i(this),
                                             o = 0; o < n.length; o++)
                                        r = n[o],
                                            u.hasClass(r) ? u.removeClass(r) : u.addClass(r);
                                else
                                    (e === void 0 || a === "boolean") && (r = we(this),
                                    r && S.set(this, "__className__", r),
                                    this.setAttribute && this.setAttribute("class", r || e === !1 ? "" : S.get(this, "__className__") || ""))
                            }))
                    },
                    hasClass: function(e) {
                        var t, n, r = 0;
                        for (t = " " + e + " "; n = this[r++]; )
                            if (n.nodeType === 1 && (" " + Ce(we(n)) + " ").indexOf(t) > -1)
                                return !0;
                        return !1
                    }
                });
                var hr = /\r/g;
                i.fn.extend({
                    val: function(e) {
                        var t, n, r, o = this[0];
                        return arguments.length ? (r = L(e),
                            this.each(function(u) {
                                var a;
                                this.nodeType === 1 && (r ? a = e.call(this, u, i(this).val()) : a = e,
                                    a == null ? a = "" : typeof a == "number" ? a += "" : Array.isArray(a) && (a = i.map(a, function(c) {
                                        return c == null ? "" : c + ""
                                    })),
                                    t = i.valHooks[this.type] || i.valHooks[this.nodeName.toLowerCase()],
                                (!t || !("set"in t) || t.set(this, a, "value") === void 0) && (this.value = a))
                            })) : o ? (t = i.valHooks[o.type] || i.valHooks[o.nodeName.toLowerCase()],
                            t && "get"in t && (n = t.get(o, "value")) !== void 0 ? n : (n = o.value,
                                typeof n == "string" ? n.replace(hr, "") : n ?? "")) : void 0
                    }
                }),
                    i.extend({
                        valHooks: {
                            option: {
                                get: function(e) {
                                    var t = i.find.attr(e, "value");
                                    return t ?? Ce(i.text(e))
                                }
                            },
                            select: {
                                get: function(e) {
                                    var t, n, r, o = e.options, u = e.selectedIndex, a = e.type === "select-one", c = a ? null : [], f = a ? u + 1 : o.length;
                                    for (u < 0 ? r = f : r = a ? u : 0; r < f; r++)
                                        if (n = o[r],
                                        (n.selected || r === u) && !n.disabled && (!n.parentNode.disabled || !$(n.parentNode, "optgroup"))) {
                                            if (t = i(n).val(),
                                                a)
                                                return t;
                                            c.push(t)
                                        }
                                    return c
                                },
                                set: function(e, t) {
                                    for (var n, r, o = e.options, u = i.makeArray(t), a = o.length; a--; )
                                        r = o[a],
                                        (r.selected = i.inArray(i.valHooks.option.get(r), u) > -1) && (n = !0);
                                    return n || (e.selectedIndex = -1),
                                        u
                                }
                            }
                        }
                    }),
                    i.each(["radio", "checkbox"], function() {
                        i.valHooks[this] = {
                            set: function(e, t) {
                                if (Array.isArray(t))
                                    return e.checked = i.inArray(i(e).val(), t) > -1
                            }
                        },
                        q.checkOn || (i.valHooks[this].get = function(e) {
                                return e.getAttribute("value") === null ? "on" : e.value
                            }
                        )
                    });
                var Xe = k.location
                    , vn = {
                    guid: Date.now()
                }
                    , wt = /\?/;
                i.parseXML = function(e) {
                    var t, n;
                    if (!e || typeof e != "string")
                        return null;
                    try {
                        t = new k.DOMParser().parseFromString(e, "text/xml")
                    } catch {}
                    return n = t && t.getElementsByTagName("parsererror")[0],
                    (!t || n) && i.error("Invalid XML: " + (n ? i.map(n.childNodes, function(r) {
                        return r.textContent
                    }).join(`
`) : e)),
                        t
                }
                ;
                var bn = /^(?:focusinfocus|focusoutblur)$/
                    , xn = function(e) {
                    e.stopPropagation()
                };
                i.extend(i.event, {
                    trigger: function(e, t, n, r) {
                        var o, u, a, c, f, d, y, b, h = [n || j], x = _e.call(e, "type") ? e.type : e, D = _e.call(e, "namespace") ? e.namespace.split(".") : [];
                        if (u = b = a = n = n || j,
                        !(n.nodeType === 3 || n.nodeType === 8) && !bn.test(x + i.event.triggered) && (x.indexOf(".") > -1 && (D = x.split("."),
                            x = D.shift(),
                            D.sort()),
                            f = x.indexOf(":") < 0 && "on" + x,
                            e = e[i.expando] ? e : new i.Event(x,typeof e == "object" && e),
                            e.isTrigger = r ? 2 : 3,
                            e.namespace = D.join("."),
                            e.rnamespace = e.namespace ? new RegExp("(^|\\.)" + D.join("\\.(?:.*\\.|)") + "(\\.|$)") : null,
                            e.result = void 0,
                        e.target || (e.target = n),
                            t = t == null ? [e] : i.makeArray(t, [e]),
                            y = i.event.special[x] || {},
                            !(!r && y.trigger && y.trigger.apply(n, t) === !1))) {
                            if (!r && !y.noBubble && !Ae(n)) {
                                for (c = y.delegateType || x,
                                     bn.test(c + x) || (u = u.parentNode); u; u = u.parentNode)
                                    h.push(u),
                                        a = u;
                                a === (n.ownerDocument || j) && h.push(a.defaultView || a.parentWindow || k)
                            }
                            for (o = 0; (u = h[o++]) && !e.isPropagationStopped(); )
                                b = u,
                                    e.type = o > 1 ? c : y.bindType || x,
                                    d = (S.get(u, "events") || Object.create(null))[e.type] && S.get(u, "handle"),
                                d && d.apply(u, t),
                                    d = f && u[f],
                                d && d.apply && Fe(u) && (e.result = d.apply(u, t),
                                e.result === !1 && e.preventDefault());
                            return e.type = x,
                            !r && !e.isDefaultPrevented() && (!y._default || y._default.apply(h.pop(), t) === !1) && Fe(n) && f && L(n[x]) && !Ae(n) && (a = n[f],
                            a && (n[f] = null),
                                i.event.triggered = x,
                            e.isPropagationStopped() && b.addEventListener(x, xn),
                                n[x](),
                            e.isPropagationStopped() && b.removeEventListener(x, xn),
                                i.event.triggered = void 0,
                            a && (n[f] = a)),
                                e.result
                        }
                    },
                    simulate: function(e, t, n) {
                        var r = i.extend(new i.Event, n, {
                            type: e,
                            isSimulated: !0
                        });
                        i.event.trigger(r, null, t)
                    }
                }),
                    i.fn.extend({
                        trigger: function(e, t) {
                            return this.each(function() {
                                i.event.trigger(e, t, this)
                            })
                        },
                        triggerHandler: function(e, t) {
                            var n = this[0];
                            if (n)
                                return i.event.trigger(e, t, n, !0)
                        }
                    });
                var gr = /\[\]$/
                    , mn = /\r?\n/g
                    , yr = /^(?:submit|button|image|reset|file)$/i
                    , vr = /^(?:input|select|textarea|keygen)/i;
                function St(e, t, n, r) {
                    var o;
                    if (Array.isArray(t))
                        i.each(t, function(u, a) {
                            n || gr.test(e) ? r(e, a) : St(e + "[" + (typeof a == "object" && a != null ? u : "") + "]", a, n, r)
                        });
                    else if (!n && De(t) === "object")
                        for (o in t)
                            St(e + "[" + o + "]", t[o], n, r);
                    else
                        r(e, t)
                }
                i.param = function(e, t) {
                    var n, r = [], o = function(u, a) {
                        var c = L(a) ? a() : a;
                        r[r.length] = encodeURIComponent(u) + "=" + encodeURIComponent(c ?? "")
                    };
                    if (e == null)
                        return "";
                    if (Array.isArray(e) || e.jquery && !i.isPlainObject(e))
                        i.each(e, function() {
                            o(this.name, this.value)
                        });
                    else
                        for (n in e)
                            St(n, e[n], t, o);
                    return r.join("&")
                }
                    ,
                    i.fn.extend({
                        serialize: function() {
                            return i.param(this.serializeArray())
                        },
                        serializeArray: function() {
                            return this.map(function() {
                                var e = i.prop(this, "elements");
                                return e ? i.makeArray(e) : this
                            }).filter(function() {
                                var e = this.type;
                                return this.name && !i(this).is(":disabled") && vr.test(this.nodeName) && !yr.test(e) && (this.checked || !Ue.test(e))
                            }).map(function(e, t) {
                                var n = i(this).val();
                                return n == null ? null : Array.isArray(n) ? i.map(n, function(r) {
                                    return {
                                        name: t.name,
                                        value: r.replace(mn, `\r
`)
                                    }
                                }) : {
                                    name: t.name,
                                    value: n.replace(mn, `\r
`)
                                }
                            }).get()
                        }
                    });
                var br = /%20/g
                    , xr = /#.*$/
                    , mr = /([?&])_=[^&]*/
                    , Tr = /^(.*?):[ \t]*([^\r\n]*)$/mg
                    , Cr = /^(?:about|app|app-storage|.+-extension|file|res|widget):$/
                    , wr = /^(?:GET|HEAD)$/
                    , Sr = /^\/\//
                    , Tn = {}
                    , Et = {}
                    , Cn = "*/".concat("*")
                    , At = j.createElement("a");
                At.href = Xe.href;
                function wn(e) {
                    return function(t, n) {
                        typeof t != "string" && (n = t,
                            t = "*");
                        var r, o = 0, u = t.toLowerCase().match(oe) || [];
                        if (L(n))
                            for (; r = u[o++]; )
                                r[0] === "+" ? (r = r.slice(1) || "*",
                                    (e[r] = e[r] || []).unshift(n)) : (e[r] = e[r] || []).push(n)
                    }
                }
                function Sn(e, t, n, r) {
                    var o = {}
                        , u = e === Et;
                    function a(c) {
                        var f;
                        return o[c] = !0,
                            i.each(e[c] || [], function(d, y) {
                                var b = y(t, n, r);
                                if (typeof b == "string" && !u && !o[b])
                                    return t.dataTypes.unshift(b),
                                        a(b),
                                        !1;
                                if (u)
                                    return !(f = b)
                            }),
                            f
                    }
                    return a(t.dataTypes[0]) || !o["*"] && a("*")
                }
                function Dt(e, t) {
                    var n, r, o = i.ajaxSettings.flatOptions || {};
                    for (n in t)
                        t[n] !== void 0 && ((o[n] ? e : r || (r = {}))[n] = t[n]);
                    return r && i.extend(!0, e, r),
                        e
                }
                function Er(e, t, n) {
                    for (var r, o, u, a, c = e.contents, f = e.dataTypes; f[0] === "*"; )
                        f.shift(),
                        r === void 0 && (r = e.mimeType || t.getResponseHeader("Content-Type"));
                    if (r) {
                        for (o in c)
                            if (c[o] && c[o].test(r)) {
                                f.unshift(o);
                                break
                            }
                    }
                    if (f[0]in n)
                        u = f[0];
                    else {
                        for (o in n) {
                            if (!f[0] || e.converters[o + " " + f[0]]) {
                                u = o;
                                break
                            }
                            a || (a = o)
                        }
                        u = u || a
                    }
                    if (u)
                        return u !== f[0] && f.unshift(u),
                            n[u]
                }
                function Ar(e, t, n, r) {
                    var o, u, a, c, f, d = {}, y = e.dataTypes.slice();
                    if (y[1])
                        for (a in e.converters)
                            d[a.toLowerCase()] = e.converters[a];
                    for (u = y.shift(); u; )
                        if (e.responseFields[u] && (n[e.responseFields[u]] = t),
                        !f && r && e.dataFilter && (t = e.dataFilter(t, e.dataType)),
                            f = u,
                            u = y.shift(),
                            u) {
                            if (u === "*")
                                u = f;
                            else if (f !== "*" && f !== u) {
                                if (a = d[f + " " + u] || d["* " + u],
                                    !a) {
                                    for (o in d)
                                        if (c = o.split(" "),
                                        c[1] === u && (a = d[f + " " + c[0]] || d["* " + c[0]],
                                            a)) {
                                            a === !0 ? a = d[o] : d[o] !== !0 && (u = c[0],
                                                y.unshift(c[1]));
                                            break
                                        }
                                }
                                if (a !== !0)
                                    if (a && e.throws)
                                        t = a(t);
                                    else
                                        try {
                                            t = a(t)
                                        } catch (b) {
                                            return {
                                                state: "parsererror",
                                                error: a ? b : "No conversion from " + f + " to " + u
                                            }
                                        }
                            }
                        }
                    return {
                        state: "success",
                        data: t
                    }
                }
                i.extend({
                    active: 0,
                    lastModified: {},
                    etag: {},
                    ajaxSettings: {
                        url: Xe.href,
                        type: "GET",
                        isLocal: Cr.test(Xe.protocol),
                        global: !0,
                        processData: !0,
                        async: !0,
                        contentType: "application/x-www-form-urlencoded; charset=UTF-8",
                        accepts: {
                            "*": Cn,
                            text: "text/plain",
                            html: "text/html",
                            xml: "application/xml, text/xml",
                            json: "application/json, text/javascript"
                        },
                        contents: {
                            xml: /\bxml\b/,
                            html: /\bhtml/,
                            json: /\bjson\b/
                        },
                        responseFields: {
                            xml: "responseXML",
                            text: "responseText",
                            json: "responseJSON"
                        },
                        converters: {
                            "* text": String,
                            "text html": !0,
                            "text json": JSON.parse,
                            "text xml": i.parseXML
                        },
                        flatOptions: {
                            url: !0,
                            context: !0
                        }
                    },
                    ajaxSetup: function(e, t) {
                        return t ? Dt(Dt(e, i.ajaxSettings), t) : Dt(i.ajaxSettings, e)
                    },
                    ajaxPrefilter: wn(Tn),
                    ajaxTransport: wn(Et),
                    ajax: function(e, t) {
                        typeof e == "object" && (t = e,
                            e = void 0),
                            t = t || {};
                        var n, r, o, u, a, c, f, d, y, b, h = i.ajaxSetup({}, t), x = h.context || h, D = h.context && (x.nodeType || x.jquery) ? i(x) : i.event, R = i.Deferred(), H = i.Callbacks("once memory"), V = h.statusCode || {}, U = {}, ae = {}, se = "canceled", M = {
                            readyState: 0,
                            getResponseHeader: function(I) {
                                var B;
                                if (f) {
                                    if (!u)
                                        for (u = {}; B = Tr.exec(o); )
                                            u[B[1].toLowerCase() + " "] = (u[B[1].toLowerCase() + " "] || []).concat(B[2]);
                                    B = u[I.toLowerCase() + " "]
                                }
                                return B == null ? null : B.join(", ")
                            },
                            getAllResponseHeaders: function() {
                                return f ? o : null
                            },
                            setRequestHeader: function(I, B) {
                                return f == null && (I = ae[I.toLowerCase()] = ae[I.toLowerCase()] || I,
                                    U[I] = B),
                                    this
                            },
                            overrideMimeType: function(I) {
                                return f == null && (h.mimeType = I),
                                    this
                            },
                            statusCode: function(I) {
                                var B;
                                if (I)
                                    if (f)
                                        M.always(I[M.status]);
                                    else
                                        for (B in I)
                                            V[B] = [V[B], I[B]];
                                return this
                            },
                            abort: function(I) {
                                var B = I || se;
                                return n && n.abort(B),
                                    Se(0, B),
                                    this
                            }
                        };
                        if (R.promise(M),
                            h.url = ((e || h.url || Xe.href) + "").replace(Sr, Xe.protocol + "//"),
                            h.type = t.method || t.type || h.method || h.type,
                            h.dataTypes = (h.dataType || "*").toLowerCase().match(oe) || [""],
                        h.crossDomain == null) {
                            c = j.createElement("a");
                            try {
                                c.href = h.url,
                                    c.href = c.href,
                                    h.crossDomain = At.protocol + "//" + At.host != c.protocol + "//" + c.host
                            } catch {
                                h.crossDomain = !0
                            }
                        }
                        if (h.data && h.processData && typeof h.data != "string" && (h.data = i.param(h.data, h.traditional)),
                            Sn(Tn, h, t, M),
                            f)
                            return M;
                        d = i.event && h.global,
                        d && i.active++ === 0 && i.event.trigger("ajaxStart"),
                            h.type = h.type.toUpperCase(),
                            h.hasContent = !wr.test(h.type),
                            r = h.url.replace(xr, ""),
                            h.hasContent ? h.data && h.processData && (h.contentType || "").indexOf("application/x-www-form-urlencoded") === 0 && (h.data = h.data.replace(br, "+")) : (b = h.url.slice(r.length),
                            h.data && (h.processData || typeof h.data == "string") && (r += (wt.test(r) ? "&" : "?") + h.data,
                                delete h.data),
                            h.cache === !1 && (r = r.replace(mr, "$1"),
                                b = (wt.test(r) ? "&" : "?") + "_=" + vn.guid++ + b),
                                h.url = r + b),
                        h.ifModified && (i.lastModified[r] && M.setRequestHeader("If-Modified-Since", i.lastModified[r]),
                        i.etag[r] && M.setRequestHeader("If-None-Match", i.etag[r])),
                        (h.data && h.hasContent && h.contentType !== !1 || t.contentType) && M.setRequestHeader("Content-Type", h.contentType),
                            M.setRequestHeader("Accept", h.dataTypes[0] && h.accepts[h.dataTypes[0]] ? h.accepts[h.dataTypes[0]] + (h.dataTypes[0] !== "*" ? ", " + Cn + "; q=0.01" : "") : h.accepts["*"]);
                        for (y in h.headers)
                            M.setRequestHeader(y, h.headers[y]);
                        if (h.beforeSend && (h.beforeSend.call(x, M, h) === !1 || f))
                            return M.abort();
                        if (se = "abort",
                            H.add(h.complete),
                            M.done(h.success),
                            M.fail(h.error),
                            n = Sn(Et, h, t, M),
                            !n)
                            Se(-1, "No Transport");
                        else {
                            if (M.readyState = 1,
                            d && D.trigger("ajaxSend", [M, h]),
                                f)
                                return M;
                            h.async && h.timeout > 0 && (a = k.setTimeout(function() {
                                M.abort("timeout")
                            }, h.timeout));
                            try {
                                f = !1,
                                    n.send(U, Se)
                            } catch (I) {
                                if (f)
                                    throw I;
                                Se(-1, I)
                            }
                        }
                        function Se(I, B, Qe, kt) {
                            var fe, Ye, ce, be, xe, te = B;
                            f || (f = !0,
                            a && k.clearTimeout(a),
                                n = void 0,
                                o = kt || "",
                                M.readyState = I > 0 ? 4 : 0,
                                fe = I >= 200 && I < 300 || I === 304,
                            Qe && (be = Er(h, M, Qe)),
                            !fe && i.inArray("script", h.dataTypes) > -1 && i.inArray("json", h.dataTypes) < 0 && (h.converters["text script"] = function() {}
                            ),
                                be = Ar(h, be, M, fe),
                                fe ? (h.ifModified && (xe = M.getResponseHeader("Last-Modified"),
                                xe && (i.lastModified[r] = xe),
                                    xe = M.getResponseHeader("etag"),
                                xe && (i.etag[r] = xe)),
                                    I === 204 || h.type === "HEAD" ? te = "nocontent" : I === 304 ? te = "notmodified" : (te = be.state,
                                        Ye = be.data,
                                        ce = be.error,
                                        fe = !ce)) : (ce = te,
                                (I || !te) && (te = "error",
                                I < 0 && (I = 0))),
                                M.status = I,
                                M.statusText = (B || te) + "",
                                fe ? R.resolveWith(x, [Ye, te, M]) : R.rejectWith(x, [M, te, ce]),
                                M.statusCode(V),
                                V = void 0,
                            d && D.trigger(fe ? "ajaxSuccess" : "ajaxError", [M, h, fe ? Ye : ce]),
                                H.fireWith(x, [M, te]),
                            d && (D.trigger("ajaxComplete", [M, h]),
                            --i.active || i.event.trigger("ajaxStop")))
                        }
                        return M
                    },
                    getJSON: function(e, t, n) {
                        return i.get(e, t, n, "json")
                    },
                    getScript: function(e, t) {
                        return i.get(e, void 0, t, "script")
                    }
                }),
                    i.each(["get", "post"], function(e, t) {
                        i[t] = function(n, r, o, u) {
                            return L(r) && (u = u || o,
                                o = r,
                                r = void 0),
                                i.ajax(i.extend({
                                    url: n,
                                    type: t,
                                    dataType: u,
                                    data: r,
                                    success: o
                                }, i.isPlainObject(n) && n))
                        }
                    }),
                    i.ajaxPrefilter(function(e) {
                        var t;
                        for (t in e.headers)
                            t.toLowerCase() === "content-type" && (e.contentType = e.headers[t] || "")
                    }),
                    i._evalUrl = function(e, t, n) {
                        return i.ajax({
                            url: e,
                            type: "GET",
                            dataType: "script",
                            cache: !0,
                            async: !1,
                            global: !1,
                            converters: {
                                "text script": function() {}
                            },
                            dataFilter: function(r) {
                                i.globalEval(r, t, n)
                            }
                        })
                    }
                    ,
                    i.fn.extend({
                        wrapAll: function(e) {
                            var t;
                            return this[0] && (L(e) && (e = e.call(this[0])),
                                t = i(e, this[0].ownerDocument).eq(0).clone(!0),
                            this[0].parentNode && t.insertBefore(this[0]),
                                t.map(function() {
                                    for (var n = this; n.firstElementChild; )
                                        n = n.firstElementChild;
                                    return n
                                }).append(this)),
                                this
                        },
                        wrapInner: function(e) {
                            return L(e) ? this.each(function(t) {
                                i(this).wrapInner(e.call(this, t))
                            }) : this.each(function() {
                                var t = i(this)
                                    , n = t.contents();
                                n.length ? n.wrapAll(e) : t.append(e)
                            })
                        },
                        wrap: function(e) {
                            var t = L(e);
                            return this.each(function(n) {
                                i(this).wrapAll(t ? e.call(this, n) : e)
                            })
                        },
                        unwrap: function(e) {
                            return this.parent(e).not("body").each(function() {
                                i(this).replaceWith(this.childNodes)
                            }),
                                this
                        }
                    }),
                    i.expr.pseudos.hidden = function(e) {
                        return !i.expr.pseudos.visible(e)
                    }
                    ,
                    i.expr.pseudos.visible = function(e) {
                        return !!(e.offsetWidth || e.offsetHeight || e.getClientRects().length)
                    }
                    ,
                    i.ajaxSettings.xhr = function() {
                        try {
                            return new k.XMLHttpRequest
                        } catch {}
                    }
                ;
                var Dr = {
                    0: 200,
                    1223: 204
                }
                    , Ge = i.ajaxSettings.xhr();
                q.cors = !!Ge && "withCredentials"in Ge,
                    q.ajax = Ge = !!Ge,
                    i.ajaxTransport(function(e) {
                        var t, n;
                        if (q.cors || Ge && !e.crossDomain)
                            return {
                                send: function(r, o) {
                                    var u, a = e.xhr();
                                    if (a.open(e.type, e.url, e.async, e.username, e.password),
                                        e.xhrFields)
                                        for (u in e.xhrFields)
                                            a[u] = e.xhrFields[u];
                                    e.mimeType && a.overrideMimeType && a.overrideMimeType(e.mimeType),
                                    !e.crossDomain && !r["X-Requested-With"] && (r["X-Requested-With"] = "XMLHttpRequest");
                                    for (u in r)
                                        a.setRequestHeader(u, r[u]);
                                    t = function(c) {
                                        return function() {
                                            t && (t = n = a.onload = a.onerror = a.onabort = a.ontimeout = a.onreadystatechange = null,
                                                c === "abort" ? a.abort() : c === "error" ? typeof a.status != "number" ? o(0, "error") : o(a.status, a.statusText) : o(Dr[a.status] || a.status, a.statusText, (a.responseType || "text") !== "text" || typeof a.responseText != "string" ? {
                                                    binary: a.response
                                                } : {
                                                    text: a.responseText
                                                }, a.getAllResponseHeaders()))
                                        }
                                    }
                                        ,
                                        a.onload = t(),
                                        n = a.onerror = a.ontimeout = t("error"),
                                        a.onabort !== void 0 ? a.onabort = n : a.onreadystatechange = function() {
                                            a.readyState === 4 && k.setTimeout(function() {
                                                t && n()
                                            })
                                        }
                                        ,
                                        t = t("abort");
                                    try {
                                        a.send(e.hasContent && e.data || null)
                                    } catch (c) {
                                        if (t)
                                            throw c
                                    }
                                },
                                abort: function() {
                                    t && t()
                                }
                            }
                    }),
                    i.ajaxPrefilter(function(e) {
                        e.crossDomain && (e.contents.script = !1)
                    }),
                    i.ajaxSetup({
                        accepts: {
                            script: "text/javascript, application/javascript, application/ecmascript, application/x-ecmascript"
                        },
                        contents: {
                            script: /\b(?:java|ecma)script\b/
                        },
                        converters: {
                            "text script": function(e) {
                                return i.globalEval(e),
                                    e
                            }
                        }
                    }),
                    i.ajaxPrefilter("script", function(e) {
                        e.cache === void 0 && (e.cache = !1),
                        e.crossDomain && (e.type = "GET")
                    }),
                    i.ajaxTransport("script", function(e) {
                        if (e.crossDomain || e.scriptAttrs) {
                            var t, n;
                            return {
                                send: function(r, o) {
                                    t = i("<script>").attr(e.scriptAttrs || {}).prop({
                                        charset: e.scriptCharset,
                                        src: e.url
                                    }).on("load error", n = function(u) {
                                            t.remove(),
                                                n = null,
                                            u && o(u.type === "error" ? 404 : 200, u.type)
                                        }
                                    ),
                                        j.head.appendChild(t[0])
                                },
                                abort: function() {
                                    n && n()
                                }
                            }
                        }
                    });
                var En = []
                    , Nt = /(=)\?(?=&|$)|\?\?/;
                i.ajaxSetup({
                    jsonp: "callback",
                    jsonpCallback: function() {
                        var e = En.pop() || i.expando + "_" + vn.guid++;
                        return this[e] = !0,
                            e
                    }
                }),
                    i.ajaxPrefilter("json jsonp", function(e, t, n) {
                        var r, o, u, a = e.jsonp !== !1 && (Nt.test(e.url) ? "url" : typeof e.data == "string" && (e.contentType || "").indexOf("application/x-www-form-urlencoded") === 0 && Nt.test(e.data) && "data");
                        if (a || e.dataTypes[0] === "jsonp")
                            return r = e.jsonpCallback = L(e.jsonpCallback) ? e.jsonpCallback() : e.jsonpCallback,
                                a ? e[a] = e[a].replace(Nt, "$1" + r) : e.jsonp !== !1 && (e.url += (wt.test(e.url) ? "&" : "?") + e.jsonp + "=" + r),
                                e.converters["script json"] = function() {
                                    return u || i.error(r + " was not called"),
                                        u[0]
                                }
                                ,
                                e.dataTypes[0] = "json",
                                o = k[r],
                                k[r] = function() {
                                    u = arguments
                                }
                                ,
                                n.always(function() {
                                    o === void 0 ? i(k).removeProp(r) : k[r] = o,
                                    e[r] && (e.jsonpCallback = t.jsonpCallback,
                                        En.push(r)),
                                    u && L(o) && o(u[0]),
                                        u = o = void 0
                                }),
                                "script"
                    }),
                    q.createHTMLDocument = function() {
                        var e = j.implementation.createHTMLDocument("").body;
                        return e.innerHTML = "<form></form><form></form>",
                        e.childNodes.length === 2
                    }(),
                    i.parseHTML = function(e, t, n) {
                        if (typeof e != "string")
                            return [];
                        typeof t == "boolean" && (n = t,
                            t = !1);
                        var r, o, u;
                        return t || (q.createHTMLDocument ? (t = j.implementation.createHTMLDocument(""),
                            r = t.createElement("base"),
                            r.href = j.location.href,
                            t.head.appendChild(r)) : t = j),
                            o = Ut.exec(e),
                            u = !n && [],
                            o ? [t.createElement(o[1])] : (o = en([e], t, u),
                            u && u.length && i(u).remove(),
                                i.merge([], o.childNodes))
                    }
                    ,
                    i.fn.load = function(e, t, n) {
                        var r, o, u, a = this, c = e.indexOf(" ");
                        return c > -1 && (r = Ce(e.slice(c)),
                            e = e.slice(0, c)),
                            L(t) ? (n = t,
                                t = void 0) : t && typeof t == "object" && (o = "POST"),
                        a.length > 0 && i.ajax({
                            url: e,
                            type: o || "GET",
                            dataType: "html",
                            data: t
                        }).done(function(f) {
                            u = arguments,
                                a.html(r ? i("<div>").append(i.parseHTML(f)).find(r) : f)
                        }).always(n && function(f, d) {
                            a.each(function() {
                                n.apply(this, u || [f.responseText, d, f])
                            })
                        }
                        ),
                            this
                    }
                    ,
                    i.expr.pseudos.animated = function(e) {
                        return i.grep(i.timers, function(t) {
                            return e === t.elem
                        }).length
                    }
                    ,
                    i.offset = {
                        setOffset: function(e, t, n) {
                            var r, o, u, a, c, f, d, y = i.css(e, "position"), b = i(e), h = {};
                            y === "static" && (e.style.position = "relative"),
                                c = b.offset(),
                                u = i.css(e, "top"),
                                f = i.css(e, "left"),
                                d = (y === "absolute" || y === "fixed") && (u + f).indexOf("auto") > -1,
                                d ? (r = b.position(),
                                    a = r.top,
                                    o = r.left) : (a = parseFloat(u) || 0,
                                    o = parseFloat(f) || 0),
                            L(t) && (t = t.call(e, n, i.extend({}, c))),
                            t.top != null && (h.top = t.top - c.top + a),
                            t.left != null && (h.left = t.left - c.left + o),
                                "using"in t ? t.using.call(e, h) : b.css(h)
                        }
                    },
                    i.fn.extend({
                        offset: function(e) {
                            if (arguments.length)
                                return e === void 0 ? this : this.each(function(o) {
                                    i.offset.setOffset(this, e, o)
                                });
                            var t, n, r = this[0];
                            if (r)
                                return r.getClientRects().length ? (t = r.getBoundingClientRect(),
                                    n = r.ownerDocument.defaultView,
                                    {
                                        top: t.top + n.pageYOffset,
                                        left: t.left + n.pageXOffset
                                    }) : {
                                    top: 0,
                                    left: 0
                                }
                        },
                        position: function() {
                            if (this[0]) {
                                var e, t, n, r = this[0], o = {
                                    top: 0,
                                    left: 0
                                };
                                if (i.css(r, "position") === "fixed")
                                    t = r.getBoundingClientRect();
                                else {
                                    for (t = this.offset(),
                                             n = r.ownerDocument,
                                             e = r.offsetParent || n.documentElement; e && (e === n.body || e === n.documentElement) && i.css(e, "position") === "static"; )
                                        e = e.parentNode;
                                    e && e !== r && e.nodeType === 1 && (o = i(e).offset(),
                                        o.top += i.css(e, "borderTopWidth", !0),
                                        o.left += i.css(e, "borderLeftWidth", !0))
                                }
                                return {
                                    top: t.top - o.top - i.css(r, "marginTop", !0),
                                    left: t.left - o.left - i.css(r, "marginLeft", !0)
                                }
                            }
                        },
                        offsetParent: function() {
                            return this.map(function() {
                                for (var e = this.offsetParent; e && i.css(e, "position") === "static"; )
                                    e = e.offsetParent;
                                return e || Te
                            })
                        }
                    }),
                    i.each({
                        scrollLeft: "pageXOffset",
                        scrollTop: "pageYOffset"
                    }, function(e, t) {
                        var n = t === "pageYOffset";
                        i.fn[e] = function(r) {
                            return he(this, function(o, u, a) {
                                var c;
                                if (Ae(o) ? c = o : o.nodeType === 9 && (c = o.defaultView),
                                a === void 0)
                                    return c ? c[t] : o[u];
                                c ? c.scrollTo(n ? c.pageXOffset : a, n ? a : c.pageYOffset) : o[u] = a
                            }, e, r, arguments.length)
                        }
                    }),
                    i.each(["top", "left"], function(e, t) {
                        i.cssHooks[t] = an(q.pixelPosition, function(n, r) {
                            if (r)
                                return r = ze(n, t),
                                    vt.test(r) ? i(n).position()[t] + "px" : r
                        })
                    }),
                    i.each({
                        Height: "height",
                        Width: "width"
                    }, function(e, t) {
                        i.each({
                            padding: "inner" + e,
                            content: t,
                            "": "outer" + e
                        }, function(n, r) {
                            i.fn[r] = function(o, u) {
                                var a = arguments.length && (n || typeof o != "boolean")
                                    , c = n || (o === !0 || u === !0 ? "margin" : "border");
                                return he(this, function(f, d, y) {
                                    var b;
                                    return Ae(f) ? r.indexOf("outer") === 0 ? f["inner" + e] : f.document.documentElement["client" + e] : f.nodeType === 9 ? (b = f.documentElement,
                                        Math.max(f.body["scroll" + e], b["scroll" + e], f.body["offset" + e], b["offset" + e], b["client" + e])) : y === void 0 ? i.css(f, d, c) : i.style(f, d, y, c)
                                }, t, a ? o : void 0, a)
                            }
                        })
                    }),
                    i.each(["ajaxStart", "ajaxStop", "ajaxComplete", "ajaxError", "ajaxSuccess", "ajaxSend"], function(e, t) {
                        i.fn[t] = function(n) {
                            return this.on(t, n)
                        }
                    }),
                    i.fn.extend({
                        bind: function(e, t, n) {
                            return this.on(e, null, t, n)
                        },
                        unbind: function(e, t) {
                            return this.off(e, null, t)
                        },
                        delegate: function(e, t, n, r) {
                            return this.on(t, e, n, r)
                        },
                        undelegate: function(e, t, n) {
                            return arguments.length === 1 ? this.off(e, "**") : this.off(t, e || "**", n)
                        },
                        hover: function(e, t) {
                            return this.mouseenter(e).mouseleave(t || e)
                        }
                    }),
                    i.each("blur focus focusin focusout resize scroll click dblclick mousedown mouseup mousemove mouseover mouseout mouseenter mouseleave change select submit keydown keypress keyup contextmenu".split(" "), function(e, t) {
                        i.fn[t] = function(n, r) {
                            return arguments.length > 0 ? this.on(t, null, n, r) : this.trigger(t)
                        }
                    });
                var Nr = /^[\s\uFEFF\xA0]+|([^\s\uFEFF\xA0])[\s\uFEFF\xA0]+$/g;
                i.proxy = function(e, t) {
                    var n, r, o;
                    if (typeof t == "string" && (n = e[t],
                        t = e,
                        e = n),
                        !!L(e))
                        return r = Z.call(arguments, 2),
                            o = function() {
                                return e.apply(t || this, r.concat(Z.call(arguments)))
                            }
                            ,
                            o.guid = e.guid = e.guid || i.guid++,
                            o
                }
                    ,
                    i.holdReady = function(e) {
                        e ? i.readyWait++ : i.ready(!0)
                    }
                    ,
                    i.isArray = Array.isArray,
                    i.parseJSON = JSON.parse,
                    i.nodeName = $,
                    i.isFunction = L,
                    i.isWindow = Ae,
                    i.camelCase = ue,
                    i.type = De,
                    i.now = Date.now,
                    i.isNumeric = function(e) {
                        var t = i.type(e);
                        return (t === "number" || t === "string") && !isNaN(e - parseFloat(e))
                    }
                    ,
                    i.trim = function(e) {
                        return e == null ? "" : (e + "").replace(Nr, "$1")
                    }
                    ,
                typeof define == "function" && define.amd && define("jquery", [], function() {
                    return i
                });
                var kr = k.jQuery
                    , jr = k.$;
                return i.noConflict = function(e) {
                    return k.$ === i && (k.$ = jr),
                    e && k.jQuery === i && (k.jQuery = kr),
                        i
                }
                    ,
                typeof Ie > "u" && (k.jQuery = k.$ = i),
                    i
            });
        }
    }["(module ep_etherpad-lite/static/js/vendors/jquery.js)"],
    "ep_etherpad-lite/static/js/rjquery.js": {
        "(module ep_etherpad-lite/static/js/rjquery.js)": function(require, exports, module) {
            "use strict";
            window.$ = require("./vendors/jquery");
            const jq = window.$.noConflict(!0);
            exports.jQuery = exports.$ = jq;
        }
    }["(module ep_etherpad-lite/static/js/rjquery.js)"],
    "async.js": null,
    "ep_etherpad-lite/static/js/underscore.js": {
        "(module ep_etherpad-lite/static/js/underscore.js)": function(require, exports, module) {
            "use strict";
            module.exports = require("underscore");
        }
    }["(module ep_etherpad-lite/static/js/underscore.js)"],
    "underscore.js": null,
    "underscore/underscore.js": {
        "(module underscore/underscore.js)": function(require, exports, module) {
            (function(O, P) {
                    typeof exports == "object" && typeof module < "u" ? module.exports = P() : typeof define == "function" && define.amd ? define("underscore", P) : (O = typeof globalThis < "u" ? globalThis : O || self,
                        function() {
                            var S = O._
                                , R = O._ = P();
                            R.noConflict = function() {
                                return O._ = S,
                                    R
                            }
                        }())
                }
            )(this, function() {
                var O = "1.13.6"
                    , P = typeof self == "object" && self.self === self && self || typeof global == "object" && global.global === global && global || Function("return this")() || {}
                    , S = Array.prototype
                    , R = Object.prototype
                    , dr = typeof Symbol < "u" ? Symbol.prototype : null
                    , Bn = S.push
                    , F = S.slice
                    , $ = R.toString
                    , Nn = R.hasOwnProperty
                    , wr = typeof ArrayBuffer < "u"
                    , Pn = typeof DataView < "u"
                    , Sn = Array.isArray
                    , _r = Object.keys
                    , Ar = Object.create
                    , Or = wr && ArrayBuffer.isView
                    , Rn = isNaN
                    , Tn = isFinite
                    , Er = !{
                    toString: null
                }.propertyIsEnumerable("toString")
                    , Mr = ["valueOf", "isPrototypeOf", "toString", "propertyIsEnumerable", "hasOwnProperty", "toLocaleString"]
                    , Dn = Math.pow(2, 53) - 1;
                function m(r, n) {
                    return n = n == null ? r.length - 1 : +n,
                        function() {
                            for (var e = Math.max(arguments.length - n, 0), t = Array(e), i = 0; i < e; i++)
                                t[i] = arguments[i + n];
                            switch (n) {
                                case 0:
                                    return r.call(this, t);
                                case 1:
                                    return r.call(this, arguments[0], t);
                                case 2:
                                    return r.call(this, arguments[0], arguments[1], t)
                            }
                            var u = Array(n + 1);
                            for (i = 0; i < n; i++)
                                u[i] = arguments[i];
                            return u[n] = t,
                                r.apply(this, u)
                        }
                }
                function M(r) {
                    var n = typeof r;
                    return n === "function" || n === "object" && !!r
                }
                function Vn(r) {
                    return r === null
                }
                function Ir(r) {
                    return r === void 0
                }
                function Br(r) {
                    return r === !0 || r === !1 || $.call(r) === "[object Boolean]"
                }
                function Fn(r) {
                    return !!(r && r.nodeType === 1)
                }
                function p(r) {
                    var n = "[object " + r + "]";
                    return function(e) {
                        return $.call(e) === n
                    }
                }
                var H = p("String")
                    , Nr = p("Number")
                    , $n = p("Date")
                    , zn = p("RegExp")
                    , qn = p("Error")
                    , Pr = p("Symbol")
                    , Sr = p("ArrayBuffer")
                    , Rr = p("Function")
                    , Ln = P.document && P.document.childNodes;
                typeof /./ != "function" && typeof Int8Array != "object" && typeof Ln != "function" && (Rr = function(r) {
                        return typeof r == "function" || !1
                    }
                );
                var h = Rr
                    , Tr = p("Object")
                    , Dr = Pn && Tr(new DataView(new ArrayBuffer(8)))
                    , Q = typeof Map < "u" && Tr(new Map)
                    , Wn = p("DataView");
                function Un(r) {
                    return r != null && h(r.getInt8) && Sr(r.buffer)
                }
                var C = Dr ? Un : Wn
                    , I = Sn || p("Array");
                function E(r, n) {
                    return r != null && Nn.call(r, n)
                }
                var Z = p("Arguments");
                (function() {
                        Z(arguments) || (Z = function(r) {
                                return E(r, "callee")
                            }
                        )
                    }
                )();
                var K = Z;
                function Cn(r) {
                    return !Pr(r) && Tn(r) && !isNaN(parseFloat(r))
                }
                function Vr(r) {
                    return Nr(r) && Rn(r)
                }
                function Fr(r) {
                    return function() {
                        return r
                    }
                }
                function $r(r) {
                    return function(n) {
                        var e = r(n);
                        return typeof e == "number" && e >= 0 && e <= Dn
                    }
                }
                function zr(r) {
                    return function(n) {
                        return n?.[r]
                    }
                }
                var X = zr("byteLength")
                    , Xn = $r(X)
                    , Gn = /\[object ((I|Ui)nt(8|16|32)|Float(32|64)|Uint8Clamped|Big(I|Ui)nt64)Array\]/;
                function Jn(r) {
                    return Or ? Or(r) && !C(r) : Xn(r) && Gn.test($.call(r))
                }
                var qr = wr ? Jn : Fr(!1)
                    , y = zr("length");
                function Yn(r) {
                    for (var n = {}, e = r.length, t = 0; t < e; ++t)
                        n[r[t]] = !0;
                    return {
                        contains: function(i) {
                            return n[i] === !0
                        },
                        push: function(i) {
                            return n[i] = !0,
                                r.push(i)
                        }
                    }
                }
                function Lr(r, n) {
                    n = Yn(n);
                    var e = Mr.length
                        , t = r.constructor
                        , i = h(t) && t.prototype || R
                        , u = "constructor";
                    for (E(r, u) && !n.contains(u) && n.push(u); e--; )
                        u = Mr[e],
                        u in r && r[u] !== i[u] && !n.contains(u) && n.push(u)
                }
                function s(r) {
                    if (!M(r))
                        return [];
                    if (_r)
                        return _r(r);
                    var n = [];
                    for (var e in r)
                        E(r, e) && n.push(e);
                    return Er && Lr(r, n),
                        n
                }
                function Hn(r) {
                    if (r == null)
                        return !0;
                    var n = y(r);
                    return typeof n == "number" && (I(r) || H(r) || K(r)) ? n === 0 : y(s(r)) === 0
                }
                function Wr(r, n) {
                    var e = s(n)
                        , t = e.length;
                    if (r == null)
                        return !t;
                    for (var i = Object(r), u = 0; u < t; u++) {
                        var f = e[u];
                        if (n[f] !== i[f] || !(f in i))
                            return !1
                    }
                    return !0
                }
                function o(r) {
                    if (r instanceof o)
                        return r;
                    if (!(this instanceof o))
                        return new o(r);
                    this._wrapped = r
                }
                o.VERSION = O,
                    o.prototype.value = function() {
                        return this._wrapped
                    }
                    ,
                    o.prototype.valueOf = o.prototype.toJSON = o.prototype.value,
                    o.prototype.toString = function() {
                        return String(this._wrapped)
                    }
                ;
                function Ur(r) {
                    return new Uint8Array(r.buffer || r,r.byteOffset || 0,X(r))
                }
                var Cr = "[object DataView]";
                function x(r, n, e, t) {
                    if (r === n)
                        return r !== 0 || 1 / r === 1 / n;
                    if (r == null || n == null)
                        return !1;
                    if (r !== r)
                        return n !== n;
                    var i = typeof r;
                    return i !== "function" && i !== "object" && typeof n != "object" ? !1 : Xr(r, n, e, t)
                }
                function Xr(r, n, e, t) {
                    r instanceof o && (r = r._wrapped),
                    n instanceof o && (n = n._wrapped);
                    var i = $.call(r);
                    if (i !== $.call(n))
                        return !1;
                    if (Dr && i == "[object Object]" && C(r)) {
                        if (!C(n))
                            return !1;
                        i = Cr
                    }
                    switch (i) {
                        case "[object RegExp]":
                        case "[object String]":
                            return "" + r == "" + n;
                        case "[object Number]":
                            return +r != +r ? +n != +n : +r == 0 ? 1 / +r === 1 / n : +r == +n;
                        case "[object Date]":
                        case "[object Boolean]":
                            return +r == +n;
                        case "[object Symbol]":
                            return dr.valueOf.call(r) === dr.valueOf.call(n);
                        case "[object ArrayBuffer]":
                        case Cr:
                            return Xr(Ur(r), Ur(n), e, t)
                    }
                    var u = i === "[object Array]";
                    if (!u && qr(r)) {
                        var f = X(r);
                        if (f !== X(n))
                            return !1;
                        if (r.buffer === n.buffer && r.byteOffset === n.byteOffset)
                            return !0;
                        u = !0
                    }
                    if (!u) {
                        if (typeof r != "object" || typeof n != "object")
                            return !1;
                        var a = r.constructor
                            , c = n.constructor;
                        if (a !== c && !(h(a) && a instanceof a && h(c) && c instanceof c) && "constructor"in r && "constructor"in n)
                            return !1
                    }
                    e = e || [],
                        t = t || [];
                    for (var l = e.length; l--; )
                        if (e[l] === r)
                            return t[l] === n;
                    if (e.push(r),
                        t.push(n),
                        u) {
                        if (l = r.length,
                        l !== n.length)
                            return !1;
                        for (; l--; )
                            if (!x(r[l], n[l], e, t))
                                return !1
                    } else {
                        var v = s(r), g;
                        if (l = v.length,
                        s(n).length !== l)
                            return !1;
                        for (; l--; )
                            if (g = v[l],
                                !(E(n, g) && x(r[g], n[g], e, t)))
                                return !1
                    }
                    return e.pop(),
                        t.pop(),
                        !0
                }
                function Qn(r, n) {
                    return x(r, n)
                }
                function z(r) {
                    if (!M(r))
                        return [];
                    var n = [];
                    for (var e in r)
                        n.push(e);
                    return Er && Lr(r, n),
                        n
                }
                function k(r) {
                    var n = y(r);
                    return function(e) {
                        if (e == null)
                            return !1;
                        var t = z(e);
                        if (y(t))
                            return !1;
                        for (var i = 0; i < n; i++)
                            if (!h(e[r[i]]))
                                return !1;
                        return r !== Yr || !h(e[b])
                    }
                }
                var b = "forEach"
                    , Gr = "has"
                    , j = ["clear", "delete"]
                    , Jr = ["get", Gr, "set"]
                    , Zn = j.concat(b, Jr)
                    , Yr = j.concat(Jr)
                    , Kn = ["add"].concat(j, b, Gr)
                    , xn = Q ? k(Zn) : p("Map")
                    , kn = Q ? k(Yr) : p("WeakMap")
                    , bn = Q ? k(Kn) : p("Set")
                    , jn = p("WeakSet");
                function T(r) {
                    for (var n = s(r), e = n.length, t = Array(e), i = 0; i < e; i++)
                        t[i] = r[n[i]];
                    return t
                }
                function re(r) {
                    for (var n = s(r), e = n.length, t = Array(e), i = 0; i < e; i++)
                        t[i] = [n[i], r[n[i]]];
                    return t
                }
                function Hr(r) {
                    for (var n = {}, e = s(r), t = 0, i = e.length; t < i; t++)
                        n[r[e[t]]] = e[t];
                    return n
                }
                function rr(r) {
                    var n = [];
                    for (var e in r)
                        h(r[e]) && n.push(e);
                    return n.sort()
                }
                function nr(r, n) {
                    return function(e) {
                        var t = arguments.length;
                        if (n && (e = Object(e)),
                        t < 2 || e == null)
                            return e;
                        for (var i = 1; i < t; i++)
                            for (var u = arguments[i], f = r(u), a = f.length, c = 0; c < a; c++) {
                                var l = f[c];
                                (!n || e[l] === void 0) && (e[l] = u[l])
                            }
                        return e
                    }
                }
                var Qr = nr(z)
                    , G = nr(s)
                    , Zr = nr(z, !0);
                function ne() {
                    return function() {}
                }
                function Kr(r) {
                    if (!M(r))
                        return {};
                    if (Ar)
                        return Ar(r);
                    var n = ne();
                    n.prototype = r;
                    var e = new n;
                    return n.prototype = null,
                        e
                }
                function ee(r, n) {
                    var e = Kr(r);
                    return n && G(e, n),
                        e
                }
                function te(r) {
                    return M(r) ? I(r) ? r.slice() : Qr({}, r) : r
                }
                function ue(r, n) {
                    return n(r),
                        r
                }
                function xr(r) {
                    return I(r) ? r : [r]
                }
                o.toPath = xr;
                function q(r) {
                    return o.toPath(r)
                }
                function er(r, n) {
                    for (var e = n.length, t = 0; t < e; t++) {
                        if (r == null)
                            return;
                        r = r[n[t]]
                    }
                    return e ? r : void 0
                }
                function kr(r, n, e) {
                    var t = er(r, q(n));
                    return Ir(t) ? e : t
                }
                function ie(r, n) {
                    n = q(n);
                    for (var e = n.length, t = 0; t < e; t++) {
                        var i = n[t];
                        if (!E(r, i))
                            return !1;
                        r = r[i]
                    }
                    return !!e
                }
                function tr(r) {
                    return r
                }
                function L(r) {
                    return r = G({}, r),
                        function(n) {
                            return Wr(n, r)
                        }
                }
                function ur(r) {
                    return r = q(r),
                        function(n) {
                            return er(n, r)
                        }
                }
                function W(r, n, e) {
                    if (n === void 0)
                        return r;
                    switch (e ?? 3) {
                        case 1:
                            return function(t) {
                                return r.call(n, t)
                            }
                                ;
                        case 3:
                            return function(t, i, u) {
                                return r.call(n, t, i, u)
                            }
                                ;
                        case 4:
                            return function(t, i, u, f) {
                                return r.call(n, t, i, u, f)
                            }
                    }
                    return function() {
                        return r.apply(n, arguments)
                    }
                }
                function br(r, n, e) {
                    return r == null ? tr : h(r) ? W(r, n, e) : M(r) && !I(r) ? L(r) : ur(r)
                }
                function ir(r, n) {
                    return br(r, n, 1 / 0)
                }
                o.iteratee = ir;
                function d(r, n, e) {
                    return o.iteratee !== ir ? o.iteratee(r, n) : br(r, n, e)
                }
                function fe(r, n, e) {
                    n = d(n, e);
                    for (var t = s(r), i = t.length, u = {}, f = 0; f < i; f++) {
                        var a = t[f];
                        u[a] = n(r[a], a, r)
                    }
                    return u
                }
                function jr() {}
                function ae(r) {
                    return r == null ? jr : function(n) {
                        return kr(r, n)
                    }
                }
                function le(r, n, e) {
                    var t = Array(Math.max(0, r));
                    n = W(n, e, 1);
                    for (var i = 0; i < r; i++)
                        t[i] = n(i);
                    return t
                }
                function fr(r, n) {
                    return n == null && (n = r,
                        r = 0),
                    r + Math.floor(Math.random() * (n - r + 1))
                }
                var U = Date.now || function() {
                        return new Date().getTime()
                    }
                ;
                function rn(r) {
                    var n = function(u) {
                        return r[u]
                    }
                        , e = "(?:" + s(r).join("|") + ")"
                        , t = RegExp(e)
                        , i = RegExp(e, "g");
                    return function(u) {
                        return u = u == null ? "" : "" + u,
                            t.test(u) ? u.replace(i, n) : u
                    }
                }
                var nn = {
                    "&": "&amp;",
                    "<": "&lt;",
                    ">": "&gt;",
                    '"': "&quot;",
                    "'": "&#x27;",
                    "`": "&#x60;"
                }
                    , ce = rn(nn)
                    , oe = Hr(nn)
                    , se = rn(oe)
                    , ve = o.templateSettings = {
                    evaluate: /<%([\s\S]+?)%>/g,
                    interpolate: /<%=([\s\S]+?)%>/g,
                    escape: /<%-([\s\S]+?)%>/g
                }
                    , ar = /(.)^/
                    , pe = {
                    "'": "'",
                    "\\": "\\",
                    "\r": "r",
                    "\n": "n",
                    "\u2028": "u2028",
                    "\u2029": "u2029"
                }
                    , he = /\\|'|\r|\n|\u2028|\u2029/g;
                function ge(r) {
                    return "\\" + pe[r]
                }
                var me = /^\s*(\w|\$)+\s*$/;
                function ye(r, n, e) {
                    !n && e && (n = e),
                        n = Zr({}, n, o.templateSettings);
                    var t = RegExp([(n.escape || ar).source, (n.interpolate || ar).source, (n.evaluate || ar).source].join("|") + "|$", "g")
                        , i = 0
                        , u = "__p+='";
                    r.replace(t, function(l, v, g, Mn, In) {
                        return u += r.slice(i, In).replace(he, ge),
                            i = In + l.length,
                            v ? u += `'+
((__t=(` + v + `))==null?'':_.escape(__t))+
'` : g ? u += `'+
((__t=(` + g + `))==null?'':__t)+
'` : Mn && (u += `';
` + Mn + `
__p+='`),
                            l
                    }),
                        u += `';
`;
                    var f = n.variable;
                    if (f) {
                        if (!me.test(f))
                            throw new Error("variable is not a bare identifier: " + f)
                    } else
                        u = `with(obj||{}){
` + u + `}
`,
                            f = "obj";
                    u = `var __t,__p='',__j=Array.prototype.join,print=function(){__p+=__j.call(arguments,'');};
` + u + `return __p;
`;
                    var a;
                    try {
                        a = new Function(f,"_",u)
                    } catch (l) {
                        throw l.source = u,
                            l
                    }
                    var c = function(l) {
                        return a.call(this, l, o)
                    };
                    return c.source = "function(" + f + `){
` + u + "}",
                        c
                }
                function de(r, n, e) {
                    n = q(n);
                    var t = n.length;
                    if (!t)
                        return h(e) ? e.call(r) : e;
                    for (var i = 0; i < t; i++) {
                        var u = r?.[n[i]];
                        u === void 0 && (u = e,
                            i = t),
                            r = h(u) ? u.call(r) : u
                    }
                    return r
                }
                var we = 0;
                function _e(r) {
                    var n = ++we + "";
                    return r ? r + n : n
                }
                function Ae(r) {
                    var n = o(r);
                    return n._chain = !0,
                        n
                }
                function en(r, n, e, t, i) {
                    if (!(t instanceof n))
                        return r.apply(e, i);
                    var u = Kr(r.prototype)
                        , f = r.apply(u, i);
                    return M(f) ? f : u
                }
                var D = m(function(r, n) {
                    var e = D.placeholder
                        , t = function() {
                        for (var i = 0, u = n.length, f = Array(u), a = 0; a < u; a++)
                            f[a] = n[a] === e ? arguments[i++] : n[a];
                        for (; i < arguments.length; )
                            f.push(arguments[i++]);
                        return en(r, t, this, this, f)
                    };
                    return t
                });
                D.placeholder = o;
                var tn = m(function(r, n, e) {
                    if (!h(r))
                        throw new TypeError("Bind must be called on a function");
                    var t = m(function(i) {
                        return en(r, t, n, this, e.concat(i))
                    });
                    return t
                })
                    , w = $r(y);
                function B(r, n, e, t) {
                    if (t = t || [],
                    !n && n !== 0)
                        n = 1 / 0;
                    else if (n <= 0)
                        return t.concat(r);
                    for (var i = t.length, u = 0, f = y(r); u < f; u++) {
                        var a = r[u];
                        if (w(a) && (I(a) || K(a)))
                            if (n > 1)
                                B(a, n - 1, e, t),
                                    i = t.length;
                            else
                                for (var c = 0, l = a.length; c < l; )
                                    t[i++] = a[c++];
                        else
                            e || (t[i++] = a)
                    }
                    return t
                }
                var Oe = m(function(r, n) {
                    n = B(n, !1, !1);
                    var e = n.length;
                    if (e < 1)
                        throw new Error("bindAll must be passed function names");
                    for (; e--; ) {
                        var t = n[e];
                        r[t] = tn(r[t], r)
                    }
                    return r
                });
                function Ee(r, n) {
                    var e = function(t) {
                        var i = e.cache
                            , u = "" + (n ? n.apply(this, arguments) : t);
                        return E(i, u) || (i[u] = r.apply(this, arguments)),
                            i[u]
                    };
                    return e.cache = {},
                        e
                }
                var un = m(function(r, n, e) {
                    return setTimeout(function() {
                        return r.apply(null, e)
                    }, n)
                })
                    , Me = D(un, o, 1);
                function Ie(r, n, e) {
                    var t, i, u, f, a = 0;
                    e || (e = {});
                    var c = function() {
                        a = e.leading === !1 ? 0 : U(),
                            t = null,
                            f = r.apply(i, u),
                        t || (i = u = null)
                    }
                        , l = function() {
                        var v = U();
                        !a && e.leading === !1 && (a = v);
                        var g = n - (v - a);
                        return i = this,
                            u = arguments,
                            g <= 0 || g > n ? (t && (clearTimeout(t),
                                t = null),
                                a = v,
                                f = r.apply(i, u),
                            t || (i = u = null)) : !t && e.trailing !== !1 && (t = setTimeout(c, g)),
                            f
                    };
                    return l.cancel = function() {
                        clearTimeout(t),
                            a = 0,
                            t = i = u = null
                    }
                        ,
                        l
                }
                function Be(r, n, e) {
                    var t, i, u, f, a, c = function() {
                        var v = U() - i;
                        n > v ? t = setTimeout(c, n - v) : (t = null,
                        e || (f = r.apply(a, u)),
                        t || (u = a = null))
                    }, l = m(function(v) {
                        return a = this,
                            u = v,
                            i = U(),
                        t || (t = setTimeout(c, n),
                        e && (f = r.apply(a, u))),
                            f
                    });
                    return l.cancel = function() {
                        clearTimeout(t),
                            t = u = a = null
                    }
                        ,
                        l
                }
                function Ne(r, n) {
                    return D(n, r)
                }
                function lr(r) {
                    return function() {
                        return !r.apply(this, arguments)
                    }
                }
                function Pe() {
                    var r = arguments
                        , n = r.length - 1;
                    return function() {
                        for (var e = n, t = r[n].apply(this, arguments); e--; )
                            t = r[e].call(this, t);
                        return t
                    }
                }
                function Se(r, n) {
                    return function() {
                        if (--r < 1)
                            return n.apply(this, arguments)
                    }
                }
                function fn(r, n) {
                    var e;
                    return function() {
                        return --r > 0 && (e = n.apply(this, arguments)),
                        r <= 1 && (n = null),
                            e
                    }
                }
                var Re = D(fn, 2);
                function an(r, n, e) {
                    n = d(n, e);
                    for (var t = s(r), i, u = 0, f = t.length; u < f; u++)
                        if (i = t[u],
                            n(r[i], i, r))
                            return i
                }
                function ln(r) {
                    return function(n, e, t) {
                        e = d(e, t);
                        for (var i = y(n), u = r > 0 ? 0 : i - 1; u >= 0 && u < i; u += r)
                            if (e(n[u], u, n))
                                return u;
                        return -1
                    }
                }
                var cr = ln(1)
                    , cn = ln(-1);
                function on(r, n, e, t) {
                    e = d(e, t, 1);
                    for (var i = e(n), u = 0, f = y(r); u < f; ) {
                        var a = Math.floor((u + f) / 2);
                        e(r[a]) < i ? u = a + 1 : f = a
                    }
                    return u
                }
                function sn(r, n, e) {
                    return function(t, i, u) {
                        var f = 0
                            , a = y(t);
                        if (typeof u == "number")
                            r > 0 ? f = u >= 0 ? u : Math.max(u + a, f) : a = u >= 0 ? Math.min(u + 1, a) : u + a + 1;
                        else if (e && u && a)
                            return u = e(t, i),
                                t[u] === i ? u : -1;
                        if (i !== i)
                            return u = n(F.call(t, f, a), Vr),
                                u >= 0 ? u + f : -1;
                        for (u = r > 0 ? f : a - 1; u >= 0 && u < a; u += r)
                            if (t[u] === i)
                                return u;
                        return -1
                    }
                }
                var vn = sn(1, cr, on)
                    , Te = sn(-1, cn);
                function or(r, n, e) {
                    var t = w(r) ? cr : an
                        , i = t(r, n, e);
                    if (i !== void 0 && i !== -1)
                        return r[i]
                }
                function De(r, n) {
                    return or(r, L(n))
                }
                function _(r, n, e) {
                    n = W(n, e);
                    var t, i;
                    if (w(r))
                        for (t = 0,
                                 i = r.length; t < i; t++)
                            n(r[t], t, r);
                    else {
                        var u = s(r);
                        for (t = 0,
                                 i = u.length; t < i; t++)
                            n(r[u[t]], u[t], r)
                    }
                    return r
                }
                function N(r, n, e) {
                    n = d(n, e);
                    for (var t = !w(r) && s(r), i = (t || r).length, u = Array(i), f = 0; f < i; f++) {
                        var a = t ? t[f] : f;
                        u[f] = n(r[a], a, r)
                    }
                    return u
                }
                function pn(r) {
                    var n = function(e, t, i, u) {
                        var f = !w(e) && s(e)
                            , a = (f || e).length
                            , c = r > 0 ? 0 : a - 1;
                        for (u || (i = e[f ? f[c] : c],
                            c += r); c >= 0 && c < a; c += r) {
                            var l = f ? f[c] : c;
                            i = t(i, e[l], l, e)
                        }
                        return i
                    };
                    return function(e, t, i, u) {
                        var f = arguments.length >= 3;
                        return n(e, W(t, u, 4), i, f)
                    }
                }
                var sr = pn(1)
                    , hn = pn(-1);
                function V(r, n, e) {
                    var t = [];
                    return n = d(n, e),
                        _(r, function(i, u, f) {
                            n(i, u, f) && t.push(i)
                        }),
                        t
                }
                function Ve(r, n, e) {
                    return V(r, lr(d(n)), e)
                }
                function gn(r, n, e) {
                    n = d(n, e);
                    for (var t = !w(r) && s(r), i = (t || r).length, u = 0; u < i; u++) {
                        var f = t ? t[u] : u;
                        if (!n(r[f], f, r))
                            return !1
                    }
                    return !0
                }
                function mn(r, n, e) {
                    n = d(n, e);
                    for (var t = !w(r) && s(r), i = (t || r).length, u = 0; u < i; u++) {
                        var f = t ? t[u] : u;
                        if (n(r[f], f, r))
                            return !0
                    }
                    return !1
                }
                function A(r, n, e, t) {
                    return w(r) || (r = T(r)),
                    (typeof e != "number" || t) && (e = 0),
                    vn(r, n, e) >= 0
                }
                var Fe = m(function(r, n, e) {
                    var t, i;
                    return h(n) ? i = n : (n = q(n),
                        t = n.slice(0, -1),
                        n = n[n.length - 1]),
                        N(r, function(u) {
                            var f = i;
                            if (!f) {
                                if (t && t.length && (u = er(u, t)),
                                u == null)
                                    return;
                                f = u[n]
                            }
                            return f == null ? f : f.apply(u, e)
                        })
                });
                function vr(r, n) {
                    return N(r, ur(n))
                }
                function $e(r, n) {
                    return V(r, L(n))
                }
                function yn(r, n, e) {
                    var t = -1 / 0, i = -1 / 0, u, f;
                    if (n == null || typeof n == "number" && typeof r[0] != "object" && r != null) {
                        r = w(r) ? r : T(r);
                        for (var a = 0, c = r.length; a < c; a++)
                            u = r[a],
                            u != null && u > t && (t = u)
                    } else
                        n = d(n, e),
                            _(r, function(l, v, g) {
                                f = n(l, v, g),
                                (f > i || f === -1 / 0 && t === -1 / 0) && (t = l,
                                    i = f)
                            });
                    return t
                }
                function ze(r, n, e) {
                    var t = 1 / 0, i = 1 / 0, u, f;
                    if (n == null || typeof n == "number" && typeof r[0] != "object" && r != null) {
                        r = w(r) ? r : T(r);
                        for (var a = 0, c = r.length; a < c; a++)
                            u = r[a],
                            u != null && u < t && (t = u)
                    } else
                        n = d(n, e),
                            _(r, function(l, v, g) {
                                f = n(l, v, g),
                                (f < i || f === 1 / 0 && t === 1 / 0) && (t = l,
                                    i = f)
                            });
                    return t
                }
                var qe = /[^\ud800-\udfff]|[\ud800-\udbff][\udc00-\udfff]|[\ud800-\udfff]/g;
                function dn(r) {
                    return r ? I(r) ? F.call(r) : H(r) ? r.match(qe) : w(r) ? N(r, tr) : T(r) : []
                }
                function wn(r, n, e) {
                    if (n == null || e)
                        return w(r) || (r = T(r)),
                            r[fr(r.length - 1)];
                    var t = dn(r)
                        , i = y(t);
                    n = Math.max(Math.min(n, i), 0);
                    for (var u = i - 1, f = 0; f < n; f++) {
                        var a = fr(f, u)
                            , c = t[f];
                        t[f] = t[a],
                            t[a] = c
                    }
                    return t.slice(0, n)
                }
                function Le(r) {
                    return wn(r, 1 / 0)
                }
                function We(r, n, e) {
                    var t = 0;
                    return n = d(n, e),
                        vr(N(r, function(i, u, f) {
                            return {
                                value: i,
                                index: t++,
                                criteria: n(i, u, f)
                            }
                        }).sort(function(i, u) {
                            var f = i.criteria
                                , a = u.criteria;
                            if (f !== a) {
                                if (f > a || f === void 0)
                                    return 1;
                                if (f < a || a === void 0)
                                    return -1
                            }
                            return i.index - u.index
                        }), "value")
                }
                function J(r, n) {
                    return function(e, t, i) {
                        var u = n ? [[], []] : {};
                        return t = d(t, i),
                            _(e, function(f, a) {
                                var c = t(f, a, e);
                                r(u, f, c)
                            }),
                            u
                    }
                }
                var Ue = J(function(r, n, e) {
                    E(r, e) ? r[e].push(n) : r[e] = [n]
                })
                    , Ce = J(function(r, n, e) {
                    r[e] = n
                })
                    , Xe = J(function(r, n, e) {
                    E(r, e) ? r[e]++ : r[e] = 1
                })
                    , Ge = J(function(r, n, e) {
                    r[e ? 0 : 1].push(n)
                }, !0);
                function Je(r) {
                    return r == null ? 0 : w(r) ? r.length : s(r).length
                }
                function Ye(r, n, e) {
                    return n in e
                }
                var _n = m(function(r, n) {
                    var e = {}
                        , t = n[0];
                    if (r == null)
                        return e;
                    h(t) ? (n.length > 1 && (t = W(t, n[1])),
                        n = z(r)) : (t = Ye,
                        n = B(n, !1, !1),
                        r = Object(r));
                    for (var i = 0, u = n.length; i < u; i++) {
                        var f = n[i]
                            , a = r[f];
                        t(a, f, r) && (e[f] = a)
                    }
                    return e
                })
                    , He = m(function(r, n) {
                    var e = n[0], t;
                    return h(e) ? (e = lr(e),
                    n.length > 1 && (t = n[1])) : (n = N(B(n, !1, !1), String),
                            e = function(i, u) {
                                return !A(n, u)
                            }
                    ),
                        _n(r, e, t)
                });
                function An(r, n, e) {
                    return F.call(r, 0, Math.max(0, r.length - (n == null || e ? 1 : n)))
                }
                function pr(r, n, e) {
                    return r == null || r.length < 1 ? n == null || e ? void 0 : [] : n == null || e ? r[0] : An(r, r.length - n)
                }
                function Y(r, n, e) {
                    return F.call(r, n == null || e ? 1 : n)
                }
                function Qe(r, n, e) {
                    return r == null || r.length < 1 ? n == null || e ? void 0 : [] : n == null || e ? r[r.length - 1] : Y(r, Math.max(0, r.length - n))
                }
                function Ze(r) {
                    return V(r, Boolean)
                }
                function Ke(r, n) {
                    return B(r, n, !1)
                }
                var On = m(function(r, n) {
                    return n = B(n, !0, !0),
                        V(r, function(e) {
                            return !A(n, e)
                        })
                })
                    , xe = m(function(r, n) {
                    return On(r, n)
                });
                function hr(r, n, e, t) {
                    Br(n) || (t = e,
                        e = n,
                        n = !1),
                    e != null && (e = d(e, t));
                    for (var i = [], u = [], f = 0, a = y(r); f < a; f++) {
                        var c = r[f]
                            , l = e ? e(c, f, r) : c;
                        n && !e ? ((!f || u !== l) && i.push(c),
                            u = l) : e ? A(u, l) || (u.push(l),
                            i.push(c)) : A(i, c) || i.push(c)
                    }
                    return i
                }
                var ke = m(function(r) {
                    return hr(B(r, !0, !0))
                });
                function be(r) {
                    for (var n = [], e = arguments.length, t = 0, i = y(r); t < i; t++) {
                        var u = r[t];
                        if (!A(n, u)) {
                            var f;
                            for (f = 1; f < e && A(arguments[f], u); f++)
                                ;
                            f === e && n.push(u)
                        }
                    }
                    return n
                }
                function gr(r) {
                    for (var n = r && yn(r, y).length || 0, e = Array(n), t = 0; t < n; t++)
                        e[t] = vr(r, t);
                    return e
                }
                var je = m(gr);
                function rt(r, n) {
                    for (var e = {}, t = 0, i = y(r); t < i; t++)
                        n ? e[r[t]] = n[t] : e[r[t][0]] = r[t][1];
                    return e
                }
                function nt(r, n, e) {
                    n == null && (n = r || 0,
                        r = 0),
                    e || (e = n < r ? -1 : 1);
                    for (var t = Math.max(Math.ceil((n - r) / e), 0), i = Array(t), u = 0; u < t; u++,
                        r += e)
                        i[u] = r;
                    return i
                }
                function et(r, n) {
                    if (n == null || n < 1)
                        return [];
                    for (var e = [], t = 0, i = r.length; t < i; )
                        e.push(F.call(r, t, t += n));
                    return e
                }
                function mr(r, n) {
                    return r._chain ? o(n).chain() : n
                }
                function En(r) {
                    return _(rr(r), function(n) {
                        var e = o[n] = r[n];
                        o.prototype[n] = function() {
                            var t = [this._wrapped];
                            return Bn.apply(t, arguments),
                                mr(this, e.apply(o, t))
                        }
                    }),
                        o
                }
                _(["pop", "push", "reverse", "shift", "sort", "splice", "unshift"], function(r) {
                    var n = S[r];
                    o.prototype[r] = function() {
                        var e = this._wrapped;
                        return e != null && (n.apply(e, arguments),
                        (r === "shift" || r === "splice") && e.length === 0 && delete e[0]),
                            mr(this, e)
                    }
                }),
                    _(["concat", "join", "slice"], function(r) {
                        var n = S[r];
                        o.prototype[r] = function() {
                            var e = this._wrapped;
                            return e != null && (e = n.apply(e, arguments)),
                                mr(this, e)
                        }
                    });
                var tt = {
                    __proto__: null,
                    VERSION: O,
                    restArguments: m,
                    isObject: M,
                    isNull: Vn,
                    isUndefined: Ir,
                    isBoolean: Br,
                    isElement: Fn,
                    isString: H,
                    isNumber: Nr,
                    isDate: $n,
                    isRegExp: zn,
                    isError: qn,
                    isSymbol: Pr,
                    isArrayBuffer: Sr,
                    isDataView: C,
                    isArray: I,
                    isFunction: h,
                    isArguments: K,
                    isFinite: Cn,
                    isNaN: Vr,
                    isTypedArray: qr,
                    isEmpty: Hn,
                    isMatch: Wr,
                    isEqual: Qn,
                    isMap: xn,
                    isWeakMap: kn,
                    isSet: bn,
                    isWeakSet: jn,
                    keys: s,
                    allKeys: z,
                    values: T,
                    pairs: re,
                    invert: Hr,
                    functions: rr,
                    methods: rr,
                    extend: Qr,
                    extendOwn: G,
                    assign: G,
                    defaults: Zr,
                    create: ee,
                    clone: te,
                    tap: ue,
                    get: kr,
                    has: ie,
                    mapObject: fe,
                    identity: tr,
                    constant: Fr,
                    noop: jr,
                    toPath: xr,
                    property: ur,
                    propertyOf: ae,
                    matcher: L,
                    matches: L,
                    times: le,
                    random: fr,
                    now: U,
                    escape: ce,
                    unescape: se,
                    templateSettings: ve,
                    template: ye,
                    result: de,
                    uniqueId: _e,
                    chain: Ae,
                    iteratee: ir,
                    partial: D,
                    bind: tn,
                    bindAll: Oe,
                    memoize: Ee,
                    delay: un,
                    defer: Me,
                    throttle: Ie,
                    debounce: Be,
                    wrap: Ne,
                    negate: lr,
                    compose: Pe,
                    after: Se,
                    before: fn,
                    once: Re,
                    findKey: an,
                    findIndex: cr,
                    findLastIndex: cn,
                    sortedIndex: on,
                    indexOf: vn,
                    lastIndexOf: Te,
                    find: or,
                    detect: or,
                    findWhere: De,
                    each: _,
                    forEach: _,
                    map: N,
                    collect: N,
                    reduce: sr,
                    foldl: sr,
                    inject: sr,
                    reduceRight: hn,
                    foldr: hn,
                    filter: V,
                    select: V,
                    reject: Ve,
                    every: gn,
                    all: gn,
                    some: mn,
                    any: mn,
                    contains: A,
                    includes: A,
                    include: A,
                    invoke: Fe,
                    pluck: vr,
                    where: $e,
                    max: yn,
                    min: ze,
                    shuffle: Le,
                    sample: wn,
                    sortBy: We,
                    groupBy: Ue,
                    indexBy: Ce,
                    countBy: Xe,
                    partition: Ge,
                    toArray: dn,
                    size: Je,
                    pick: _n,
                    omit: He,
                    first: pr,
                    head: pr,
                    take: pr,
                    initial: An,
                    last: Qe,
                    rest: Y,
                    tail: Y,
                    drop: Y,
                    compact: Ze,
                    flatten: Ke,
                    without: xe,
                    uniq: hr,
                    unique: hr,
                    union: ke,
                    intersection: be,
                    difference: On,
                    unzip: gr,
                    transpose: gr,
                    zip: je,
                    object: rt,
                    range: nt,
                    chunk: et,
                    mixin: En,
                    default: o
                }
                    , yr = En(tt);
                return yr._ = yr,
                    yr
            });
        }
    }["(module underscore/underscore.js)"],
    "ep_etherpad-lite/static/js/security.js": {
        "(module ep_etherpad-lite/static/js/security.js)": function(require, exports, module) {
            "use strict";
            module.exports = require("security");
        }
    }["(module ep_etherpad-lite/static/js/security.js)"],
    "security.js": null,
    "ep_etherpad-lite/static/js/pluginfw/client_plugins.js": {
        "(module ep_etherpad-lite/static/js/pluginfw/client_plugins.js)": function(require, exports, module) {
            "use strict";
            const pluginUtils = require("./shared")
                , defs = require("./plugin_defs");
            exports.baseURL = "",
                exports.ensure = e=>defs.loaded ? e() : exports.update(e),
                exports.update = e=>{
                    const t = ()=>setTimeout(e, 0);
                    jQuery.getJSON(`${exports.baseURL}pluginfw/plugin-definitions.json?v=${clientVars.randomVersionString}`).done(s=>{
                            defs.plugins = s.plugins,
                                defs.parts = s.parts,
                                defs.hooks = pluginUtils.extractHooks(defs.parts, "client_hooks"),
                                defs.loaded = !0,
                                t()
                        }
                    ).fail(s=>{
                            console.error(`Failed to load plugin-definitions: ${s}`),
                                t()
                        }
                    )
                }
            ;
            const adoptPluginsFromAncestorsOf = e=>{
                    let t = null;
                    try {
                        for (; e = e.parent; )
                            if (typeof e.require < "u") {
                                t = e.require;
                                break
                            }
                    } catch (r) {
                        console.error(r)
                    }
                    if (!t)
                        throw new Error("Parent plugins could not be found.");
                    const s = t("ep_etherpad-lite/static/js/pluginfw/plugin_defs");
                    defs.hooks = s.hooks,
                        defs.loaded = s.loaded,
                        defs.parts = s.parts,
                        defs.plugins = s.plugins;
                    const o = t("ep_etherpad-lite/static/js/pluginfw/client_plugins");
                    exports.baseURL = o.baseURL,
                        exports.ensure = o.ensure,
                        exports.update = o.update
                }
            ;
            exports.adoptPluginsFromAncestorsOf = adoptPluginsFromAncestorsOf;
        }
    }["(module ep_etherpad-lite/static/js/pluginfw/client_plugins.js)"],
    "ep_etherpad-lite/static/js/pluginfw/plugin_defs.js": {
        "(module ep_etherpad-lite/static/js/pluginfw/plugin_defs.js)": function(require, exports, module) {
            "use strict";
            exports.hooks = {},
                exports.loaded = !1,
                exports.parts = [],
                exports.plugins = {};
        }
    }["(module ep_etherpad-lite/static/js/pluginfw/plugin_defs.js)"],
    "ep_etherpad-lite/static/js/pluginfw/shared.js": {
        "(module ep_etherpad-lite/static/js/pluginfw/shared.js)": function(require, exports, module) {
            "use strict";
            const defs = require("./plugin_defs")
                , disabledHookReasons = {
                    hooks: {
                        indexCustomInlineScripts: "The hook makes it impossible to use a Content Security Policy that prohibits inline code. Permitting inline code makes XSS vulnerabilities more likely"
                    }
                }
                , loadFn = (s,t)=>{
                    let i;
                    const e = s.split(":");
                    e[0].length === 1 ? (e.length === 3 && (i = e.pop()),
                        s = e.join(":")) : (s = e[0],
                        i = e[1]);
                    let n = require(s);
                    i = i || t;
                    for (const o of i.split("."))
                        n = n[o];
                    return n
                }
                , extractHooks = (s,t,i)=>{
                    const e = {};
                    for (const n of s)
                        for (const [o,c] of Object.entries(n[t] || {})) {
                            const r = i ? i(n, c, o) : c
                                , a = (disabledHookReasons[t] || {})[o];
                            if (a) {
                                console.error(`Hook ${t}/${o} is disabled. Reason: ${a}`),
                                    console.error(`The hook function ${r} from plugin ${n.plugin} will never be called, which may cause the plugin to fail`),
                                    console.error(`Please update the ${n.plugin} plugin to not use the ${o} hook`);
                                return
                            }
                            let l;
                            try {
                                if (l = loadFn(r, o),
                                    !l)
                                    throw new Error("Not a function")
                            } catch (u) {
                                console.error(`Failed to load hook function "${r}" for plugin "${n.plugin}" part "${n.name}" hook set "${t}" hook "${o}": ${u.stack || u}`)
                            }
                            l && (e[o] == null && (e[o] = []),
                                e[o].push({
                                    hook_name: o,
                                    hook_fn: l,
                                    hook_fn_name: r,
                                    part: n
                                }))
                        }
                    return e
                }
            ;
            exports.extractHooks = extractHooks,
                exports.clientPluginNames = ()=>{
                    const s = defs.parts.filter(t=>Object.prototype.hasOwnProperty.call(t, "client_hooks")).map(t=>`plugin-${t.plugin}`);
                    return [...new Set(s)]
                }
            ;
        }
    }["(module ep_etherpad-lite/static/js/pluginfw/shared.js)"],
    "ep_etherpad-lite/static/js/pluginfw/hooks.js": {
        "(module ep_etherpad-lite/static/js/pluginfw/hooks.js)": function(require, exports, module) {
            "use strict";
            const pluginDefs = require("./plugin_defs");
            exports.deprecationNotices = {};
            const deprecationWarned = {}
                , checkDeprecation = e=>{
                    const n = exports.deprecationNotices[e.hook_name];
                    n != null && (deprecationWarned[e.hook_fn_name] || (console.warn(`${e.hook_name} hook used by the ${e.part.plugin} plugin (${e.hook_fn_name}) is deprecated: ${n}`),
                        deprecationWarned[e.hook_fn_name] = !0))
                }
                , attachCallback = (e,n)=>e.then(t=>n(null, t), t=>n(t || new Error(t)))
                , normalizeValue = e=>e === void 0 ? [] : Array.isArray(e) ? e : [e]
                , flatten1 = e=>e.reduce((n,t)=>n.concat(t), [])
                , callHookFnSync = (e,n)=>{
                    checkDeprecation(e);
                    let t, r;
                    const o = (a,l,i)=>{
                            r = null;
                            const c = a == null ? "resolved" : "rejected";
                            if (t != null) {
                                const h = a == null ? "resolve" : "reject"
                                    , f = `DOUBLE SETTLE BUG IN HOOK FUNCTION (plugin: ${e.part.plugin}, function name: ${e.hook_fn_name}, hook: ${e.hook_name}): Attempt to ${h} via ${i} but it already ${t.state} via ${t.how}. Ignoring this attempt to ${h}.`;
                                if (console.error(f),
                                c !== t.state || (a == null ? l !== t.val : a !== t.err))
                                    throw r = new Error(f),
                                        r;
                                return
                            }
                            t = {
                                state: c,
                                err: a,
                                val: l,
                                how: i
                            },
                            l && typeof l.then == "function" && console.error(`PROHIBITED PROMISE BUG IN HOOK FUNCTION (plugin: ${e.part.plugin}, function name: ${e.hook_fn_name}, hook: ${e.hook_name}): The hook function provided a "thenable" (e.g., a Promise) which is prohibited because the hook expects to get the value synchronously.`)
                        }
                        , s = a=>{
                            o(null, a, "callback")
                        }
                    ;
                    let u;
                    try {
                        u = e.hook_fn(e.hook_name, n, s)
                    } catch (a) {
                        if (a === r)
                            throw a;
                        try {
                            o(a, null, "thrown exception")
                        } catch (l) {
                            Promise.resolve().then(()=>{
                                    throw l
                                }
                            )
                        }
                        throw a
                    }
                    if (u === void 0) {
                        if (t != null)
                            return t.val;
                        e.hook_fn.length >= 3 && console.error(`UNSETTLED FUNCTION BUG IN HOOK FUNCTION (plugin: ${e.part.plugin}, function name: ${e.hook_fn_name}, hook: ${e.hook_name}): The hook function neither called the callback nor returned a non-undefined value. This is prohibited because it will result in freezes when a future version of Etherpad updates the hook to support asynchronous behavior.`)
                    }
                    return o(null, u, "returned value"),
                        t.val
                }
            ;
            exports.callAll = (e,n)=>{
                n == null && (n = {});
                const t = pluginDefs.hooks[e] || [];
                return flatten1(t.map(r=>normalizeValue(callHookFnSync(r, n))))
            }
            ;
            const callHookFnAsync = async(e,n)=>(checkDeprecation(e),
                await new Promise((t,r)=>{
                        let o;
                        const s = (l,i,c)=>{
                                const h = l == null ? "resolved" : "rejected";
                                if (o != null) {
                                    const f = l == null ? "resolve" : "reject"
                                        , p = `DOUBLE SETTLE BUG IN HOOK FUNCTION (plugin: ${e.part.plugin}, function name: ${e.hook_fn_name}, hook: ${e.hook_name}): Attempt to ${f} via ${c} but it already ${o.state} via ${o.how}. Ignoring this attempt to ${f}.`;
                                    if (console.error(p),
                                    h !== o.state || (l == null ? i !== o.val : l !== o.err))
                                        throw new Error(p);
                                    return
                                }
                                o = {
                                    state: h,
                                    err: l,
                                    val: i,
                                    how: c
                                },
                                    l == null ? t(i) : r(l)
                            }
                            , u = l=>{
                                Promise.resolve(l).then(i=>s(null, i, "callback"), i=>s(i, null, "rejected Promise passed to callback"))
                            }
                        ;
                        let a;
                        try {
                            a = e.hook_fn(e.hook_name, n, u)
                        } catch (l) {
                            try {
                                s(l, null, "thrown exception")
                            } catch (i) {
                                Promise.resolve().then(()=>{
                                        throw i
                                    }
                                )
                            }
                            throw l
                        }
                        a === void 0 && e.hook_fn.length >= 3 || Promise.resolve(a).then(l=>s(null, l, "returned value"), l=>s(l, null, "Promise rejection"))
                    }
                ));
            exports.aCallAll = async(e,n,t=null)=>{
                if (t != null)
                    return await attachCallback(exports.aCallAll(e, n), t);
                n == null && (n = {});
                const r = pluginDefs.hooks[e] || []
                    , o = await Promise.all(r.map(async s=>normalizeValue(await callHookFnAsync(s, n))));
                return flatten1(o)
            }
                ,
                exports.callAllSerial = async(e,n)=>{
                    n == null && (n = {});
                    const t = pluginDefs.hooks[e] || []
                        , r = [];
                    for (const o of t)
                        r.push(normalizeValue(await callHookFnAsync(o, n)));
                    return flatten1(r)
                }
                ,
                exports.callFirst = (e,n)=>{
                    n == null && (n = {});
                    const t = o=>o.length
                        , r = pluginDefs.hooks[e] || [];
                    for (const o of r) {
                        const s = normalizeValue(callHookFnSync(o, n));
                        if (t(s))
                            return s
                    }
                    return []
                }
                ,
                exports.aCallFirst = async(e,n,t=null,r=null)=>{
                    if (t != null)
                        return await attachCallback(exports.aCallFirst(e, n, null, r), t);
                    n == null && (n = {}),
                    r == null && (r = s=>s.length);
                    const o = pluginDefs.hooks[e] || [];
                    for (const s of o) {
                        const u = normalizeValue(await callHookFnAsync(s, n));
                        if (r(u))
                            return u
                    }
                    return []
                }
                ,
                exports.exportedForTestingOnly = {
                    callHookFnAsync,
                    callHookFnSync,
                    deprecationWarned
                };
        }
    }["(module ep_etherpad-lite/static/js/pluginfw/hooks.js)"],
    "ep_etherpad-lite/static/js/ace2_common": null,
    "ep_etherpad-lite/static/js/vendors/browser": null,
    "ep_etherpad-lite/static/js/vendors/jquery": null,
    "ep_etherpad-lite/static/js/rjquery": null,
    "async": null,
    "ep_etherpad-lite/static/js/underscore": null,
    "underscore": null,
    "underscore/underscore": null,
    "ep_etherpad-lite/static/js/security": null,
    "security": null,
    "ep_etherpad-lite/static/js/pluginfw/client_plugins": null,
    "ep_etherpad-lite/static/js/pluginfw/plugin_defs": null,
    "ep_etherpad-lite/static/js/pluginfw/shared": null,
    "ep_etherpad-lite/static/js/pluginfw/hooks": null,
    "ep_etherpad-lite/static/js/ace2_common/index.js": null,
    "ep_etherpad-lite/static/js/vendors/browser/index.js": null,
    "ep_etherpad-lite/static/js/vendors/jquery/index.js": null,
    "ep_etherpad-lite/static/js/rjquery/index.js": null,
    "async/index.js": {
        "(module async/index.js)": function(require, exports, module) {
            "use strict";
            Object.defineProperty(exports, "__esModule", {
                value: !0
            }),
                exports.doDuring = exports.during = exports.wrapSync = void 0,
                exports.selectSeries = exports.selectLimit = exports.select = exports.foldr = exports.foldl = exports.inject = exports.forEachOfLimit = exports.forEachOfSeries = exports.forEachOf = exports.forEachLimit = exports.forEachSeries = exports.forEach = exports.flatMapSeries = exports.flatMapLimit = exports.flatMap = exports.findSeries = exports.findLimit = exports.find = exports.anySeries = exports.anyLimit = exports.any = exports.allSeries = exports.allLimit = exports.all = exports.whilst = exports.waterfall = exports.until = exports.unmemoize = exports.tryEach = exports.transform = exports.timesSeries = exports.timesLimit = exports.times = exports.timeout = exports.sortBy = exports.someSeries = exports.someLimit = exports.some = exports.setImmediate = exports.series = exports.seq = exports.retryable = exports.retry = exports.rejectSeries = exports.rejectLimit = exports.reject = exports.reflectAll = exports.reflect = exports.reduceRight = exports.reduce = exports.race = exports.queue = exports.priorityQueue = exports.parallelLimit = exports.parallel = exports.nextTick = exports.memoize = exports.mapValuesSeries = exports.mapValuesLimit = exports.mapValues = exports.mapSeries = exports.mapLimit = exports.map = exports.log = exports.groupBySeries = exports.groupByLimit = exports.groupBy = exports.forever = exports.filterSeries = exports.filterLimit = exports.filter = exports.everySeries = exports.everyLimit = exports.every = exports.ensureAsync = exports.eachSeries = exports.eachOfSeries = exports.eachOfLimit = exports.eachOf = exports.eachLimit = exports.each = exports.doWhilst = exports.doUntil = exports.dir = exports.detectSeries = exports.detectLimit = exports.detect = exports.constant = exports.concatSeries = exports.concatLimit = exports.concat = exports.compose = exports.cargoQueue = exports.cargo = exports.autoInject = exports.auto = exports.asyncify = exports.applyEachSeries = exports.applyEach = exports.apply = void 0;
            var _apply = require("./apply")
                , _apply2 = _interopRequireDefault(_apply)
                , _applyEach = require("./applyEach")
                , _applyEach2 = _interopRequireDefault(_applyEach)
                , _applyEachSeries = require("./applyEachSeries")
                , _applyEachSeries2 = _interopRequireDefault(_applyEachSeries)
                , _asyncify = require("./asyncify")
                , _asyncify2 = _interopRequireDefault(_asyncify)
                , _auto = require("./auto")
                , _auto2 = _interopRequireDefault(_auto)
                , _autoInject = require("./autoInject")
                , _autoInject2 = _interopRequireDefault(_autoInject)
                , _cargo = require("./cargo")
                , _cargo2 = _interopRequireDefault(_cargo)
                , _cargoQueue = require("./cargoQueue")
                , _cargoQueue2 = _interopRequireDefault(_cargoQueue)
                , _compose = require("./compose")
                , _compose2 = _interopRequireDefault(_compose)
                , _concat = require("./concat")
                , _concat2 = _interopRequireDefault(_concat)
                , _concatLimit = require("./concatLimit")
                , _concatLimit2 = _interopRequireDefault(_concatLimit)
                , _concatSeries = require("./concatSeries")
                , _concatSeries2 = _interopRequireDefault(_concatSeries)
                , _constant = require("./constant")
                , _constant2 = _interopRequireDefault(_constant)
                , _detect = require("./detect")
                , _detect2 = _interopRequireDefault(_detect)
                , _detectLimit = require("./detectLimit")
                , _detectLimit2 = _interopRequireDefault(_detectLimit)
                , _detectSeries = require("./detectSeries")
                , _detectSeries2 = _interopRequireDefault(_detectSeries)
                , _dir = require("./dir")
                , _dir2 = _interopRequireDefault(_dir)
                , _doUntil = require("./doUntil")
                , _doUntil2 = _interopRequireDefault(_doUntil)
                , _doWhilst = require("./doWhilst")
                , _doWhilst2 = _interopRequireDefault(_doWhilst)
                , _each = require("./each")
                , _each2 = _interopRequireDefault(_each)
                , _eachLimit = require("./eachLimit")
                , _eachLimit2 = _interopRequireDefault(_eachLimit)
                , _eachOf = require("./eachOf")
                , _eachOf2 = _interopRequireDefault(_eachOf)
                , _eachOfLimit = require("./eachOfLimit")
                , _eachOfLimit2 = _interopRequireDefault(_eachOfLimit)
                , _eachOfSeries = require("./eachOfSeries")
                , _eachOfSeries2 = _interopRequireDefault(_eachOfSeries)
                , _eachSeries = require("./eachSeries")
                , _eachSeries2 = _interopRequireDefault(_eachSeries)
                , _ensureAsync = require("./ensureAsync")
                , _ensureAsync2 = _interopRequireDefault(_ensureAsync)
                , _every = require("./every")
                , _every2 = _interopRequireDefault(_every)
                , _everyLimit = require("./everyLimit")
                , _everyLimit2 = _interopRequireDefault(_everyLimit)
                , _everySeries = require("./everySeries")
                , _everySeries2 = _interopRequireDefault(_everySeries)
                , _filter = require("./filter")
                , _filter2 = _interopRequireDefault(_filter)
                , _filterLimit = require("./filterLimit")
                , _filterLimit2 = _interopRequireDefault(_filterLimit)
                , _filterSeries = require("./filterSeries")
                , _filterSeries2 = _interopRequireDefault(_filterSeries)
                , _forever = require("./forever")
                , _forever2 = _interopRequireDefault(_forever)
                , _groupBy = require("./groupBy")
                , _groupBy2 = _interopRequireDefault(_groupBy)
                , _groupByLimit = require("./groupByLimit")
                , _groupByLimit2 = _interopRequireDefault(_groupByLimit)
                , _groupBySeries = require("./groupBySeries")
                , _groupBySeries2 = _interopRequireDefault(_groupBySeries)
                , _log = require("./log")
                , _log2 = _interopRequireDefault(_log)
                , _map = require("./map")
                , _map2 = _interopRequireDefault(_map)
                , _mapLimit = require("./mapLimit")
                , _mapLimit2 = _interopRequireDefault(_mapLimit)
                , _mapSeries = require("./mapSeries")
                , _mapSeries2 = _interopRequireDefault(_mapSeries)
                , _mapValues = require("./mapValues")
                , _mapValues2 = _interopRequireDefault(_mapValues)
                , _mapValuesLimit = require("./mapValuesLimit")
                , _mapValuesLimit2 = _interopRequireDefault(_mapValuesLimit)
                , _mapValuesSeries = require("./mapValuesSeries")
                , _mapValuesSeries2 = _interopRequireDefault(_mapValuesSeries)
                , _memoize = require("./memoize")
                , _memoize2 = _interopRequireDefault(_memoize)
                , _nextTick = require("./nextTick")
                , _nextTick2 = _interopRequireDefault(_nextTick)
                , _parallel = require("./parallel")
                , _parallel2 = _interopRequireDefault(_parallel)
                , _parallelLimit = require("./parallelLimit")
                , _parallelLimit2 = _interopRequireDefault(_parallelLimit)
                , _priorityQueue = require("./priorityQueue")
                , _priorityQueue2 = _interopRequireDefault(_priorityQueue)
                , _queue = require("./queue")
                , _queue2 = _interopRequireDefault(_queue)
                , _race = require("./race")
                , _race2 = _interopRequireDefault(_race)
                , _reduce = require("./reduce")
                , _reduce2 = _interopRequireDefault(_reduce)
                , _reduceRight = require("./reduceRight")
                , _reduceRight2 = _interopRequireDefault(_reduceRight)
                , _reflect = require("./reflect")
                , _reflect2 = _interopRequireDefault(_reflect)
                , _reflectAll = require("./reflectAll")
                , _reflectAll2 = _interopRequireDefault(_reflectAll)
                , _reject = require("./reject")
                , _reject2 = _interopRequireDefault(_reject)
                , _rejectLimit = require("./rejectLimit")
                , _rejectLimit2 = _interopRequireDefault(_rejectLimit)
                , _rejectSeries = require("./rejectSeries")
                , _rejectSeries2 = _interopRequireDefault(_rejectSeries)
                , _retry = require("./retry")
                , _retry2 = _interopRequireDefault(_retry)
                , _retryable = require("./retryable")
                , _retryable2 = _interopRequireDefault(_retryable)
                , _seq = require("./seq")
                , _seq2 = _interopRequireDefault(_seq)
                , _series = require("./series")
                , _series2 = _interopRequireDefault(_series)
                , _setImmediate = require("./setImmediate")
                , _setImmediate2 = _interopRequireDefault(_setImmediate)
                , _some = require("./some")
                , _some2 = _interopRequireDefault(_some)
                , _someLimit = require("./someLimit")
                , _someLimit2 = _interopRequireDefault(_someLimit)
                , _someSeries = require("./someSeries")
                , _someSeries2 = _interopRequireDefault(_someSeries)
                , _sortBy = require("./sortBy")
                , _sortBy2 = _interopRequireDefault(_sortBy)
                , _timeout = require("./timeout")
                , _timeout2 = _interopRequireDefault(_timeout)
                , _times = require("./times")
                , _times2 = _interopRequireDefault(_times)
                , _timesLimit = require("./timesLimit")
                , _timesLimit2 = _interopRequireDefault(_timesLimit)
                , _timesSeries = require("./timesSeries")
                , _timesSeries2 = _interopRequireDefault(_timesSeries)
                , _transform = require("./transform")
                , _transform2 = _interopRequireDefault(_transform)
                , _tryEach = require("./tryEach")
                , _tryEach2 = _interopRequireDefault(_tryEach)
                , _unmemoize = require("./unmemoize")
                , _unmemoize2 = _interopRequireDefault(_unmemoize)
                , _until = require("./until")
                , _until2 = _interopRequireDefault(_until)
                , _waterfall = require("./waterfall")
                , _waterfall2 = _interopRequireDefault(_waterfall)
                , _whilst = require("./whilst")
                , _whilst2 = _interopRequireDefault(_whilst);
            function _interopRequireDefault(e) {
                return e && e.__esModule ? e : {
                    default: e
                }
            }
            exports.default = {
                apply: _apply2.default,
                applyEach: _applyEach2.default,
                applyEachSeries: _applyEachSeries2.default,
                asyncify: _asyncify2.default,
                auto: _auto2.default,
                autoInject: _autoInject2.default,
                cargo: _cargo2.default,
                cargoQueue: _cargoQueue2.default,
                compose: _compose2.default,
                concat: _concat2.default,
                concatLimit: _concatLimit2.default,
                concatSeries: _concatSeries2.default,
                constant: _constant2.default,
                detect: _detect2.default,
                detectLimit: _detectLimit2.default,
                detectSeries: _detectSeries2.default,
                dir: _dir2.default,
                doUntil: _doUntil2.default,
                doWhilst: _doWhilst2.default,
                each: _each2.default,
                eachLimit: _eachLimit2.default,
                eachOf: _eachOf2.default,
                eachOfLimit: _eachOfLimit2.default,
                eachOfSeries: _eachOfSeries2.default,
                eachSeries: _eachSeries2.default,
                ensureAsync: _ensureAsync2.default,
                every: _every2.default,
                everyLimit: _everyLimit2.default,
                everySeries: _everySeries2.default,
                filter: _filter2.default,
                filterLimit: _filterLimit2.default,
                filterSeries: _filterSeries2.default,
                forever: _forever2.default,
                groupBy: _groupBy2.default,
                groupByLimit: _groupByLimit2.default,
                groupBySeries: _groupBySeries2.default,
                log: _log2.default,
                map: _map2.default,
                mapLimit: _mapLimit2.default,
                mapSeries: _mapSeries2.default,
                mapValues: _mapValues2.default,
                mapValuesLimit: _mapValuesLimit2.default,
                mapValuesSeries: _mapValuesSeries2.default,
                memoize: _memoize2.default,
                nextTick: _nextTick2.default,
                parallel: _parallel2.default,
                parallelLimit: _parallelLimit2.default,
                priorityQueue: _priorityQueue2.default,
                queue: _queue2.default,
                race: _race2.default,
                reduce: _reduce2.default,
                reduceRight: _reduceRight2.default,
                reflect: _reflect2.default,
                reflectAll: _reflectAll2.default,
                reject: _reject2.default,
                rejectLimit: _rejectLimit2.default,
                rejectSeries: _rejectSeries2.default,
                retry: _retry2.default,
                retryable: _retryable2.default,
                seq: _seq2.default,
                series: _series2.default,
                setImmediate: _setImmediate2.default,
                some: _some2.default,
                someLimit: _someLimit2.default,
                someSeries: _someSeries2.default,
                sortBy: _sortBy2.default,
                timeout: _timeout2.default,
                times: _times2.default,
                timesLimit: _timesLimit2.default,
                timesSeries: _timesSeries2.default,
                transform: _transform2.default,
                tryEach: _tryEach2.default,
                unmemoize: _unmemoize2.default,
                until: _until2.default,
                waterfall: _waterfall2.default,
                whilst: _whilst2.default,
                all: _every2.default,
                allLimit: _everyLimit2.default,
                allSeries: _everySeries2.default,
                any: _some2.default,
                anyLimit: _someLimit2.default,
                anySeries: _someSeries2.default,
                find: _detect2.default,
                findLimit: _detectLimit2.default,
                findSeries: _detectSeries2.default,
                flatMap: _concat2.default,
                flatMapLimit: _concatLimit2.default,
                flatMapSeries: _concatSeries2.default,
                forEach: _each2.default,
                forEachSeries: _eachSeries2.default,
                forEachLimit: _eachLimit2.default,
                forEachOf: _eachOf2.default,
                forEachOfSeries: _eachOfSeries2.default,
                forEachOfLimit: _eachOfLimit2.default,
                inject: _reduce2.default,
                foldl: _reduce2.default,
                foldr: _reduceRight2.default,
                select: _filter2.default,
                selectLimit: _filterLimit2.default,
                selectSeries: _filterSeries2.default,
                wrapSync: _asyncify2.default,
                during: _whilst2.default,
                doDuring: _doWhilst2.default
            },
                exports.apply = _apply2.default,
                exports.applyEach = _applyEach2.default,
                exports.applyEachSeries = _applyEachSeries2.default,
                exports.asyncify = _asyncify2.default,
                exports.auto = _auto2.default,
                exports.autoInject = _autoInject2.default,
                exports.cargo = _cargo2.default,
                exports.cargoQueue = _cargoQueue2.default,
                exports.compose = _compose2.default,
                exports.concat = _concat2.default,
                exports.concatLimit = _concatLimit2.default,
                exports.concatSeries = _concatSeries2.default,
                exports.constant = _constant2.default,
                exports.detect = _detect2.default,
                exports.detectLimit = _detectLimit2.default,
                exports.detectSeries = _detectSeries2.default,
                exports.dir = _dir2.default,
                exports.doUntil = _doUntil2.default,
                exports.doWhilst = _doWhilst2.default,
                exports.each = _each2.default,
                exports.eachLimit = _eachLimit2.default,
                exports.eachOf = _eachOf2.default,
                exports.eachOfLimit = _eachOfLimit2.default,
                exports.eachOfSeries = _eachOfSeries2.default,
                exports.eachSeries = _eachSeries2.default,
                exports.ensureAsync = _ensureAsync2.default,
                exports.every = _every2.default,
                exports.everyLimit = _everyLimit2.default,
                exports.everySeries = _everySeries2.default,
                exports.filter = _filter2.default,
                exports.filterLimit = _filterLimit2.default,
                exports.filterSeries = _filterSeries2.default,
                exports.forever = _forever2.default,
                exports.groupBy = _groupBy2.default,
                exports.groupByLimit = _groupByLimit2.default,
                exports.groupBySeries = _groupBySeries2.default,
                exports.log = _log2.default,
                exports.map = _map2.default,
                exports.mapLimit = _mapLimit2.default,
                exports.mapSeries = _mapSeries2.default,
                exports.mapValues = _mapValues2.default,
                exports.mapValuesLimit = _mapValuesLimit2.default,
                exports.mapValuesSeries = _mapValuesSeries2.default,
                exports.memoize = _memoize2.default,
                exports.nextTick = _nextTick2.default,
                exports.parallel = _parallel2.default,
                exports.parallelLimit = _parallelLimit2.default,
                exports.priorityQueue = _priorityQueue2.default,
                exports.queue = _queue2.default,
                exports.race = _race2.default,
                exports.reduce = _reduce2.default,
                exports.reduceRight = _reduceRight2.default,
                exports.reflect = _reflect2.default,
                exports.reflectAll = _reflectAll2.default,
                exports.reject = _reject2.default,
                exports.rejectLimit = _rejectLimit2.default,
                exports.rejectSeries = _rejectSeries2.default,
                exports.retry = _retry2.default,
                exports.retryable = _retryable2.default,
                exports.seq = _seq2.default,
                exports.series = _series2.default,
                exports.setImmediate = _setImmediate2.default,
                exports.some = _some2.default,
                exports.someLimit = _someLimit2.default,
                exports.someSeries = _someSeries2.default,
                exports.sortBy = _sortBy2.default,
                exports.timeout = _timeout2.default,
                exports.times = _times2.default,
                exports.timesLimit = _timesLimit2.default,
                exports.timesSeries = _timesSeries2.default,
                exports.transform = _transform2.default,
                exports.tryEach = _tryEach2.default,
                exports.unmemoize = _unmemoize2.default,
                exports.until = _until2.default,
                exports.waterfall = _waterfall2.default,
                exports.whilst = _whilst2.default,
                exports.all = _every2.default,
                exports.allLimit = _everyLimit2.default,
                exports.allSeries = _everySeries2.default,
                exports.any = _some2.default,
                exports.anyLimit = _someLimit2.default,
                exports.anySeries = _someSeries2.default,
                exports.find = _detect2.default,
                exports.findLimit = _detectLimit2.default,
                exports.findSeries = _detectSeries2.default,
                exports.flatMap = _concat2.default,
                exports.flatMapLimit = _concatLimit2.default,
                exports.flatMapSeries = _concatSeries2.default,
                exports.forEach = _each2.default,
                exports.forEachSeries = _eachSeries2.default,
                exports.forEachLimit = _eachLimit2.default,
                exports.forEachOf = _eachOf2.default,
                exports.forEachOfSeries = _eachOfSeries2.default,
                exports.forEachOfLimit = _eachOfLimit2.default,
                exports.inject = _reduce2.default,
                exports.foldl = _reduce2.default,
                exports.foldr = _reduceRight2.default,
                exports.select = _filter2.default,
                exports.selectLimit = _filterLimit2.default,
                exports.selectSeries = _filterSeries2.default,
            exports.wrapSync = _asyncify2.default,
            exports.during = _whilst2.default,
            exports.doDuring = _doWhilst2.default;
        }
    }["(module async/index.js)"],
    "ep_etherpad-lite/static/js/underscore/index.js": null,
    "underscore/index.js": null,
    "underscore/underscore/index.js": null,
    "ep_etherpad-lite/static/js/security/index.js": null,
    "security/index.js": {
        "(module security/index.js)": function(require, exports, module) {
            /*!

  Copyright (c) 2011 Chad Weider

  Permission is hereby granted, free of charge, to any person obtaining a copy
  of this software and associated documentation files (the "Software"), to deal
  in the Software without restriction, including without limitation the rights
  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
  copies of the Software, and to permit persons to whom the Software is
  furnished to do so, subject to the following conditions:

  The above copyright notice and this permission notice shall be included in
  all copies or substantial portions of the Software.

  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
  SOFTWARE.

*/
            var HTML_ENTITY_MAP = {
                "&": "&amp;",
                "<": "&lt;",
                ">": "&gt;",
                '"': "&quot;",
                "'": "&#x27;",
                "/": "&#x2F;"
            }
                , HTML_CHARACTERS_EXPRESSION = /[&"'<>\/]/gm;
            function escapeHTML(e) {
                return e && e.replace(HTML_CHARACTERS_EXPRESSION, function(r) {
                    return HTML_ENTITY_MAP[r] || r
                })
            }
            var HTML_ATTRIBUTE_CHARACTERS_EXPRESSION = /[\x00-\x2F\x3A-\x40\x5B-\x60\x7B-\xFF]/gm;
            function escapeHTMLAttribute(e) {
                return e && e.replace(HTML_ATTRIBUTE_CHARACTERS_EXPRESSION, function(r) {
                    return HTML_ENTITY_MAP[r] || "&#x" + ("00" + r.charCodeAt(0).toString(16)).slice(-2) + ";"
                })
            }
            var JAVASCRIPT_CHARACTERS_EXPRESSION = /[\x00-\x2F\x3A-\x40\x5B-\x60\x7B-\xFF\u2028\u2029]/gm;
            function encodeJavaScriptIdentifier(e) {
                return e && e.replace(JAVASCRIPT_CHARACTERS_EXPRESSION, function(r) {
                    return "\\u" + ("0000" + r.charCodeAt(0).toString(16)).slice(-4)
                })
            }
            function encodeJavaScriptString(e) {
                return e && '"' + encodeJavaScriptIdentifier(e) + '"'
            }
            var JSON_STRING_LITERAL_EXPRESSION = /"(?:\\.|[^"])*"/gm;
            function encodeJavaScriptData(e) {
                return JSON.stringify(e).replace(JSON_STRING_LITERAL_EXPRESSION, function(r) {
                    return encodeJavaScriptString(JSON.parse(r))
                })
            }
            var CSS_CHARACTERS_EXPRESSION = /[\x00-\x2F\x3A-\x40\x5B-\x60\x7B-\xFF]/gm;
            function encodeCSSIdentifier(e) {
                return e && e.replace(CSS_CHARACTERS_EXPRESSION, function(r) {
                    return "\\" + ("000000" + r.charCodeAt(0).toString(16)).slice(-6)
                })
            }
            function encodeCSSString(e) {
                return e && '"' + encodeCSSIdentifier(e) + '"'
            }
            exports.escapeHTML = escapeHTML,
                exports.escapeHTMLAttribute = escapeHTMLAttribute,
                exports.encodeJavaScriptIdentifier = encodeJavaScriptIdentifier,
                exports.encodeJavaScriptString = encodeJavaScriptString,
                exports.encodeJavaScriptData = encodeJavaScriptData,
                exports.encodeCSSIdentifier = encodeCSSIdentifier,
                exports.encodeCSSString = encodeCSSString;
        }
    }["(module security/index.js)"],
    "ep_etherpad-lite/static/js/pluginfw/client_plugins/index.js": null,
    "ep_etherpad-lite/static/js/pluginfw/plugin_defs/index.js": null,
    "ep_etherpad-lite/static/js/pluginfw/shared/index.js": null,
    "ep_etherpad-lite/static/js/pluginfw/hooks/index.js": null,
});
