// Package workerpool provides a fixed-size pool of goroutines that consume
// jobs from a bounded queue, so work can be parallelized without spawning an
// unbounded number of goroutines under load.
package workerpool

import "sync"

// Pool runs submitted jobs on a fixed number of worker goroutines.
type Pool struct {
	jobs chan func()
	wg   sync.WaitGroup
}

// New starts a Pool with the given number of workers, each pulling from a
// queue buffered to hold queueSize pending jobs.
func New(workers, queueSize int) *Pool {
	p := &Pool{jobs: make(chan func(), queueSize)}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer p.wg.Done()
			for job := range p.jobs {
				job()
			}
		}()
	}
	return p
}

// Submit enqueues a job to be run by a worker. It blocks if the queue is
// full, applying backpressure rather than growing without bound.
func (p *Pool) Submit(job func()) {
	p.jobs <- job
}

// Close stops accepting new jobs and waits for in-flight jobs to finish.
func (p *Pool) Close() {
	close(p.jobs)
	p.wg.Wait()
}
