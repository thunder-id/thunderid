// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package eventlistener

import "errors"

// Hook registers listeners on one topic. The producer hands it to the composition root, which needs
// it because listeners usually depend on services built after the producer.
type Hook[T any] interface {
	// Add registers a listener. It rejects a nil listener. Listeners cannot be removed.
	Add(listener Listener[T]) error
}

// hook implements Hook by appending to its topic's listener list.
type hook[T any] struct {
	topic *Topic[T]
}

// Add implements Hook.
func (h hook[T]) Add(listener Listener[T]) error {
	if listener == nil {
		return errors.New("event listener must not be nil")
	}
	h.topic.listeners = append(h.topic.listeners, listener)
	return nil
}

// Hook returns the registration end of the topic.
func (t *Topic[T]) Hook() Hook[T] {
	return hook[T]{topic: t}
}
