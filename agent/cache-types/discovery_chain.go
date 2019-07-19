package cachetype

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/hashstructure"
)

// Recommended name for registration.
const CompiledDiscoveryChainName = "compiled-discovery-chain"

// CompiledDiscoveryChain supports fetching the complete discovery chain for a
// service and caching its compilation.
type CompiledDiscoveryChain struct {
	RPC RPC
}

func (c *CompiledDiscoveryChain) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a DiscoveryChainRequest.
	reqReal, ok := req.(*DiscoveryChainRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Set the minimum query index to our current index so we block
	reqReal.QueryOptions.MinQueryIndex = opts.MinIndex
	reqReal.QueryOptions.MaxQueryTime = opts.Timeout

	// Always allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and endup arbitrarily stale anyway. This
	// allows cached compiled-discovery-chain to automatically read scale across all
	// servers too.
	reqReal.AllowStale = true

	// Generate config entry query.
	cfgReq := &structs.RelatedConfigEntryQuery{
		ServiceName:  reqReal.ServiceName,
		Datacenter:   reqReal.Datacenter,
		QueryOptions: reqReal.QueryOptions,
	}

	// Fetch config entries.
	var cfgReply structs.IndexedGenericConfigEntries
	if err := c.RPC.RPC("ConfigEntry.ListRelated", cfgReq, &cfgReply); err != nil {
		return result, err
	}

	entries := structs.NewDiscoveryChainConfigEntries()
	if len(cfgReply.Entries) > 0 {
		entries.AddEntries(cfgReply.Entries...)
	}

	// Then we compile it into something useful.
	chain, err := discoverychain.Compile(discoverychain.CompileRequest{
		ServiceName:       reqReal.ServiceName,
		CurrentNamespace:  reqReal.EvaluateInNamespace,
		CurrentDatacenter: reqReal.EvaluateInDatacenter,
		InferDefaults:     true,
		Entries:           entries,
	})
	if err != nil {
		return result, err
	}

	reply := DiscoveryChainResponse{
		ConfigEntries: entries,
		Chain:         chain,
		QueryMeta:     cfgReply.QueryMeta,
	}

	result.Value = &reply
	result.Index = reply.QueryMeta.Index
	return result, nil
}

func (c *CompiledDiscoveryChain) SupportsBlocking() bool {
	return true
}

// DiscoveryChainRequest is the cache.Request implementation for the
// CompiledDiscoveryChain cache type. This is implemented here and not in
// structs since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type DiscoveryChainRequest struct {
	ServiceName          string
	EvaluateInDatacenter string
	EvaluateInNamespace  string

	Datacenter           string // where to service the request
	structs.QueryOptions        // passed on to underlying ListRelated operation
}

func (r *DiscoveryChainRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	v, err := hashstructure.Hash(struct {
		ServiceName          string
		EvaluateInDatacenter string
		EvaluateInNamespace  string
	}{
		ServiceName:          r.ServiceName,
		EvaluateInDatacenter: r.EvaluateInDatacenter,
		EvaluateInNamespace:  r.EvaluateInNamespace,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// TODO(rb): either fix the compiled results, or take the derived data and stash it here in a json/msgpack-friendly way?
type DiscoveryChainResponse struct {
	ConfigEntries *structs.DiscoveryChainConfigEntries `json:",omitempty"` // TODO(rb): remove these?
	Chain         *structs.CompiledDiscoveryChain      `json:",omitempty"`
	structs.QueryMeta
}
