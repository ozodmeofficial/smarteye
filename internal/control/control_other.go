//go:build !windows

package control

import "github.com/ozodmeofficial/smarteye/internal/protocol"

// noopInjector satisfies Injector on non-Windows dev machines.
type noopInjector struct{ clip string }

func newInjector() Injector { return &noopInjector{} }

func (n *noopInjector) Apply(ev protocol.InputEvent, monW, monH, monX, monY int) error { return nil }
func (n *noopInjector) SetClipboard(text string) error                                 { n.clip = text; return nil }
func (n *noopInjector) GetClipboard() (string, error)                                  { return n.clip, nil }
