// Package pipeline implements the basic data processing pipeline used by peco
package pipeline


import (
	"fmt"

	"github.com/pkg/errors"

	"golang.org/x/net/context"
)

// EndMark returns true
func (e EndMark) EndMark() bool {
	return true
}

// Error returns the error string "end of input"
func (e EndMark) Error() string {
	return "end of input"
}

// IsEndMark is an utility function that checks if the given error
// object is an EndMark
func IsEndMark(err error) bool {
	if em, ok := errors.Cause(err).(EndMarker); ok {
		fmt.Printf("is end marker!\n")
		return em.EndMark()
	}
	return false
}

// OutCh returns the channel that acceptors can listen to
func (oc OutputChannel) OutCh() <-chan interface{} {
	return oc
}

// Send sends the data `v` through this channel
func (oc OutputChannel) Send(v interface{}) {
	oc <- v
}

// SendEndMark sends an end mark
func (oc OutputChannel) SendEndMark(s string) {
	oc.Send(errors.Wrap(EndMark{}, s))
}

// New creates a new Pipeline
func New() *Pipeline {
	return &Pipeline{}
}

// SetSource sets the source.
// If called during `Run`, this method will block.
func (p *Pipeline) SetSource(s Source) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.src = s
}

// Add adds new ProcNodes that work on data that goes through the Pipeline.
// If called during `Run`, this method will block.
func (p *Pipeline) Add(n ProcNode) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.nodes = append(p.nodes, n)
}

// SetDestination sets the destination.
// If called during `Run`, this method will block.
func (p *Pipeline) SetDestination(d Destination) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.dst = d
}

// Run starts the processing. Mutator methods for `Pipeline` cannot be
// called while `Run` is running.
func (p *Pipeline) Run(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.src == nil {
		return errors.New("source must be non-nil")
	}

	if p.dst == nil {
		return errors.New("destination must be non-nil")
	}

	// Reset is called on the destination to effectively reset
	// any state changes that may have happened in the end of
	// the previous call to Run()
	p.dst.Reset()

	// Setup the ProcNodes, effectively chaining all nodes
	// starting from the destination, working all the way
	// up to the Source
	var prev Acceptor // Explicit type here
	prev = p.dst
	for i := len(p.nodes) - 1; i >= 0; i-- {
		cur := p.nodes[i]
		go prev.Accept(ctx, cur)
		prev = cur
	}

	// Chain to Source...
	go prev.Accept(ctx, p.src)

	// And now tell the Source to send the values so data chugs
	// through the pipeline
	go p.src.Start(ctx)

	// Wait till we're done
	<-p.dst.Done()

	return nil
}
