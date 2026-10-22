package oauthcleanup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	oauthModel "github.com/rendau/kusec/internal/domain/oauth/model"
)

type fakeOAuth struct {
	idle   []*oauthModel.Grant
	before time.Time
}

func (f *fakeOAuth) DeleteExpiredTokens(context.Context) (int64, error) {
	return 3, nil
}

func (f *fakeOAuth) DeleteIdleGrants(_ context.Context, createdBefore time.Time) ([]*oauthModel.Grant, error) {
	f.before = createdBefore
	return f.idle, nil
}

type fakeTxm struct{ calls int }

func (t *fakeTxm) TxFn(ctx context.Context, f func(context.Context) error) error {
	t.calls++
	return f(ctx)
}

type fakeAudit struct {
	deleted []string
	batches []string
}

func (a *fakeAudit) RecordOAuthGrant(_ context.Context, old, cur *oauthModel.Grant, batchId *string) error {
	if old != nil && cur == nil {
		a.deleted = append(a.deleted, old.Id)
		a.batches = append(a.batches, *batchId)
	}
	return nil
}

func (a *fakeAudit) NewBatchId() string {
	return "batch-1"
}

func TestCleanup(t *testing.T) {
	oauth := &fakeOAuth{idle: []*oauthModel.Grant{{Id: "g1"}, {Id: "g2"}}}
	txm := &fakeTxm{}
	audit := &fakeAudit{}

	New(oauth, txm, audit).Cleanup(context.Background())

	assert.Equal(t, 1, txm.calls)
	assert.Equal(t, []string{"g1", "g2"}, audit.deleted)
	assert.Equal(t, []string{"batch-1", "batch-1"}, audit.batches)
	assert.WithinDuration(t, time.Now().Add(-idleGrantTTL), oauth.before, time.Minute)
}

func TestCleanupNothingIdle(t *testing.T) {
	audit := &fakeAudit{}

	New(&fakeOAuth{}, &fakeTxm{}, audit).Cleanup(context.Background())

	assert.Empty(t, audit.deleted)
}
