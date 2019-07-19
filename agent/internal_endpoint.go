package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
)

// InternalDiscoveryChain is helpful for debugging. Eventually we should expose
// this data officially somehow.
func (s *HTTPServer) InternalDiscoveryChain(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.RelatedConfigEntryQuery
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/internal/discovery-chain/")
	if args.ServiceName == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing chain name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedGenericConfigEntries
	defer setMeta(resp, &out.QueryMeta)

	if err := s.agent.RPC("ConfigEntry.ListRelated", &args, &out); err != nil {
		return nil, err
	}

	entries := structs.NewDiscoveryChainConfigEntries()
	if len(out.Entries) > 0 {
		entries.AddEntries(out.Entries...)
	}

	const currentNamespace = "default"

	// Then we compile it into something useful.
	chain, err := discoverychain.Compile(discoverychain.CompileRequest{
		ServiceName:       args.ServiceName,
		CurrentNamespace:  currentNamespace,
		CurrentDatacenter: s.agent.config.Datacenter,
		InferDefaults:     true,
		Entries:           entries,
	})
	if err != nil {
		return nil, err
	}

	if chain == nil {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	return chain, nil
}
