package ratelimiter

import (
	stdcontext "context"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/fallback"

	metrics "github.com/rcrowley/go-metrics"
	"golang.org/x/time/rate"
)

const (
	// Kind is the kind of RateLimiter.
	Kind = "RateLimiter"

	resultTimeout = "timeout"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultTimeout},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// RateLimiter is the entity to complete rate limiting.
	RateLimiter struct {
		spec *Spec

		fallback *fallback.Fallback

		limiter *rate.Limiter
		rate1   metrics.EWMA
		done    chan struct{}
	}

	// Spec describes RateLimiter.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		TPS      uint32         `yaml:"tps" v:"gte=1"`
		Timeout  string         `yaml:"timeout" v:"omitempty,duration,dmin=1ms"`
		Fallback *fallback.Spec `yaml:"fallback"`

		timeout *time.Duration
	}

	// Status contains status info of RateLimiter.
	Status struct {
		TPS uint64 `yaml:"tps"`
	}
)

// New creates a RateLimiter.
func New(spec *Spec, prev *RateLimiter) *RateLimiter {
	if spec.Timeout != "" {
		timeout, err := time.ParseDuration(spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse durantion %s failed: %v",
				spec.Timeout, err)
		} else {
			spec.timeout = &timeout
		}
	}

	rl := &RateLimiter{spec: spec}
	if spec.Fallback != nil {
		rl.fallback = fallback.New(spec.Fallback)
	}

	if prev == nil {
		rl.limiter = rate.NewLimiter(rate.Limit(spec.TPS), 1)
		rl.rate1 = metrics.NewEWMA1()
		rl.done = make(chan struct{})
		go func() {
			for {
				select {
				case <-time.After(5 * time.Second):
					rl.rate1.Tick()
				case <-rl.done:
					return
				}
			}
		}()
		return rl
	}

	rl.limiter = prev.limiter
	rl.limiter.SetLimit(rate.Limit(spec.TPS))
	rl.rate1 = prev.rate1
	rl.done = prev.done

	return rl
}

// Handle limits HTTPContext.
func (rl *RateLimiter) Handle(ctx context.HTTPContext) string {
	defer rl.rate1.Update(1)

	var rlCtx stdcontext.Context = ctx
	if rl.spec.timeout != nil {
		var cancel stdcontext.CancelFunc
		rlCtx, cancel = stdcontext.WithTimeout(rlCtx, *rl.spec.timeout)
		defer cancel()
	}

	err := rl.limiter.Wait(rlCtx)
	if err != nil {
		if rl.fallback != nil {
			rl.fallback.Fallback(ctx)
		}
		ctx.Response().SetStatusCode(http.StatusTooManyRequests)
		return resultTimeout
	}

	return ""
}

// Status returns RateLimiter status.
func (rl *RateLimiter) Status() *Status {
	return &Status{
		TPS: uint64(rl.rate1.Rate()),
	}
}

// Close closes RateLimiter.
func (rl *RateLimiter) Close() {
	close(rl.done)
}
