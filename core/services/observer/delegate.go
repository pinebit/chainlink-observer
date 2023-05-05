package observer

import (
	"github.com/pkg/errors"

	"github.com/smartcontractkit/chainlink/v2/core/chains/evm"
	"github.com/smartcontractkit/chainlink/v2/core/logger"
	"github.com/smartcontractkit/chainlink/v2/core/services/job"
	"github.com/smartcontractkit/chainlink/v2/core/services/pg"
	"github.com/smartcontractkit/chainlink/v2/core/services/pipeline"
)

type Delegate struct {
	pipelineRunner pipeline.Runner
	chains         evm.ChainSet
	lggr           logger.Logger
}

var _ job.Delegate = (*Delegate)(nil)

func NewDelegate(pipelineRunner pipeline.Runner, chains evm.ChainSet, lggr logger.Logger) *Delegate {
	return &Delegate{
		pipelineRunner: pipelineRunner,
		chains:         chains,
		lggr:           lggr,
	}
}

func (d *Delegate) JobType() job.Type {
	return job.Observer
}

func (d *Delegate) BeforeJobCreated(spec job.Job)                {}
func (d *Delegate) AfterJobCreated(spec job.Job)                 {}
func (d *Delegate) BeforeJobDeleted(spec job.Job)                {}
func (d *Delegate) OnDeleteJob(spec job.Job, q pg.Queryer) error { return nil }

// ServicesForSpec returns the scheduler to be used for running observer jobs
func (d *Delegate) ServicesForSpec(spec job.Job) (services []job.ServiceCtx, err error) {
	if spec.ObserverSpec == nil {
		return nil, errors.Errorf("services.Delegate expects a *jobSpec.ObserverSpec to be present, got %v", spec)
	}

	observer, err := NewObserverFromJobSpec(spec, d.pipelineRunner, d.chains, d.lggr)
	if err != nil {
		return nil, err
	}

	return []job.ServiceCtx{observer}, nil
}
