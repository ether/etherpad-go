//go:build js && wasm

package main

import (
	"errors"
	"syscall/js"
)

type promiseResult struct {
	value js.Value
	err   error
}

func awaitPromise(promise js.Value) (js.Value, error) {
	ch := make(chan promiseResult, 1)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			ch <- promiseResult{value: args[0]}
		} else {
			ch <- promiseResult{value: js.Undefined()}
		}
		return nil
	})
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		msg := "promise rejected"
		if len(args) > 0 {
			msg = args[0].String()
		}
		ch <- promiseResult{err: errors.New(msg)}
		return nil
	})
	promise.Call("then", then)
	promise.Call("catch", catch)
	result := <-ch
	then.Release()
	catch.Release()
	return result.value, result.err
}

func (a *app) validateToken(token string) (bool, error) {
	fn := js.Global().Get("__adminValidateToken")
	if fn.IsUndefined() || fn.IsNull() {
		return true, nil
	}
	value, err := awaitPromise(fn.Invoke(token))
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}

func (a *app) reauth() (string, error) {
	fn := js.Global().Get("__adminReauth")
	if fn.IsUndefined() || fn.IsNull() {
		return "", errors.New("reauth function missing")
	}
	value, err := awaitPromise(fn.Invoke())
	if err != nil {
		return "", err
	}
	if value.Type() == js.TypeString {
		return value.String(), nil
	}
	return "", nil
}
