package packer

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/SevereCloud/vksdk/v2/api"
)

// VKHandler - alias to function which proceeds requests to VK API.
type VKHandler = func(string, ...api.Params) (api.Response, error)

// FilterMode - batch filter mode
type FilterMode bool

const (
	// Allow mode
	Allow FilterMode = true
	// Ignore mode
	Ignore FilterMode = false
)

// Packer struct
type Packer struct {
	maxPackedRequests int
	tokenPool         *tokenPool
	tokenLazyLoading  bool
	filterMode        FilterMode
	filterMethods     map[string]struct{}
	debug             bool
	vkHandler         VKHandler
	batch             batch
	mtx               sync.Mutex
}

// Option - Packer option
type Option func(*Packer)

// MaxPackedRequests sets the maximum API calls inside one batch.
func MaxPackedRequests(max int) Option {
	if max < 1 || max > 25 {
		max = 25
	}
	return func(p *Packer) {
		p.maxPackedRequests = max
	}
}

// Rules sets the batching rules (ignore some methods or allow it).
func Rules(mode FilterMode, methods ...string) Option {
	return func(p *Packer) {
		for _, m := range methods {
			p.filterMode = mode
			p.filterMethods[m] = struct{}{}
		}
	}
}

// Debug enables printing debug info into stdout.
func Debug() Option {
	return func(p *Packer) {
		p.debug = true
	}
}

// Tokens provides tokens which will be used for sending batch requests.
// If tokens are not provided, packer will use tokens from incoming requests.
func Tokens(tokens ...string) Option {
	return func(p *Packer) {
		p.tokenLazyLoading = false
		p.tokenPool = newTokenPool(tokens...)
	}
}

// New creates a new Packer.
//
// NOTE: this method will not create any trigger for sending batches
// which means that the batch will be sent only when the number of requests in it
// equals to 'maxPackedRequests' (default 25, can be overwritten with MaxPackedRequests() option).
// You will need to create your custom logic which sometimes will call packer.Send() method to solve this.
func New(handler VKHandler, opts ...Option) *Packer {
	p := &Packer{
		tokenLazyLoading:  true,
		tokenPool:         newTokenPool(),
		maxPackedRequests: 25,
		filterMode:        Ignore,
		filterMethods:     make(map[string]struct{}),
		vkHandler:         handler,
		batch:             make(batch),
	}
	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Default creates new Packer, wraps vk.Handler and creates
// timeout-based trigger for sending batches every 2 seconds.
func Default(vk *api.VK, opts ...Option) {
	p := New(vk.Handler, opts...)
	vk.Handler = p.Handler
	go func() {
		for {
			time.Sleep(time.Second * 2)
			p.Send()
		}
	}()
}

// Handler implements vk.Handler function, which proceeds requests to VK API.
func (p *Packer) Handler(method string, params ...api.Params) (api.Response, error) {
	if p.debug {
		log.Printf("packer: Handler call (%s)\n", method)
	}

	if method == "execute" {
		return p.vkHandler(method, params...)
	}

	_, found := p.filterMethods[method]
	if (p.filterMode == Allow && !found) ||
		(p.filterMode == Ignore && found) {
		return p.vkHandler(method, params...)
	}

	if p.tokenLazyLoading {
		tokenIface, ok := getTokenFromParams(params...)
		if !ok && p.tokenPool.Len() == 0 {
			return api.Response{}, fmt.Errorf("packer: missing access_token param")
		}

		token, ok := tokenIface.(string)
		if !ok && p.tokenPool.Len() == 0 {
			return api.Response{}, fmt.Errorf("packer: bad access_token type")
		}

		p.tokenPool.Append(token)
	}

	var (
		resp api.Response
		err  error
		wg   sync.WaitGroup
	)

	wg.Add(1)
	handler := func(r api.Response, e error) {
		resp = r
		err = e
		wg.Done()
	}

	p.mtx.Lock()
	p.batch.appendRequest(request{method, params, handler})
	if len(p.batch) == p.maxPackedRequests {
		go p.sendBatch(p.batch)
		p.batch = make(batch)
	}
	p.mtx.Unlock()

	wg.Wait()
	return resp, err
}

// Send sends current batch if it contains at least one request.
func (p *Packer) Send() {
	p.mtx.Lock()
	if len(p.batch) > 0 {
		go p.sendBatch(p.batch)
		p.batch = make(batch)
	}
	p.mtx.Unlock()
}
