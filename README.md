# Chainlink with Observer Job Type

This is fork of https://github.com/smartcontractkit/chainlink that introduces a new job type: "observer":

```toml
type = "observer"
schemaVersion = 1
evmChainID = 80001
interval = "5s"
name = "FunctionsOracle Logs Observer for Mumbai"
addresses = [ 
    "0xeA6721aC65BCeD841B8ec3fc5fEdeA6141a0aDE4", 
    "0xd538f0E053389523f81aaDb502083849D6bE1638" 
]
events = [ 
    "OracleRequest(bytes32 indexed requestId, address requestingContract, address requestInitiator, uint64 subscriptionId, address subscriptionOwner, bytes data)",
    "OracleResponse(bytes32 indexed requestId)" 
]

observationSource = """
    send [type="bridge" name="kafka_ea" requestData="{\\"topic\\":\\"FunctionsOracle\\", \\"key\\":$(jobRun.event.requestId), \\"message\\":$(jobRun.event)}"]
    send
"""
```
