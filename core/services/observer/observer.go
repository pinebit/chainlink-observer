package observer

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/chainlink/v2/core/chains/evm"
	"github.com/smartcontractkit/chainlink/v2/core/chains/evm/logpoller"
	evmtypes "github.com/smartcontractkit/chainlink/v2/core/chains/evm/types"
	"github.com/smartcontractkit/chainlink/v2/core/logger"
	"github.com/smartcontractkit/chainlink/v2/core/services/job"
	"github.com/smartcontractkit/chainlink/v2/core/services/pipeline"
	"github.com/smartcontractkit/chainlink/v2/core/utils"
)

// Observer runs an observer jobSpec from a ObserverSpec
type Observer struct {
	logger         logger.Logger
	jobSpec        job.Job
	pipelineRunner pipeline.Runner
	chains         evm.ChainSet
	ticker         *time.Ticker
	chStop         utils.StopChan

	lastBlock int64
	addresses evmtypes.AddressArray
	hashes    evmtypes.HashArray
	abis      map[common.Hash]abi.Event
}

// NewObserverFromJobSpec instantiates a job that observes evm log events.
func NewObserverFromJobSpec(
	jobSpec job.Job,
	pipelineRunner pipeline.Runner,
	chains evm.ChainSet,
	logger logger.Logger,
) (*Observer, error) {
	return &Observer{
		logger:         logger.Named("Observer").With("jobID", jobSpec.ID),
		jobSpec:        jobSpec,
		pipelineRunner: pipelineRunner,
		chains:         chains,
		ticker:         nil,
		chStop:         make(chan struct{}),
		lastBlock:      0,
		addresses:      evmtypes.AddressArray{},
		hashes:         evmtypes.HashArray{},
		abis:           make(map[common.Hash]abi.Event),
	}, nil
}

// Start implements the job.Service interface.
func (o *Observer) Start(context.Context) error {
	o.logger.Debug("Starting")

	// Configuring LogPoller
	observerJobSpec := o.jobSpec.ObserverSpec
	chain, err := o.chains.Get(observerJobSpec.EVMChainID.ToInt())
	if err != nil {
		o.logger.Errorw("cannot resolve evm chain", "EVMChainID", observerJobSpec.EVMChainID)
		return err
	}
	for _, address := range observerJobSpec.Addresses {
		addr, err := common.NewMixedcaseAddressFromString(address)
		if err != nil {
			return err
		}
		o.addresses = append(o.addresses, addr.Address())
	}
	for _, event := range observerJobSpec.Events {
		name, args, _, err := pipeline.ParseETHABIString([]byte(event), true)
		if err != nil {
			o.logger.Errorw("malformed event signature", "event", event)
			return err
		}
		abiEvent := abi.NewEvent(name, name, false, args)
		o.abis[abiEvent.ID] = abiEvent
		o.hashes = append(o.hashes, abiEvent.ID)
	}

	o.lastBlock, err = chain.LogPoller().LatestBlock()
	if err != nil {
		o.logger.Errorw("failed LogPoller().LatestBlock()", "err", err)
		return err
	}

	filter := logpoller.Filter{
		Name:      fmt.Sprintf("Observer job: %s", o.jobSpec.Name.ValueOrZero()),
		Addresses: o.addresses,
		EventSigs: o.hashes,
		Retention: 5 * time.Minute,
	}

	err = chain.LogPoller().RegisterFilter(filter)
	if err != nil {
		o.logger.Errorw("failed LogPoller().RegisterFilter", "err", err)
		return err
	}

	o.logger.Infow("Registered filter", "filter", filter)

	// Firing a ticker
	o.ticker = time.NewTicker(observerJobSpec.Interval.Duration())
	go func() {
		for {
			select {
			case <-o.chStop:
				return
			case <-o.ticker.C:
				o.checkEvents()
			}
		}
	}()

	return nil
}

// Close implements the job.Service interface. It stops this job from
// running and cleans up resources.
func (o *Observer) Close() error {
	o.logger.Debug("Closing")
	if o.ticker != nil {
		o.ticker.Stop()
	}
	close(o.chStop)
	return nil
}

func (o *Observer) checkEvents() {
	o.logger.Info("checkEvents()")

	ctx, cancel := o.chStop.NewCtx()
	defer cancel()

	observerJobSpec := o.jobSpec.ObserverSpec
	chain, err := o.chains.Get(observerJobSpec.EVMChainID.ToInt())
	if err != nil {
		o.logger.Errorw("cannot resolve evm chain, job will be stopped", "EVMChainID", observerJobSpec.EVMChainID)
		o.ticker.Stop()
		return
	}
	currentBlock, err := chain.LogPoller().LatestBlock()
	if err != nil {
		o.logger.Errorw("failed LogPoller().LatestBlock()", "err", err)
		return
	}
	if o.lastBlock == currentBlock {
		o.logger.Info("no new blocks yet")
		return
	}
	o.logger.Info("got new block(s)")

	for _, address := range o.addresses {
		o.logger.Infow("querying logs with sigs", "address", address, "hashes", o.hashes, "from", o.lastBlock+1, "to", currentBlock)
		logs, err := chain.LogPoller().LogsWithSigs(o.lastBlock+1, currentBlock, o.hashes, address)
		if err != nil {
			o.logger.Errorw("failed to get LogsWithSigs", "address", address, "err", err)
			continue
		}
		o.logger.Infof("got %d logs", len(logs))
		for _, log := range logs {
			abi, ok := o.abis[log.EventSig]
			if !ok {
				o.logger.Errorw("unexpected EventSig from LogsWithSigs", "address", address, "err", err, "eventsig", log.EventSig)
				continue
			}

			eventMap := make(map[string]interface{})
			if err := abi.Inputs.UnpackIntoMap(eventMap, log.Data); err != nil {
				o.logger.Errorw("failed to decode event log", "address", address, "err", err, "eventsig", log.EventSig)
				continue
			}

			eventMap["requestId"] = log.ToGethLog().Topics[1]
			o.logger.Infow("unpacked event", "event", eventMap)

			vars := pipeline.NewVarsFrom(map[string]interface{}{
				"jobSpec": map[string]interface{}{
					"databaseID":    o.jobSpec.ID,
					"externalJobID": o.jobSpec.ExternalJobID,
					"name":          o.jobSpec.Name.ValueOrZero(),
				},
				"jobRun": map[string]interface{}{
					"meta":    map[string]interface{}{},
					"address": address.String(),
					"event":   eventMap,
				},
			})

			run := pipeline.NewRun(*o.jobSpec.PipelineSpec, vars)

			if _, err := o.pipelineRunner.Run(ctx, &run, o.logger, false, nil); err != nil {
				o.logger.Errorf("Error executing new run for jobSpec ID %v", o.jobSpec.ID)
			}
		}
	}

	o.lastBlock = currentBlock
}
