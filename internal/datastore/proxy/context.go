package proxy

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	log "github.com/authzed/spicedb/internal/logging"
	"github.com/authzed/spicedb/pkg/datastore"
	"github.com/authzed/spicedb/pkg/datastore/options"
	core "github.com/authzed/spicedb/pkg/proto/core/v1"
)

// SeparateContextWithTracing is a utility method which allows for severing the
// context between grpc and the datastore to prevent context cancellation from
// killing database connections that should otherwise go back to the connection
// pool.
func SeparateContextWithTracing(ctx context.Context) context.Context {
	span := trace.SpanFromContext(ctx)
	ctxWithObservability := trace.ContextWithSpan(context.Background(), span)

	loggerFromContext := log.Ctx(ctx)
	if loggerFromContext != nil {
		ctxWithObservability = loggerFromContext.WithContext(ctxWithObservability)
	}

	return ctxWithObservability
}

// NewSeparatingContextDatastoreProxy severs any timeouts in the context being
// passed to the datastore and only retains tracing metadata.
//
// This is useful for datastores that do not want to close connections when a
// cancel or deadline occurs.
func NewSeparatingContextDatastoreProxy(d datastore.Datastore) datastore.Datastore {
	return &ctxProxy{d}
}

type ctxProxy struct{ delegate datastore.Datastore }

func (p *ctxProxy) ReadWriteTx(ctx context.Context, f datastore.TxUserFunc) (datastore.Revision, error) {
	return p.delegate.ReadWriteTx(ctx, f)
}

func (p *ctxProxy) OptimizedRevision(ctx context.Context) (datastore.Revision, error) {
	return p.delegate.OptimizedRevision(SeparateContextWithTracing(ctx))
}

func (p *ctxProxy) CheckRevision(ctx context.Context, revision datastore.Revision) error {
	return p.delegate.CheckRevision(SeparateContextWithTracing(ctx), revision)
}

func (p *ctxProxy) HeadRevision(ctx context.Context) (datastore.Revision, error) {
	return p.delegate.HeadRevision(SeparateContextWithTracing(ctx))
}

func (p *ctxProxy) RevisionFromString(serialized string) (datastore.Revision, error) {
	return p.delegate.RevisionFromString(serialized)
}

func (p *ctxProxy) Watch(ctx context.Context, afterRevision datastore.Revision) (<-chan *datastore.RevisionChanges, <-chan error) {
	return p.delegate.Watch(ctx, afterRevision)
}

func (p *ctxProxy) Features(ctx context.Context) (*datastore.Features, error) {
	return p.delegate.Features(SeparateContextWithTracing(ctx))
}

func (p *ctxProxy) Statistics(ctx context.Context) (datastore.Stats, error) {
	return p.delegate.Statistics(SeparateContextWithTracing(ctx))
}

func (p *ctxProxy) ReadyState(ctx context.Context) (datastore.ReadyState, error) {
	return p.delegate.ReadyState(SeparateContextWithTracing(ctx))
}

func (p *ctxProxy) Close() error { return p.delegate.Close() }

func (p *ctxProxy) SnapshotReader(rev datastore.Revision) datastore.Reader {
	delegateReader := p.delegate.SnapshotReader(rev)
	return &ctxReader{delegateReader}
}

func (p *ctxProxy) Unwrap() datastore.Datastore {
	return p.delegate
}

type ctxReader struct{ delegate datastore.Reader }

func (r *ctxReader) ReadCaveatByName(ctx context.Context, name string) (*core.CaveatDefinition, datastore.Revision, error) {
	return r.delegate.ReadCaveatByName(SeparateContextWithTracing(ctx), name)
}

func (r *ctxReader) ListAllCaveats(ctx context.Context) ([]datastore.RevisionedCaveat, error) {
	return r.delegate.ListAllCaveats(SeparateContextWithTracing(ctx))
}

func (r *ctxReader) LookupCaveatsWithNames(ctx context.Context, caveatNames []string) ([]datastore.RevisionedCaveat, error) {
	return r.delegate.LookupCaveatsWithNames(SeparateContextWithTracing(ctx), caveatNames)
}

func (r *ctxReader) ListAllNamespaces(ctx context.Context) ([]datastore.RevisionedNamespace, error) {
	return r.delegate.ListAllNamespaces(SeparateContextWithTracing(ctx))
}

func (r *ctxReader) LookupNamespacesWithNames(ctx context.Context, nsNames []string) ([]datastore.RevisionedNamespace, error) {
	return r.delegate.LookupNamespacesWithNames(SeparateContextWithTracing(ctx), nsNames)
}

func (r *ctxReader) ReadNamespaceByName(ctx context.Context, nsName string) (*core.NamespaceDefinition, datastore.Revision, error) {
	return r.delegate.ReadNamespaceByName(SeparateContextWithTracing(ctx), nsName)
}

func (r *ctxReader) QueryRelationships(ctx context.Context, filter datastore.RelationshipsFilter, options ...options.QueryOptionsOption) (datastore.RelationshipIterator, error) {
	return r.delegate.QueryRelationships(SeparateContextWithTracing(ctx), filter, options...)
}

func (r *ctxReader) ReverseQueryRelationships(ctx context.Context, subjectsFilter datastore.SubjectsFilter, options ...options.ReverseQueryOptionsOption) (datastore.RelationshipIterator, error) {
	return r.delegate.ReverseQueryRelationships(SeparateContextWithTracing(ctx), subjectsFilter, options...)
}

var (
	_ datastore.Datastore = (*ctxProxy)(nil)
	_ datastore.Reader    = (*ctxReader)(nil)
)
