package stress

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Scheduler manages request scheduling for stress tests
type Scheduler struct {
	config  *Config
	limiter *rate.Limiter
	sem     chan struct{} // semaphore for max concurrency

	// For weighted request selection
	weights     []int
	totalWeight int

	mu       sync.Mutex
	requests []*ScheduledRequest
}

// ScheduledRequest holds a request with its configuration
type ScheduledRequest struct {
	Index  int
	Name   string
	Config *RequestConfig
}

// NewScheduler creates a new scheduler with the given config
func NewScheduler(config *Config) *Scheduler {
	s := &Scheduler{
		config:   config,
		requests: make([]*ScheduledRequest, 0),
		weights:  make([]int, 0),
	}

	// Initialize rate limiter for rate mode
	if config.Mode == RateMode && config.Rate > 0 {
		s.limiter = rate.NewLimiter(rate.Limit(config.Rate), 1)
	}

	// Initialize semaphore for max concurrency
	maxVUs := config.MaxVUs
	if maxVUs < 1 {
		maxVUs = 100
	}
	s.sem = make(chan struct{}, maxVUs)

	return s
}

// AddRequest adds a request to the scheduler
func (s *Scheduler) AddRequest(index int, name string, config *RequestConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req := &ScheduledRequest{
		Index:  index,
		Name:   name,
		Config: config,
	}
	s.requests = append(s.requests, req)

	weight := 1
	if config != nil && config.Weight > 0 {
		weight = config.Weight
	}
	s.weights = append(s.weights, weight)
	s.totalWeight += weight
}

// SelectRequest selects a request based on weights
func (s *Scheduler) SelectRequest() *ScheduledRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.requests) == 0 {
		return nil
	}

	if len(s.requests) == 1 {
		return s.requests[0]
	}

	// Weighted random selection
	r := rand.Intn(s.totalWeight)
	cumulative := 0
	for i, w := range s.weights {
		cumulative += w
		if r < cumulative {
			return s.requests[i]
		}
	}

	// Fallback to last request
	return s.requests[len(s.requests)-1]
}

// Wait waits for rate limiter (rate mode) or returns immediately (VU mode)
func (s *Scheduler) Wait(ctx context.Context) error {
	if s.limiter != nil {
		return s.limiter.Wait(ctx)
	}
	return nil
}

// Acquire acquires a slot from the concurrency semaphore
func (s *Scheduler) Acquire(ctx context.Context) error {
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a slot back to the semaphore
func (s *Scheduler) Release() {
	<-s.sem
}

// GetCurrentRate returns the current target rate based on ramp-up
func (s *Scheduler) GetCurrentRate(elapsed time.Duration) float64 {
	if s.config.RampUp <= 0 || elapsed >= s.config.RampUp {
		return s.config.Rate
	}

	// Linear ramp-up
	progress := float64(elapsed) / float64(s.config.RampUp)
	return s.config.Rate * progress
}

// GetCurrentVUs returns the current target VUs based on ramp-up
func (s *Scheduler) GetCurrentVUs(elapsed time.Duration) int {
	if s.config.RampUp <= 0 || elapsed >= s.config.RampUp {
		return s.config.VUs
	}

	// Linear ramp-up
	progress := float64(elapsed) / float64(s.config.RampUp)
	return int(float64(s.config.VUs) * progress)
}

// UpdateRate updates the rate limiter's rate
func (s *Scheduler) UpdateRate(newRate float64) {
	if s.limiter != nil && newRate > 0 {
		s.limiter.SetLimit(rate.Limit(newRate))
	}
}

// RequestCount returns the number of registered requests
func (s *Scheduler) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

// GetRequests returns all registered requests
func (s *Scheduler) GetRequests() []*ScheduledRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*ScheduledRequest, len(s.requests))
	copy(result, s.requests)
	return result
}

// VURunner runs requests in VU mode
type VURunner struct {
	id        int
	scheduler *Scheduler
	config    *Config
	metrics   *Metrics
	executor  func(ctx context.Context, req *ScheduledRequest) error
	ctx       context.Context
	cancel    context.CancelFunc
	wg        *sync.WaitGroup
}

// NewVURunner creates a new VU runner
func NewVURunner(id int, scheduler *Scheduler, config *Config, metrics *Metrics, executor func(ctx context.Context, req *ScheduledRequest) error) *VURunner {
	return &VURunner{
		id:        id,
		scheduler: scheduler,
		config:    config,
		metrics:   metrics,
		executor:  executor,
	}
}

// Start starts the VU runner
func (v *VURunner) Start(ctx context.Context, wg *sync.WaitGroup) {
	v.ctx, v.cancel = context.WithCancel(ctx)
	v.wg = wg

	wg.Add(1)
	go v.run()
}

// Stop stops the VU runner
func (v *VURunner) Stop() {
	if v.cancel != nil {
		v.cancel()
	}
}

func (v *VURunner) run() {
	defer v.wg.Done()

	v.metrics.IncrementActiveVUs()
	defer v.metrics.DecrementActiveVUs()

	for {
		select {
		case <-v.ctx.Done():
			return
		default:
		}

		// Select a request
		req := v.scheduler.SelectRequest()
		if req == nil {
			return
		}

		// Acquire semaphore slot
		if err := v.scheduler.Acquire(v.ctx); err != nil {
			return
		}

		// Execute request
		_ = v.executor(v.ctx, req)

		// Release slot
		v.scheduler.Release()

		// Think time
		thinkTime := v.config.ThinkTime
		if req.Config != nil && req.Config.Think > 0 {
			thinkTime = time.Duration(req.Config.Think) * time.Millisecond
		}

		if thinkTime > 0 {
			select {
			case <-v.ctx.Done():
				return
			case <-time.After(thinkTime):
			}
		}
	}
}

// VUPool manages a pool of virtual users
type VUPool struct {
	scheduler *Scheduler
	config    *Config
	metrics   *Metrics
	executor  func(ctx context.Context, req *ScheduledRequest) error
	runners   []*VURunner
	mu        sync.Mutex
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewVUPool creates a new VU pool
func NewVUPool(scheduler *Scheduler, config *Config, metrics *Metrics, executor func(ctx context.Context, req *ScheduledRequest) error) *VUPool {
	return &VUPool{
		scheduler: scheduler,
		config:    config,
		metrics:   metrics,
		executor:  executor,
		runners:   make([]*VURunner, 0),
	}
}

// Start starts the VU pool with the initial number of VUs
func (p *VUPool) Start(ctx context.Context) {
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start initial VUs (or ramp up from 0)
	initialVUs := p.scheduler.GetCurrentVUs(0)
	if initialVUs < 1 {
		initialVUs = 1
	}

	for i := 0; i < initialVUs; i++ {
		p.addVU()
	}
}

// Scale adjusts the number of running VUs
func (p *VUPool) Scale(targetVUs int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	currentVUs := len(p.runners)

	if targetVUs > currentVUs {
		// Scale up
		for i := currentVUs; i < targetVUs; i++ {
			p.addVULocked()
		}
	} else if targetVUs < currentVUs {
		// Scale down
		for i := currentVUs - 1; i >= targetVUs; i-- {
			if i >= 0 && i < len(p.runners) {
				p.runners[i].Stop()
				p.runners = p.runners[:i]
			}
		}
	}
}

func (p *VUPool) addVU() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.addVULocked()
}

func (p *VUPool) addVULocked() {
	id := len(p.runners)
	runner := NewVURunner(id, p.scheduler, p.config, p.metrics, p.executor)
	runner.Start(p.ctx, &p.wg)
	p.runners = append(p.runners, runner)
}

// Stop stops all VUs
func (p *VUPool) Stop() {
	p.mu.Lock()
	for _, r := range p.runners {
		r.Stop()
	}
	p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}
}

// Wait waits for all VUs to finish
func (p *VUPool) Wait() {
	p.wg.Wait()
}

// Count returns the current number of running VUs
func (p *VUPool) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.runners)
}
