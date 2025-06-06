package revisions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/authzed/spicedb/pkg/datastore"
	"github.com/authzed/spicedb/pkg/datastore/revision"
)

type trackingRevisionFunction struct {
	mock.Mock
}

func (m *trackingRevisionFunction) optimizedRevisionFunc(_ context.Context) (datastore.Revision, time.Duration, error) {
	args := m.Called()
	return args.Get(0).(revision.Decimal), args.Get(1).(time.Duration), args.Error(2)
}

var (
	one   = revision.NewFromDecimal(decimal.NewFromInt(1))
	two   = revision.NewFromDecimal(decimal.NewFromInt(2))
	three = revision.NewFromDecimal(decimal.NewFromInt(3))
)

func TestOptimizedRevisionCache(t *testing.T) {
	type revisionResponse struct {
		rev      datastore.Revision
		validFor time.Duration
	}

	testCases := []struct {
		name                  string
		maxStaleness          time.Duration
		expectedCallResponses []revisionResponse
		expectedRevisions     []datastore.Revision
	}{
		{
			"single request",
			0,
			[]revisionResponse{
				{one, 0},
			},
			[]datastore.Revision{one},
		},
		{
			"simple no caching request",
			0,
			[]revisionResponse{
				{one, 0},
				{two, 0},
				{three, 0},
			},
			[]datastore.Revision{one, two, three},
		},
		{
			"simple cached once",
			0,
			[]revisionResponse{
				{one, 7 * time.Millisecond},
				{two, 0},
			},
			[]datastore.Revision{one, one, two},
		},
		{
			"cached by staleness",
			7 * time.Millisecond,
			[]revisionResponse{
				{one, 0},
				{two, 0},
			},
			[]datastore.Revision{one, one, two, two},
		},
		{
			"cached by staleness and validity",
			2 * time.Millisecond,
			[]revisionResponse{
				{one, 4 * time.Millisecond},
				{two, 0},
				{three, 0},
			},
			[]datastore.Revision{one, one, two, three},
		},
		{
			"cached for a while",
			0,
			[]revisionResponse{
				{one, 28 * time.Millisecond},
				{two, 0},
			},
			[]datastore.Revision{one, one, one, one, one, one, two},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			or := NewCachedOptimizedRevisions(tc.maxStaleness)
			mockTime := clock.NewMock()
			or.clockFn = mockTime
			mock := trackingRevisionFunction{}
			or.SetOptimizedRevisionFunc(mock.optimizedRevisionFunc)

			for _, callSpec := range tc.expectedCallResponses {
				mock.On("optimizedRevisionFunc").Return(callSpec.rev, callSpec.validFor, nil).Once()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			for _, expectedRev := range tc.expectedRevisions {
				revision, err := or.OptimizedRevision(ctx)
				require.NoError(err)
				require.True(expectedRev.Equal(revision), "must return the proper revision %s != %s", expectedRev, revision)

				mockTime.Add(5 * time.Millisecond)
			}

			mock.AssertExpectations(t)
		})
	}
}

func TestOptimizedRevisionCacheSingleFlight(t *testing.T) {
	require := require.New(t)

	or := NewCachedOptimizedRevisions(0)
	mock := trackingRevisionFunction{}
	or.SetOptimizedRevisionFunc(mock.optimizedRevisionFunc)

	mock.
		On("optimizedRevisionFunc").
		Return(one, time.Duration(0), nil).
		After(50 * time.Millisecond).
		Once()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	g := errgroup.Group{}
	for i := 0; i < 10; i++ {
		g.Go(func() error {
			revision, err := or.OptimizedRevision(ctx)
			if err != nil {
				return err
			}
			require.True(one.Equal(revision), "must return the proper revision %s != %s", one, revision)
			return nil
		})
		time.Sleep(1 * time.Millisecond)
	}

	err := g.Wait()
	require.NoError(err)

	mock.AssertExpectations(t)
}

func TestSingleFlightError(t *testing.T) {
	req := require.New(t)

	or := NewCachedOptimizedRevisions(0)
	mock := trackingRevisionFunction{}
	or.SetOptimizedRevisionFunc(mock.optimizedRevisionFunc)

	mock.
		On("optimizedRevisionFunc").
		Return(one, time.Duration(0), errors.New("fail")).
		Once()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := or.OptimizedRevision(ctx)
	req.Error(err)
	mock.AssertExpectations(t)
}
