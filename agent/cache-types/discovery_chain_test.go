package cachetype

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCompiledDiscoveryChain(t *testing.T) {
	rpc := TestRPC(t)
	typ := &CompiledDiscoveryChain{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var innerResp *structs.IndexedGenericConfigEntries
	rpc.On("RPC", "ConfigEntry.ListRelated", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.RelatedConfigEntryQuery)
			require.Equal(t, "web", req.ServiceName)
			require.Equal(t, "dc1", req.Datacenter)
			require.Equal(t, uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(t, 1*time.Second, req.QueryOptions.MaxQueryTime)
			require.True(t, req.AllowStale)

			reply := args.Get(2).(*structs.IndexedGenericConfigEntries)
			reply.Entries = []structs.ConfigEntry{} // just do the default chain
			reply.QueryMeta.Index = 48
			innerResp = reply
		})

	// Fetch
	resultA, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &DiscoveryChainRequest{
		ServiceName:          "web",
		Datacenter:           "dc1",
		EvaluateInDatacenter: "dc1",
		EvaluateInNamespace:  "default",
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: &DiscoveryChainResponse{
			ConfigEntries: structs.NewDiscoveryChainConfigEntries(),
			Chain:         discoverychain.TestCompileConfigEntries(t, "web", "default", "dc1"),
			QueryMeta:     innerResp.QueryMeta,
		},
		Index: 48,
	}, resultA)

	rpc.AssertExpectations(t)
}

func TestCompiledDiscoveryChain_badReqType(t *testing.T) {
	rpc := TestRPC(t)
	typ := &CompiledDiscoveryChain{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
	rpc.AssertExpectations(t)
}
