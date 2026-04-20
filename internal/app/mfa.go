package app

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/bubbletea"
	ilog "github.com/shidaxi/iaws/internal/log"
)

// mfaPrompter bridges the AWS SDK's synchronous MFA TokenProvider callback
// (invoked from a tea.Cmd goroutine) to the TUI's asynchronous message loop.
//
// Flow:
//   1. SDK calls Provide() from a background goroutine; it enqueues
//      mfaRequestMsg via program.Send() and blocks on a response channel.
//   2. TUI receives mfaRequestMsg, opens the MFA prompt overlay.
//   3. User types the code, presses Enter; TUI calls Submit(code) which
//      resolves the pending channel; SDK call unblocks with the code.
//
// Concurrent callers are serialized with a mutex so only one prompt is
// visible at a time; subsequent callers share the first submission if it
// is still pending.
type mfaPrompter struct {
	program *tea.Program

	mu      sync.Mutex
	pending chan mfaResult
}

type mfaResult struct {
	code string
	err  error
}

func newMFAPrompter() *mfaPrompter {
	return &mfaPrompter{}
}

// SetProgram wires the bubbletea program so Provide() can deliver messages
// to the TUI. Must be called after tea.NewProgram(...) and before any SDK
// call that may trigger MFA.
func (p *mfaPrompter) SetProgram(prog *tea.Program) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.program = prog
}

// Provide is installed as LoadOptions.MFATokenProvider. It runs on a
// tea.Cmd goroutine and blocks until Submit() or Cancel() is called.
func (p *mfaPrompter) Provide() (string, error) {
	p.mu.Lock()
	if code := os.Getenv("AWS_MFA_CODE"); code != "" {
		p.mu.Unlock()
		return code, nil
	}
	if p.program == nil {
		p.mu.Unlock()
		return "", fmt.Errorf("MFA required but TUI program not attached; set AWS_MFA_CODE env var")
	}
	if p.pending != nil {
		// another goroutine already opened a prompt; wait for its result
		ch := p.pending
		p.mu.Unlock()
		res := <-ch
		return res.code, res.err
	}
	ch := make(chan mfaResult, 1)
	p.pending = ch
	prog := p.program
	p.mu.Unlock()

	ilog.Info("mfa: prompting user for code via TUI")
	prog.Send(mfaRequestMsg{})

	res := <-ch
	if res.err != nil {
		ilog.Info("mfa: prompt cancelled: %v", res.err)
		return "", res.err
	}
	ilog.Info("mfa: code received from TUI")
	return res.code, nil
}

// Submit resolves the pending prompt with a user-entered code.
// Safe to call from the TUI Update loop.
func (p *mfaPrompter) Submit(code string) {
	p.mu.Lock()
	ch := p.pending
	p.pending = nil
	p.mu.Unlock()
	if ch == nil {
		return
	}
	ch <- mfaResult{code: code}
}

// Cancel aborts the pending prompt (e.g. user pressed Esc).
func (p *mfaPrompter) Cancel() {
	p.mu.Lock()
	ch := p.pending
	p.pending = nil
	p.mu.Unlock()
	if ch == nil {
		return
	}
	ch <- mfaResult{err: fmt.Errorf("MFA prompt cancelled by user")}
}

// HasPending reports whether a prompt is currently open.
func (p *mfaPrompter) HasPending() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pending != nil
}
