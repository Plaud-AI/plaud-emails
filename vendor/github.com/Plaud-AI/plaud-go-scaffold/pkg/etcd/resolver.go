package etcd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

// etcdResolverBuilder implements a per-dial resolver builder backed by this ServiceRegistry
type etcdResolverBuilder struct {
	registry *ServiceRegistry
}

func (b *etcdResolverBuilder) Scheme() string { return "etcd" }

func (b *etcdResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &etcdResolver{registry: b.registry, target: target, cc: cc, ctx: ctx, cancel: cancel}
	// Initial resolve
	if err := r.resolveNow(); err != nil {
		// Continue with empty addresses; Dial may block until available
		logger.Warnf("initial resolve failed for target %v: %v", target, err)
	}
	go r.watchLoop()
	return r, nil
}

// etcdResolver resolves serviceName -> list of addresses via etcd and watches for changes
type etcdResolver struct {
	registry *ServiceRegistry
	target   resolver.Target
	cc       resolver.ClientConn
	ctx      context.Context
	cancel   context.CancelFunc
}

func (r *etcdResolver) ResolveNow(resolver.ResolveNowOptions) { _ = r.resolveNow() }

func (r *etcdResolver) Close() { r.cancel() }

func (r *etcdResolver) serviceName() string {
	// Expect target.URL.Path like "/<service>"
	path := strings.TrimPrefix(r.target.URL.Path, "/")
	if path != "" {
		return path
	}
	// Fallback to Endpoint for older grpc target parsing
	if ep := r.target.Endpoint(); ep != "" {
		return ep
	}
	return r.target.URL.Host
}

func (r *etcdResolver) resolveNow() error {
	serviceName := r.serviceName()
	if serviceName == "" {
		return fmt.Errorf("empty service name in target: %v", r.target)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	services, err := r.registry.GetServices(ctx, serviceName)
	if err != nil {
		return err
	}
	addrs := make([]resolver.Address, 0, len(services))
	for _, s := range services {
		addr := fmt.Sprintf("%s:%d", s.Addr, s.Port)
		addrs = append(addrs, resolver.Address{Addr: addr})
	}
	return r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (r *etcdResolver) watchLoop() {
	serviceName := r.serviceName()
	key := r.registry.getServiceKey(serviceName)
	watchChan := r.registry.client.Watch(r.ctx, key, clientv3.WithPrefix())
	for {
		select {
		case <-r.ctx.Done():
			return
		case _, ok := <-watchChan:
			if !ok {
				return
			}
			// Re-resolve on any change
			if err := r.resolveNow(); err != nil {
				logger.Warnf("resolve failed on watch update for %s: %v", serviceName, err)
			}
		}
	}
}
