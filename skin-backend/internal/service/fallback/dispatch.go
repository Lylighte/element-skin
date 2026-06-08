package fallback

import (
	"context"
	"sort"
	"sync"
)

func (f Fallback) enabledEndpoints(ctx context.Context, kind string) ([]map[string]any, error) {
	eps, err := f.DB.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(eps))
	for _, ep := range eps {
		switch kind {
		case "profile":
			if ep["enable_profile"].(bool) {
				out = append(out, ep)
			}
		case "hasJoined":
			if ep["enable_hasjoined"].(bool) {
				out = append(out, ep)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i]["priority"].(int) < out[j]["priority"].(int)
	})
	return out, nil
}

func (f Fallback) dispatch(ctx context.Context, eps []map[string]any, strategy string, call func(map[string]any) (*FallbackResponse, error)) (*FallbackResponse, error) {
	if strategy != "parallel" {
		for _, ep := range eps {
			resp, err := call(ep)
			if err != nil {
				continue
			}
			if resp != nil {
				return resp, nil
			}
		}
		return nil, nil
	}
	type result struct {
		resp *FallbackResponse
		err  error
	}
	ch := make(chan result, len(eps))
	var wg sync.WaitGroup
	for _, ep := range eps {
		ep := ep
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := call(ep)
			ch <- result{resp: resp, err: err}
		}()
	}
	wg.Wait()
	close(ch)
	for r := range ch {
		if r.err != nil {
			continue
		}
		if r.resp != nil {
			return r.resp, nil
		}
	}
	return nil, nil
}
