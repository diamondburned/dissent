//go:build !linux || nowebkit

package login

const browserLoginSupported = false

func (c *Component) loginBrowser() {}
