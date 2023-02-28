package assertion

import (
	"sync"
	"time"

	"go.ddosify.com/ddosify/core/scenario/scripting/assertion"
	"go.ddosify.com/ddosify/core/scenario/scripting/assertion/evaluator"
	"go.ddosify.com/ddosify/core/types"
)

var tickerInterval = 1000 // interval in millisecond
type AssertionService struct {
	assertions map[string]types.TestAssertionOpt // Rule -> Opts
	resultChan chan *types.ScenarioResult
	abortChan  chan struct{}
	assertEnv  *evaluator.AssertEnv
	abortTick  map[string]int // rule -> tickIndex
	mu         sync.Mutex
}

func NewAssertionService() (service *AssertionService) {
	return &AssertionService{}
}

func (as *AssertionService) Init(assertions map[string]types.TestAssertionOpt) chan struct{} {
	as.assertions = assertions
	abortChan := make(chan struct{})
	as.abortChan = abortChan
	as.assertEnv = &evaluator.AssertEnv{}
	as.abortTick = make(map[string]int)
	as.mu = sync.Mutex{}
	return as.abortChan
}

func (as *AssertionService) Start(input chan *types.ScenarioResult) {
	// get iteration results ,add store them cumulatively
	for r := range input {
		as.mu.Lock()
		as.aggregate(r)
		as.mu.Unlock()
	}
}

func (as *AssertionService) aggregate(r *types.ScenarioResult) {
	var iterationTime int64
	for _, sr := range r.StepResults {
		iterationTime += sr.Duration.Milliseconds()
	}
	as.assertEnv.TotalTime = append(as.assertEnv.TotalTime, iterationTime)
}

func (as *AssertionService) ApplyAssertions() {
	ticker := time.NewTicker(time.Duration(tickerInterval) * time.Millisecond)
	tickIndex := 1
	for range ticker.C {
		// apply assertions
		for rule, opts := range as.assertions {
			res, err := assertion.Assert(rule, as.assertEnv)
			if err != nil {
				// TODO
			}
			if res == false && opts.Abort {
				// if delay is zero, immediately abort
				if opts.Delay == 0 || as.abortTick[rule] == tickIndex {
					as.abortChan <- struct{}{}
					return
				}
				if _, ok := as.abortTick[rule]; !ok {
					// schedule check at
					delayTick := (time.Duration(opts.Delay) * time.Second) / (time.Duration(tickerInterval) * time.Millisecond)
					as.abortTick[rule] = tickIndex + int(delayTick) - 1
				}
			}
		}
		tickIndex++
	}
}

// TODO return a verbose explanation
func (as *AssertionService) GiveFinalResult() bool {
	// return final result
	for rule, _ := range as.assertions {
		res, err := assertion.Assert(rule, as.assertEnv)
		if err != nil {
			// TODO
		}
		if res == false {
			return false
		}
	}
	return true
}