package session

type Options struct {
	genid             func() string
	name              string
	proxy             bool
	propagateTouch    bool
	resave            bool
	rolling           bool
	saveUninitialized bool
	secret            string
	store             MemoryStore
}

type SessionMiddleware struct {
	option *Options
}

func NewSessionMiddleware(o *Options) *SessionMiddleware {
	return &SessionMiddleware{
		option: o,
	}
}
