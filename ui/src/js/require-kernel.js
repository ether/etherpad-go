export var require = (()=>{
        "use strict";
        let A = null;
        const g = {}
            , h = {}
            , w = {}
            , u = {}
            , E = [];
        let $ = 0
            , L = 2;
        const f = [];
        let d = !1, R, y, S, _, p;
        const M = 60 * 1e3;
        function v(e) {
            this.name = "CircularDependencyError",
                this.message = e
        }
        v.prototype = Error.prototype;
        function c(e) {
            this.name = "ArgumentError",
                this.message = e
        }
        c.prototype = Error.prototype;
        const m = (e,t)=>Object.prototype.hasOwnProperty.call(e, t)
            , O = (...e)=>f.push(...e)
            , k = ()=>{
                try {
                    for (; f.length; )
                        f.shift()();
                    d = !1
                } finally {
                    d = f.length > 0,
                    f.length && setTimeout(k, 0)
                }
            }
            , x = ()=>{
                !d && f.length > 0 && (R ? (d = !0,
                    setTimeout(k, 0)) : k())
            }
            , B = e=>{
                try {
                    d = !0,
                        e(),
                        d = !1,
                        x()
                } finally {
                    d = !1,
                    f.length && setTimeout(x, 0)
                }
            }
            , b = e=>{
                const t = e.split("/")
                    , n = [];
                let o;
                for (let s = 0, r = t.length; s < r; s++)
                    switch (o = t[s],
                        o) {
                        case "":
                            (s === 0 || s === r - 1) && n.push(o);
                            break;
                        case ".":
                            break;
                        case "..":
                            if (n.length > 1 || n.length === 1 && n[0] !== "" && n[0] !== ".") {
                                n.pop();
                                break
                            }
                        default:
                            n.push(o)
                    }
                return n.join("/")
            }
            , C = (e,t)=>{
                let n = e;
                return e.charAt(0) === "." && (e.charAt(1) === "/" || e.charAt(1) === "." && e.charAt(2) === "/") && (t ? t.charAt(t.length - 1) !== "/" && (t += "/") : t = "",
                    n = t + e),
                    n
            }
            , J = e=>{
                if (!e)
                    throw new c("Invalid root URI.");
                S = e.charAt(e.length - 1) === "/" ? e.slice(0, -1) : e
            }
            , Q = e=>{
                _ = e.charAt(e.length - 1) === "/" ? e : `${e}/`
            }
            , z = e=>{
                if (e = e && e.toString(),
                    !e)
                    p = void 0;
                else {
                    if (e.match(/\//))
                        throw new c("Invalid path component.");
                    p = e
                }
            }
            , Y = (e,t)=>{
                if (e = b(e),
                e.charAt(0) !== "/" && p) {
                    const n = []
                        , o = t.split("/");
                    for (; o.length > 1; ) {
                        o[o.length - 1] === p && o.pop();
                        const s = b(C(`./${p}/${e}`, `${o.join("/")}/`));
                        n.push(s),
                            o.pop()
                    }
                    return n.push(e),
                        n
                } else
                    return [b(C(e, t))]
            }
            , P = e=>{
                const t = e.split("/");
                for (let n = 0, o = t.length; n < o; n++)
                    t[n] = encodeURIComponent(t[n]);
                if (e = t.join("/"),
                e.charAt(0) === "/") {
                    if (!S)
                        throw new Error(`Attempt to retrieve the root module "${e}" but no root URI is defined.`);
                    return S + e
                } else {
                    if (!_)
                        throw new Error(`Attempt to retrieve the library module "${e}" but no libary URI is defined.`);
                    return _ + e
                }
            }
            , Z = e=>({
                [e]: t=>t()
            })[e]
            , U = (e,...t)=>{
                const n = Function.bind(null, ...t);
                return Z(e)(n)
            }
            , ee = e=>{
                if (e = parseInt(e),
                e > 0)
                    L = e,
                        D();
                else
                    throw new c("Value must be a positive integer.")
            }
            , te = e=>{
                y = e
            }
        ;
        let I = null;
        const F = ()=>{
            if (typeof window > "u")
                return null;
            for (let e = window; I == null && ({clientVars: {randomVersionString: I}={}} = e,
            e !== window.top); e = e.parent)
                ;
            return I
        }
            , ne = (e,t,n)=>{
            F() && (e += `&v=${F()}`);
            const o = new XMLHttpRequest
                , s = r=>{
                    r.status === 200 ? n(void 0, r.responseText) : n(!0, void 0)
                }
            ;
            o.open("GET", e, !!t),
                t ? (o.onreadystatechange = r=>{
                    o.readyState === 4 && s(o)
                }
                    ,
                    o.send(null)) : (o.send(null),
                    s(o))
        }
            , K = (e,t)=>{
            const n = y
                , o = (r,i)=>{
                    if (r)
                        q(e, null);
                    else if (n)
                        U(`(bundle ${e})`, i)();
                    else {
                        const a = U(`(module ${e})`, "require", "exports", "module", i);
                        q(e, a)
                    }
                }
            ;
            let s = P(e);
            n && (s += `?callback=${encodeURIComponent(`${y}.define`)}`),
                ne(s, t, o)
        }
            , oe = e=>{
            const t = document.head || document.getElementsByTagName("head")[0] || document.documentElement
                , n = document.createElement("script");
            if (n.async !== void 0 ? n.async = "true" : n.defer = "true",
                n.type = "application/javascript",
                n.src = `${P(e)}?callback=${encodeURIComponent(y)}.define`,
            M < 1 / 0) {
                const o = setTimeout(()=>q(e, null), M);
                u[e].unshift(()=>clearTimeout(o))
            }
            t.insertBefore(n, t.firstChild)
        }
            , se = (e,t)=>{
            m(u, e) ? u[e].push(t) : (u[e] = [t],
                re(e))
        }
            , re = e=>{
            E.push(e),
                D()
        }
            , D = ()=>{
            if (E.length > 0 && $ < L) {
                const e = E.pop();
                $++,
                    u[e].unshift(()=>{
                            $--,
                                D()
                        }
                    ),
                    y && typeof document < "u" && document.readyState && /^loaded|complete$/.test(document.readyState) ? oe(e) : K(e, !0)
            }
        }
            , ie = (e,t)=>{
            K(e, !1),
                t()
        }
            , le = e=>m(g, e)
            , ce = (e,t)=>{
            if (le(e)) {
                const n = g[e];
                t(n)
            } else {
                if (m(w, e))
                    throw new v("Encountered circular dependency.");
                if (T(e))
                    if (h[e] === null)
                        t(null);
                    else {
                        const n = h[e]
                            , o = {
                            id: e,
                            exports: {}
                        }
                            , s = H(e);
                        A || (A = o);
                        try {
                            w[e] = !0,
                                n(s, o.exports, o),
                                g[e] = o,
                                delete w[e],
                                t(o)
                        } finally {
                            delete w[e]
                        }
                    }
                else
                    throw new Error("Attempt to load undefined module.")
            }
        }
            , N = (e,t,n)=>{
            const o = e.endsWith(".js") ? [""] : e.endsWith("/") ? ["index.js"] : [".js", "/index.js", ""]
                , s = o.length
                , r = i=>{
                    if (i < s) {
                        const a = e + o[i]
                            , W = ()=>{
                                ce(a, X=>{
                                        X === null ? r(i + 1) : n(X)
                                    }
                                )
                            }
                        ;
                        T(a) ? W() : t(a, W)
                    } else
                        n(null)
                }
            ;
            r(0)
        }
            , ue = (e,t)=>{
            O(()=>{
                    N(e, se, t)
                }
            )
        }
            , fe = e=>{
            let t;
            const n = R;
            R = !0,
            e === "async" && (e = "async/lib/async"),
            e === "underscore" && (e = "underscore/underscore"),
            e === "unorm" && (e = "unorm/lib/unorm");
            try {
                N(e, ie, o=>{
                        t = o
                    }
                )
            } finally {
                R = n
            }
            return t
        }
            , T = e=>m(h, e)
            , V = (e,t)=>{
            if (typeof e != "string" || !(typeof t == "function" || t === null))
                throw new c("Definition must be a (string, function) pair.");
            T(e) || (h[e] = t)
        }
            , de = e=>{
            if (typeof e != "object")
                throw new c("Mapping must be an object.");
            for (const t in e)
                m(e, t) && V(t, e[t])
        }
            , q = (...e)=>{
            let t;
            if (e.length === 1)
                [t] = e,
                    de(t);
            else if (e.length === 2) {
                const [n,o] = e;
                V(n, o),
                    t = {
                        [n]: o
                    }
            } else
                throw new c(`Expected 1 or 2 arguments, but got ${e.length}.`);
            for (const n in t)
                m(t, n) && m(u, n) && (O(...u[n]),
                    delete u[n]);
            x()
        }
            , G = (e,t,n)=>{
            const o = Y(e, n);
            if (t === void 0) {
                let s;
                for (let r = 0, i = o.length; r < i && !s; r++) {
                    const a = o[r];
                    s = fe(a)
                }
                if (!s)
                    throw new Error(`The module at "${e}" does not exist.`);
                return s.exports
            } else {
                if (typeof t != "function")
                    throw new c("Continuation must be a function.");
                B(()=>{
                        const s = ()=>{
                                const r = o.shift();
                                return ue(r, i=>{
                                        i || o.length === 0 ? t(i && i.exports) : s()
                                    }
                                )
                            }
                        ;
                        s()
                    }
                )
            }
        }
            , me = (...e)=>(l._designatedRequire || G)(...e)
            , j = (e,t,n)=>{
            t = t.toString();
            const o = b(C(t, e));
            return me(o, n, e)
        }
            , ae = (e,t,n)=>{
            if (typeof n != "function")
                throw new c("Final argument must be a continuation.");
            {
                const o = t.map(i=>i.toString())
                    , s = []
                    , r = i=>{
                        s.push(i),
                            t.length > 0 ? j(e, t.shift(), r) : n(...s)
                    }
                ;
                for (let i = 0; i < t.length; i++)
                    j(e, o[i], r)
            }
        }
            , H = e=>{
            e = e.replace(/[^/]+$/, "");
            const t = (...n)=>{
                    if (n.length > 2) {
                        const o = n.pop();
                        return ae(e, n, o)
                    } else {
                        const [o,s] = n;
                        return j(e, o, s)
                    }
                }
            ;
            return t.main = A,
                t
        }
            , l = H("/");
        return l._modules = g,
            l._definitions = h,
            l._designatedRequire = G,
            l.define = q,
            l.setRequestMaximum = ee,
            l.setGlobalKeyPath = te,
            l.setRootURI = J,
            l.setLibraryURI = Q,
            l.setLibraryLookupComponent = z,
            l
    }
)();
