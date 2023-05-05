package observer_test

import (
	"testing"

	"github.com/manyminds/api2go/jsonapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink/v2/core/services/job"
	"github.com/smartcontractkit/chainlink/v2/core/services/observer"
)

func TestValidatedObserverJobSpec(t *testing.T) {
	var tt = []struct {
		name      string
		toml      string
		assertion func(t *testing.T, os job.Job, err error)
	}{
		{
			name: "valid spec",
			toml: `
type          = "observer"
schemaVersion = 1
evmChainID = 43113
interval = "5s"
name = "FunctionsOracle Logs Observer for Fuji"
addresses = [ 
    "0xE569061eD8244643169e81293b0aA0d3335fD563", 
    "0xb252c56A4Ccb681BFA3E4D7CD04590e9Ec23B52E" 
]
events = [ 
    "OracleRequest(bytes32 indexed requestId, address requestingContract, address requestInitiator, uint64 subscriptionId, address subscriptionOwner, bytes data)",
    "OracleResponse(bytes32 indexed requestId)" 
]
observationSource = """
    send [type="bridge" name="kafka_bridge" requestData="{\\"topic\\":\\"FunctionsOracle\\", \\"key\\":$(jobRun.event.requestId), \\"message\\":$(jobRun.event)}"]
    send
"""
`,
			assertion: func(t *testing.T, s job.Job, err error) {
				require.NoError(t, err)
				require.NotNil(t, s.ObserverSpec)
				b, err := jsonapi.Marshal(s.ObserverSpec)
				require.NoError(t, err)
				var r job.ObserverSpec
				err = jsonapi.Unmarshal(b, &r)
				require.NoError(t, err)
				assert.Equal(t, int64(43113), r.EVMChainID.Int64())
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s, err := observer.ValidatedObserverSpec(tc.toml)
			tc.assertion(t, s, err)
		})
	}
}
