package signaling

type callback func()

// Signaler manages signaling events to callbacks.
// A zero-value Signaler is ready to use.
type Signaler struct {
	callbacks map[*callback]struct{}
}

// Connect connects a callback to the signaler. The returned function
// disconnects the callback.
func (s *Signaler) Connect(f func()) func() {
	if s.callbacks == nil {
		s.callbacks = make(map[*callback]struct{})
	}

	cb := (*callback)(&f)
	s.callbacks[cb] = struct{}{}

	return func() {
		delete(s.callbacks, cb)
	}
}

// Signal signals all callbacks.
func (s *Signaler) Signal() {
	for cb := range s.callbacks {
		(*cb)()
	}
}

// Disconnect disconnects all callbacks.
func (s *Signaler) Disconnect() {
	for cb := range s.callbacks {
		delete(s.callbacks, cb)
	}
}

// DisconnectStack is a stack of disconnect functions.
// Use it to defer disconnecting callbacks.
type DisconnectStack struct {
	funcs []func()
}

// Push pushes a disconnect function to the stack.
func (d *DisconnectStack) Push(funcs ...func()) {
	d.funcs = append(d.funcs, funcs...)
}

// Connect connects a callback to the stack.
func (d *DisconnectStack) Connect(s *Signaler, f func()) {
	d.Push(s.Connect(f))
}

// Pop pops a disconnect function from the stack.
func (d *DisconnectStack) Pop() {
	if len(d.funcs) == 0 {
		return
	}

	f := d.funcs[len(d.funcs)-1]
	d.funcs[len(d.funcs)-1] = nil
	d.funcs = d.funcs[:len(d.funcs)-1]
	f()
}

// Disconnect disconnects all callbacks.
func (d *DisconnectStack) Disconnect() {
	for _, f := range d.funcs {
		f()
	}
	d.funcs = nil
}
