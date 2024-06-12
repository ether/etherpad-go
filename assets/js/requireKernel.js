var require = /* eslint-disable strict */ (() => {
    // The contents of this file are evaluated as the right-hand side of an assignment expression
    // (this file is not loaded as a module or in a <script> element). Because of this unusual
    // evaluation, strict mode cannot be enabled outside the IIFE.
    //
    // Note: Enabling strict mode here does not affect strict mode of code passed to a Function
    // constructor, so it does not cause modules to load in strict mode.
    //
    // Warning: Strict mode changes eval's behavior, and the behavior of the code evaluated, so the
    // require kernel must not use eval to load modules if strict mode is enabled.
    'use strict';

    /*
     * require-kernel
     *
     * Created by Chad Weider on 01/04/11.
     * Released to the Public Domain on 17/01/12.
     */

    /* Storage */
    let main = null; // Reference to main module in `modules`.
    const modules = {}; // Repository of module objects build from `definitions`.
    const definitions = {}; // Functions that construct `modules`.
    const loadingModules = {}; // Locks for detecting circular dependencies.
    const definitionWaiters = {}; // Locks for clearing duplicate requires.
    const fetchRequests = []; // Queue of pending requests.
    let currentRequests = 0; // Synchronization for parallel requests.
    let maximumRequests = 2; // The maximum number of parallel requests.
    const deferred = []; // A list of callbacks that can be evaluated eventually.
    let deferredScheduled = false; // If deferred functions will be executed.

    let syncLock = undefined;
    let globalKeyPath = undefined;

    let rootURI = undefined;
    let libraryURI = undefined;

    let libraryLookupComponent = undefined;

    const JSONP_TIMEOUT = 60 * 1000;

    function CircularDependencyError(message) {
        this.name = 'CircularDependencyError';
        this.message = message;
    }
    CircularDependencyError.prototype = Error.prototype;
    function ArgumentError(message) {
        this.name = 'ArgumentError';
        this.message = message;
    }
    ArgumentError.prototype = Error.prototype;

    /* Utility */
    // Object-independent because an object may define `hasOwnProperty`.
    const hasOwnProperty = (object, key) => Object.prototype.hasOwnProperty.call(object, key);

    /* Deferral */
    const defer = (...fns) => deferred.push(...fns);

    const _flushDefer = () => {
        // Let exceptions happen, but don't allow them to break notification.
        try {
            while (deferred.length) {
                const continuation = deferred.shift();
                continuation();
            }
            deferredScheduled = false;
        } finally {
            deferredScheduled = deferred.length > 0;
            deferred.length && setTimeout(_flushDefer, 0);
        }
    };

    const flushDefer = () => {
        if (!deferredScheduled && deferred.length > 0) {
            if (syncLock) {
                // Only asynchronous operations will wait on this condition so schedule
                // and don't interfere with the synchronous operation in progress.
                deferredScheduled = true;
                setTimeout(_flushDefer, 0);
            } else {
                _flushDefer();
            }
        }
    };

    const flushDeferAfter = (f) => {
        try {
            deferredScheduled = true;
            f();
            deferredScheduled = false;
            flushDefer();
        } finally {
            deferredScheduled = false;
            deferred.length && setTimeout(flushDefer, 0);
        }
    };

    const normalizePath = (path) => {
        const pathComponents1 = path.split('/');
        const pathComponents2 = [];

        let component;
        for (let i = 0, ii = pathComponents1.length; i < ii; i++) {
            component = pathComponents1[i];
            switch (component) {
                case '':
                    if (i === 0 || i === ii - 1) {
                        // This indicates a leading or trailing slash.
                        pathComponents2.push(component);
                    }
                    break;
                case '.':
                    // Always skip.
                    break;
                case '..':
                    if (pathComponents2.length > 1 ||
                        (pathComponents2.length === 1 &&
                            pathComponents2[0] !== '' &&
                            pathComponents2[0] !== '.')) {
                        pathComponents2.pop();
                        break;
                    }
                // Fall through:
                default:
                    pathComponents2.push(component);
            }
        }

        return pathComponents2.join('/');
    };

    const fullyQualifyPath = (path, basePath) => {
        let fullyQualifiedPath = path;
        if (path.charAt(0) === '.' &&
            (path.charAt(1) === '/' ||
                (path.charAt(1) === '.' && path.charAt(2) === '/'))) {
            if (!basePath) {
                basePath = '';
            } else if (basePath.charAt(basePath.length - 1) !== '/') {
                basePath += '/';
            }
            fullyQualifiedPath = basePath + path;
        }
        return fullyQualifiedPath;
    };

    const setRootURI = (URI) => {
        if (!URI) {
            throw new ArgumentError('Invalid root URI.');
        }
        rootURI = URI.charAt(URI.length - 1) === '/' ? URI.slice(0, -1) : URI;
    };

    const setLibraryURI = (URI) => {
        libraryURI = URI.charAt(URI.length - 1) === '/' ? URI : `${URI}/`;
    };

    const setLibraryLookupComponent = (component) => {
        component = component && component.toString();
        if (!component) {
            libraryLookupComponent = undefined;
        } else if (component.match(/\//)) {
            throw new ArgumentError('Invalid path component.');
        } else {
            libraryLookupComponent = component;
        }
    };

    // If a `libraryLookupComponent` is defined, then library modules should
    // be looked at in every parent directory (roughly).
    const searchPathsForModulePath = (path, basePath) => {
        path = normalizePath(path);

        // Should look for nearby libarary modules.
        if (path.charAt(0) !== '/' && libraryLookupComponent) {
            const paths = [];
            const components = basePath.split('/');

            while (components.length > 1) {
                if (components[components.length - 1] === libraryLookupComponent) {
                    components.pop();
                }
                const searchPath = normalizePath(fullyQualifyPath(
                    `./${libraryLookupComponent}/${path}`, `${components.join('/')}/`
                ));
                paths.push(searchPath);
                components.pop();
            }
            paths.push(path);
            return paths;
        } else {
            return [normalizePath(fullyQualifyPath(path, basePath))];
        }
    };

    const uriForModulePath = (path) => {
        const components = path.split('/');
        for (let i = 0, ii = components.length; i < ii; i++) {
            components[i] = encodeURIComponent(components[i]);
        }
        path = components.join('/');

        if (path.charAt(0) === '/') {
            if (!rootURI) {
                throw new Error(
                    `Attempt to retrieve the root module "${path}" but no root URI is defined.`);
            }
            return rootURI + path;
        } else {
            if (!libraryURI) {
                throw new Error(
                    `Attempt to retrieve the library module "${path}" but no libary URI is defined.`);
            }
            return libraryURI + path;
        }
    };

    // Returns a function that behaves like `(f) => f()` and whose name property is `name`.
    //
    // This is used to improve the readability of stack traces containing anonymous functions. It
    // works by taking advantage of an ES6 feature: When an anonymous function expression is assigned
    // to a variable or an object property, the function's `.name` property is set to the variable
    // name or object property name.
    //
    // For example, instead of:
    //     const x = computeThing(arg);
    // you could do:
    //     const x = stackDecorator('this name appears in the stack')(computeThing.bind(null, arg));
    // If computeThing() throws, you would see "this name appears in the stack" in the stack trace.
    const stackDecorator = (name) => ({[name]: (f) => f()})[name];

    // The first argument is a helpful hint that will appear in the stack trace if there is a syntax
    // error. The remaining arguments are passed to Function().
    const compileFunction = (name, ...args) => {
        // `Function.bind(null, ...args)` is used instead of `() => new Function(...args)` to improve
        // the readability of the stack trace by avoiding an extra anonymous function in the stack.
        const f = Function.bind(null, ...args);
        return stackDecorator(name)(f);
    };

    /* Remote */
    const setRequestMaximum = (value) => {
        value = parseInt(value);
        if (value > 0) {
            maximumRequests = value;
            checkScheduledfetchDefines();
        } else {
            throw new ArgumentError('Value must be a positive integer.');
        }
    };

    const setGlobalKeyPath = (value) => {
        globalKeyPath = value;
    };

    let randomVersionString = null;
    const getRandomVersionString = () => {
        if (typeof window === 'undefined') return null;
        for (let win = window; randomVersionString == null; win = win.parent) {
            ({clientVars: {randomVersionString} = {}} = win);
            if (win === window.top) break;
        }
        return randomVersionString;
    };

    const getXHR = (uri, async, callback) => {
        if (getRandomVersionString()) {
            uri += `&v=${getRandomVersionString()}`;
        }
        const request = new XMLHttpRequest();
        const onComplete = (request) => {
            // Build module constructor.
            if (request.status === 200) {
                callback(undefined, request.responseText);
            } else {
                callback(true, undefined);
            }
        };

        request.open('GET', uri, !!(async));
        if (async) {
            request.onreadystatechange = (event) => {
                if (request.readyState === 4) {
                    onComplete(request);
                }
            };
            request.send(null);
        } else {
            request.send(null);
            onComplete(request);
        }
    };

    const fetchDefineXHR = (path, async) => {
        // If cross domain and request doesn't support such requests, go straight
        // to mirroring.

        const _globalKeyPath = globalKeyPath;

        const callback = (error, text) => {
            if (error) {
                define(path, null);
            } else if (_globalKeyPath) {
                // The space in the name argument passed to compileFunction() is important: When displaying
                // a stack trace in the developer console, Firefox (as of v90) does some mysterious
                // processing that sometimes chops off the first part of the function's `.name` property.
                // The logic seems arbitrary -- it's not simply keeping "good" characters. For example, "foo
                // bar.js" is printed in its entirety, but "foobar.js" becomes "js". Introducing a space
                // seems to cause Firefox to reliably print the entire name. (The `Error.stack` property
                // does not suffer from this problem -- the complete name is always included.)
                compileFunction(`(bundle ${path})`, text)();
            } else {
                // See the above comment for the importance of the space in the name passed to
                // compileFunction().
                const def = compileFunction(`(module ${path})`, 'require', 'exports', 'module', text);
                define(path, def);
            }
        };

        let uri = uriForModulePath(path);
        if (_globalKeyPath) {
            uri += `?callback=${encodeURIComponent(`${globalKeyPath}.define`)}`;
        }
        getXHR(uri, async, callback);
    };

    const fetchDefineJSONP = (path) => {
        const head = document.head ||
            document.getElementsByTagName('head')[0] ||
            document.documentElement;
        const script = document.createElement('script');
        if (script.async !== undefined) {
            script.async = 'true';
        } else {
            script.defer = 'true';
        }
        script.type = 'application/javascript';
        script.src = `${uriForModulePath(path)}?callback=${encodeURIComponent(globalKeyPath)}.define`;

        // Handle failure of JSONP request.
        if (JSONP_TIMEOUT < Infinity) {
            const timeoutId = setTimeout(() => define(path, null), JSONP_TIMEOUT);
            definitionWaiters[path].unshift(() => clearTimeout(timeoutId));
        }

        head.insertBefore(script, head.firstChild);
    };

    /* Modules */
    const fetchModule = (path, continuation) => {
        if (hasOwnProperty(definitionWaiters, path)) {
            definitionWaiters[path].push(continuation);
        } else {
            definitionWaiters[path] = [continuation];
            schedulefetchDefine(path);
        }
    };

    const schedulefetchDefine = (path) => {
        fetchRequests.push(path);
        checkScheduledfetchDefines();
    };

    const checkScheduledfetchDefines = () => {
        if (fetchRequests.length > 0 && currentRequests < maximumRequests) {
            const fetchRequest = fetchRequests.pop();
            currentRequests++;
            definitionWaiters[fetchRequest].unshift(() => {
                currentRequests--;
                checkScheduledfetchDefines();
            });
            if (globalKeyPath &&
                typeof document !== 'undefined' &&
                document.readyState &&
                /^loaded|complete$/.test(document.readyState)) {
                fetchDefineJSONP(fetchRequest);
            } else {
                fetchDefineXHR(fetchRequest, true);
            }
        }
    };

    const fetchModuleSync = (path, continuation) => {
        fetchDefineXHR(path, false);
        continuation();
    };

    const moduleIsLoaded = (path) => hasOwnProperty(modules, path);

    const loadModule = (path, continuation) => {
        // If it's a function then it hasn't been exported yet. Run function and
        //  then replace with exports result.
        if (!moduleIsLoaded(path)) {
            if (hasOwnProperty(loadingModules, path)) {
                throw new CircularDependencyError('Encountered circular dependency.');
            } else if (!moduleIsDefined(path)) {
                throw new Error('Attempt to load undefined module.');
            } else if (definitions[path] === null) { // eslint-disable-line eqeqeq
                continuation(null);
            } else {
                const definition = definitions[path];
                const _module = {id: path, exports: {}};
                const _require = requireRelativeTo(path);
                if (!main) {
                    main = _module;
                }
                try {
                    loadingModules[path] = true;
                    definition(_require, _module.exports, _module);
                    modules[path] = _module;
                    delete loadingModules[path];
                    continuation(_module);
                } finally {
                    delete loadingModules[path];
                }
            }
        } else {
            const module = modules[path];
            continuation(module);
        }
    };

    const _moduleAtPath = (path, fetchFunc, continuation) => {
        const suffixes =
            path.endsWith('.js') ? ['']
                : path.endsWith('/') ? ['index.js']
                    : ['.js', '/index.js', ''];
        const ii = suffixes.length;
        const _find = (i) => {
            if (i < ii) {
                const path_ = path + suffixes[i];
                const after = () => {
                    loadModule(path_, (module) => {
                        if (module === null) { // eslint-disable-line eqeqeq
                            _find(i + 1);
                        } else {
                            continuation(module);
                        }
                    });
                };

                if (!moduleIsDefined(path_)) {
                    fetchFunc(path_, after);
                } else {
                    after();
                }
            } else {
                continuation(null);
            }
        };
        _find(0);
    };

    const moduleAtPath = (path, continuation) => {
        defer(() => {
            _moduleAtPath(path, fetchModule, continuation);
        });
    };

    const moduleAtPathSync = (path) => {
        let module;
        const oldSyncLock = syncLock;
        syncLock = true;

        // HACK TODO
        // This is completely the wrong way to do it but for now it shows it works
        if (path === 'async') {
            // console.warn("path is async and we're doing a ghetto fix");
            path = 'async/lib/async';
        }

        // HACK TODO
        // This is completely the wrong way to do it but for now it shows it works
        if (path === 'underscore') {
            // console.warn("path is async and we're doing a ghetto fix");
            path = 'underscore/underscore';
        }

        // HACK TODO
        // This is completely the wrong way to do it but for now it shows it works
        if (path === 'unorm') {
            // console.warn("path is async and we're doing a ghetto fix");
            path = 'unorm/lib/unorm';
        }

        try {
            _moduleAtPath(path, fetchModuleSync, (_module) => {
                module = _module;
            });
        } finally {
            syncLock = oldSyncLock;
        }
        return module;
    };

    /* Definition */
    const moduleIsDefined = (path) => hasOwnProperty(definitions, path);

    const defineModule = (path, module) => {
        if (typeof path !== 'string' ||
            !((typeof module === 'function') || module === null)) { // eslint-disable-line eqeqeq
            throw new ArgumentError('Definition must be a (string, function) pair.');
        }

        if (moduleIsDefined(path)) {
            // Drop import silently
        } else {
            definitions[path] = module;
        }
    };

    const defineModules = (moduleMap) => {
        if (typeof moduleMap !== 'object') {
            throw new ArgumentError('Mapping must be an object.');
        }
        for (const path in moduleMap) {
            if (hasOwnProperty(moduleMap, path)) {
                defineModule(path, moduleMap[path]);
            }
        }
    };

    const define = (...args) => {
        let moduleMap;
        if (args.length === 1) {
            [moduleMap] = args;
            defineModules(moduleMap);
        } else if (args.length === 2) {
            const [path, module] = args;
            defineModule(path, module);
            moduleMap = {[path]: module};
        } else {
            throw new ArgumentError(`Expected 1 or 2 arguments, but got ${args.length}.`);
        }

        // With all modules installed satisfy those conditions for all waiters.
        for (const path in moduleMap) {
            if (hasOwnProperty(moduleMap, path) && hasOwnProperty(definitionWaiters, path)) {
                defer(...definitionWaiters[path]);
                delete definitionWaiters[path];
            }
        }

        flushDefer();
    };

    /* Require */
    const _designatedRequire = (path, continuation, relativeTo) => {
        const paths = searchPathsForModulePath(path, relativeTo);

        if (continuation === undefined) {
            let module;
            for (let i = 0, ii = paths.length; i < ii && !module; i++) {
                const path = paths[i];
                module = moduleAtPathSync(path);
            }
            if (!module) {
                throw new Error(`The module at "${path}" does not exist.`);
            }
            return module.exports;
        } else {
            if (!(typeof continuation === 'function')) {
                throw new ArgumentError('Continuation must be a function.');
            }

            flushDeferAfter(() => {
                const search = () => {
                    const path = paths.shift();
                    return moduleAtPath(path, (module) => {
                        if (module || paths.length === 0) {
                            continuation(module && module.exports);
                        } else {
                            search();
                        }
                    });
                };
                search();
            });
        }
    };

    const designatedRequire =
        (...args) => (rootRequire._designatedRequire || _designatedRequire)(...args);

    const requireRelative = (basePath, qualifiedPath, continuation) => {
        qualifiedPath = qualifiedPath.toString();
        const path = normalizePath(fullyQualifyPath(qualifiedPath, basePath));
        return designatedRequire(path, continuation, basePath);
    };

    const requireRelativeN = (basePath, qualifiedPaths, continuation) => {
        if (!(typeof continuation === 'function')) {
            throw new ArgumentError('Final argument must be a continuation.');
        } else {
            // Copy and validate parameters
            const _qualifiedPaths = qualifiedPaths.map((p) => p.toString());
            const results = [];
            const _require = (result) => {
                results.push(result);
                if (qualifiedPaths.length > 0) {
                    requireRelative(basePath, qualifiedPaths.shift(), _require);
                } else {
                    continuation(...results);
                }
            };
            for (let i = 0; i < qualifiedPaths.length; i++) {
                requireRelative(basePath, _qualifiedPaths[i], _require);
            }
        }
    };

    const requireRelativeTo = (basePath) => {
        basePath = basePath.replace(/[^/]+$/, '');
        const require = (...args) => {
            if (args.length > 2) {
                const continuation = args.pop();
                const qualifiedPaths = args;
                return requireRelativeN(basePath, qualifiedPaths, continuation);
            } else {
                const [qualifiedPath, continuation] = args;
                return requireRelative(basePath, qualifiedPath, continuation);
            }
        };
        require.main = main;

        return require;
    };

    const rootRequire = requireRelativeTo('/');

    /* Private internals */
    rootRequire._modules = modules;
    rootRequire._definitions = definitions;
    rootRequire._designatedRequire = _designatedRequire;

    /* Public interface */
    rootRequire.define = define;
    rootRequire.setRequestMaximum = setRequestMaximum;
    rootRequire.setGlobalKeyPath = setGlobalKeyPath;
    rootRequire.setRootURI = setRootURI;
    rootRequire.setLibraryURI = setLibraryURI;
    rootRequire.setLibraryLookupComponent = setLibraryLookupComponent;

    return rootRequire;
})(); /* eslint-enable strict */
;
