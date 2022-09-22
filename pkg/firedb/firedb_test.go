package firedb

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"

	schemav1 "github.com/grafana/fire/pkg/firedb/schemas/v1"
	commonv1 "github.com/grafana/fire/pkg/gen/common/v1"
	googlev1 "github.com/grafana/fire/pkg/gen/google/v1"
	ingestv1 "github.com/grafana/fire/pkg/gen/ingester/v1"
	"github.com/grafana/fire/pkg/gen/ingester/v1/ingesterv1connect"
	pushv1 "github.com/grafana/fire/pkg/gen/push/v1"
	"github.com/grafana/fire/pkg/iter"
	firemodel "github.com/grafana/fire/pkg/model"
	"github.com/grafana/fire/pkg/testhelper"
)

func TestCreateLocalDir(t *testing.T) {
	dataPath := t.TempDir()
	localFile := dataPath + "/local"
	require.NoError(t, ioutil.WriteFile(localFile, []byte("d"), 0o644))
	_, err := New(&Config{
		DataPath:      dataPath,
		BlockDuration: 30 * time.Minute,
	}, log.NewNopLogger(), nil)
	require.Error(t, err)
	require.NoError(t, os.Remove(localFile))
	_, err = New(&Config{
		DataPath:      dataPath,
		BlockDuration: 30 * time.Minute,
	}, log.NewNopLogger(), nil)
	require.NoError(t, err)
}

var cpuProfileGenerator = func(tsNano int64, t testing.TB) (*googlev1.Profile, string) {
	p := parseProfile(t, "testdata/profile")
	p.TimeNanos = tsNano
	return p, "process_cpu"
}

func ingestProfiles(b testing.TB, db *FireDB, generator func(tsNano int64, t testing.TB) (*googlev1.Profile, string), from, to int64, step time.Duration, externalLabels ...*commonv1.LabelPair) {
	b.Helper()
	for i := from; i <= to; i += int64(step) {
		p, name := generator(i, b)
		require.NoError(b, db.Head().Ingest(
			context.Background(), p, uuid.New(), append(externalLabels, &commonv1.LabelPair{Name: model.MetricNameLabel, Value: name})...))
	}
}

func BenchmarkDBSelectProfile(b *testing.B) {
	var (
		testDir = b.TempDir()
		end     = time.Unix(0, int64(time.Hour))
		start   = end.Add(-time.Hour)
		step    = 15 * time.Second
	)

	db, err := New(&Config{
		DataPath:      testDir,
		BlockDuration: time.Duration(100000) * time.Minute, // we will manually flush
	}, log.NewNopLogger(), nil)
	require.NoError(b, err)

	ctx := context.Background()
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(16)
	for j := 0; j < 6; j++ {
		for i := 0; i < 200; i++ {
			func(i, j int) {
				g.Go(func() error {
					ingestProfiles(b, db, cpuProfileGenerator, start.UnixNano(), end.UnixNano(), step,
						&commonv1.LabelPair{Name: "namespace", Value: fmt.Sprintf("%d", j)},
						&commonv1.LabelPair{Name: "pod", Value: fmt.Sprintf("%d", i)},
					)
					return nil
				})
			}(i, j)
		}
	}
	require.NoError(b, g.Wait())
	require.NoError(b, db.Flush(ctx))
	db.runBlockQuerierSync(ctx)

	b.ResetTimer()
	b.ReportAllocs()

	benchmarkSelectProfile(`{}`, start, end, db, b)
	benchmarkSelectProfile(`{}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace="3"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace="3", pod="100"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace="3", pod=~".*1"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace!="3", pod=~".*1"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace=~"1|4"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace=~"1|4",pod=~"1.*"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
	benchmarkSelectProfile(`{namespace=~".*", pod=~"10|20|30"}`, start.Add(15*time.Minute), end.Add(-15*time.Minute), db, b)
}

func benchmarkSelectProfile(query string, start, end time.Time, db *FireDB, b *testing.B) {
	b.Helper()
	b.Run(query, func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			resp, err := db.SelectProfiles(context.Background(), connect.NewRequest(&ingestv1.SelectProfilesRequest{
				LabelSelector: query,
				Type:          mustParseProfileSelector(b, "process_cpu:cpu:nanoseconds:cpu:nanoseconds"),
				Start:         int64(model.TimeFromUnixNano(start.UnixNano())),
				End:           int64(model.TimeFromUnixNano(end.UnixNano())),
			}))
			require.NoError(b, err)
			b.Log(len(resp.Msg.Profiles))
		}
	})
}

func TestCloseFile(t *testing.T) {
	var (
		dataPath = t.TempDir()
		end      = time.Unix(0, int64(time.Hour))
		start    = end.Add(-time.Hour)
		step     = 15 * time.Second
	)
	db, err := New(&Config{
		DataPath:      dataPath,
		BlockDuration: 30 * time.Minute,
	}, log.NewNopLogger(), nil)
	require.NoError(t, err)
	require.NoError(t, db.StartAsync(context.Background()))
	require.NoError(t, db.AwaitRunning(context.Background()))

	ingestProfiles(t, db, cpuProfileGenerator, start.UnixNano(), end.UnixNano(), step)
	require.NoError(t, db.Flush(context.Background()))
	db.runBlockQuerierSync(context.Background())
	_, err = db.SelectProfiles(context.Background(), connect.NewRequest(&ingestv1.SelectProfilesRequest{
		LabelSelector: "{}",
		Type:          mustParseProfileSelector(t, "process_cpu:cpu:nanoseconds:cpu:nanoseconds"),
		Start:         int64(model.TimeFromUnixNano(start.UnixNano())),
		End:           int64(model.TimeFromUnixNano(end.UnixNano())),
	}))

	require.NoError(t, err)
	db.StopAsync()
	require.NoError(t, db.AwaitTerminated(context.Background()))
	require.NoError(t, os.RemoveAll(dataPath))
}

type fakeBidiServerMergeProfilesStacktraces struct {
	profilesSent []*ingestv1.ProfileSets
	keep         [][]bool
	t            *testing.T
}

func (f *fakeBidiServerMergeProfilesStacktraces) Send(resp *ingestv1.MergeProfilesStacktracesResponse) error {
	f.profilesSent = append(f.profilesSent, testhelper.CloneProto(f.t, resp.SelectedProfiles))
	return nil
}

func (f *fakeBidiServerMergeProfilesStacktraces) Receive() (*ingestv1.MergeProfilesStacktracesRequest, error) {
	res := &ingestv1.MergeProfilesStacktracesRequest{
		Profiles: f.keep[0],
	}
	f.keep = f.keep[1:]
	return res, nil
}

func (f *FireDB) ingesterClient() (ingesterv1connect.IngesterServiceClient, func()) {
	mux := http.NewServeMux()
	mux.Handle(ingesterv1connect.NewIngesterServiceHandler(&ingesterHandlerFireDB{f}))
	serv := testhelper.NewInMemoryServer(mux)

	var httpClient *http.Client = serv.Client()

	client := ingesterv1connect.NewIngesterServiceClient(
		httpClient, serv.URL(),
	)

	return client, serv.Close
}

type ingesterHandlerFireDB struct {
	*FireDB
}

func (i *ingesterHandlerFireDB) Push(context.Context, *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error) {
	return nil, errors.New("not implemented")
}

func (i *ingesterHandlerFireDB) LabelValues(context.Context, *connect.Request[ingestv1.LabelValuesRequest]) (*connect.Response[ingestv1.LabelValuesResponse], error) {
	return nil, errors.New("not implemented")
}

func (i *ingesterHandlerFireDB) LabelNames(context.Context, *connect.Request[ingestv1.LabelNamesRequest]) (*connect.Response[ingestv1.LabelNamesResponse], error) {
	return nil, errors.New("not implemented")
}

func (i *ingesterHandlerFireDB) ProfileTypes(context.Context, *connect.Request[ingestv1.ProfileTypesRequest]) (*connect.Response[ingestv1.ProfileTypesResponse], error) {
	return nil, errors.New("not implemented")
}

func (i *ingesterHandlerFireDB) Series(context.Context, *connect.Request[ingestv1.SeriesRequest]) (*connect.Response[ingestv1.SeriesResponse], error) {
	return nil, errors.New("not implemented")
}

func (i *ingesterHandlerFireDB) Flush(context.Context, *connect.Request[ingestv1.FlushRequest]) (*connect.Response[ingestv1.FlushResponse], error) {
	return nil, errors.New("not implemented")
}

func TestMergeProfilesStacktraces(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// ingest some sample data
	var (
		testDir = t.TempDir()
		end     = time.Unix(0, int64(time.Hour))
		start   = end.Add(-time.Minute)
		step    = 15 * time.Second
	)

	db, err := New(&Config{
		DataPath:      testDir,
		BlockDuration: time.Duration(100000) * time.Minute, // we will manually flush
	}, log.NewNopLogger(), nil)
	require.NoError(t, err)

	ingestProfiles(t, db, cpuProfileGenerator, start.UnixNano(), end.UnixNano(), step,
		&commonv1.LabelPair{Name: "namespace", Value: "my-namespace"},
		&commonv1.LabelPair{Name: "pod", Value: "my-pod"},
	)

	// create client
	ctx := context.Background()

	client, cleanup := db.ingesterClient()
	defer cleanup()

	t.Run("request the one existing series", func(t *testing.T) {
		bidi := client.MergeProfilesStacktraces(ctx)

		require.NoError(t, bidi.Send(&ingestv1.MergeProfilesStacktracesRequest{
			Request: &ingestv1.SelectProfilesRequest{
				LabelSelector: `{pod="my-pod"}`,
				Type:          mustParseProfileSelector(t, "process_cpu:cpu:nanoseconds:cpu:nanoseconds"),
				Start:         start.UnixMilli(),
				End:           end.UnixMilli(),
			},
		}))

		resp, err := bidi.Receive()
		require.NoError(t, err)
		require.Nil(t, resp.Result)
		require.Len(t, resp.SelectedProfiles.LabelsSets, 1)
		require.Len(t, resp.SelectedProfiles.Profiles, 5)

		require.NoError(t, bidi.Send(&ingestv1.MergeProfilesStacktracesRequest{
			Profiles: []bool{true},
		}))

		// expect empty resp to signal it is finished
		resp, err = bidi.Receive()
		require.NoError(t, err)
		require.Nil(t, resp.Result)

		// received result
		resp, err = bidi.Receive()
		require.NoError(t, err)
		require.NotNil(t, resp.Result)
		require.Len(t, resp.Result.Stacktraces, 48)
		require.Len(t, resp.Result.FunctionNames, 247)

	})

	t.Run("request non existing series", func(t *testing.T) {
		bidi := client.MergeProfilesStacktraces(ctx)

		require.NoError(t, bidi.Send(&ingestv1.MergeProfilesStacktracesRequest{
			Request: &ingestv1.SelectProfilesRequest{
				LabelSelector: `{pod="not-my-pod"}`,
				Type:          mustParseProfileSelector(t, "process_cpu:cpu:nanoseconds:cpu:nanoseconds"),
				Start:         start.UnixMilli(),
				End:           end.UnixMilli(),
			},
		}))

		// expect empty resp to signal it is finished
		resp, err := bidi.Receive()
		require.NoError(t, err)
		require.Nil(t, resp.Result)
		require.Nil(t, resp.SelectedProfiles)

		// still receiving a result
		resp, err = bidi.Receive()
		require.NoError(t, err)
		require.NotNil(t, resp.Result)
		require.Len(t, resp.Result.Stacktraces, 0)
		require.Len(t, resp.Result.FunctionNames, 0)
		require.Nil(t, resp.SelectedProfiles)
	})

	t.Run("empty request fails", func(t *testing.T) {
		bidi := client.MergeProfilesStacktraces(ctx)

		require.NoError(t, bidi.Send(&ingestv1.MergeProfilesStacktracesRequest{}))

		_, err := bidi.Receive()
		require.EqualError(t, err, "invalid_argument: missing initial select request")
	})

	t.Run("test cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		bidi := client.MergeProfilesStacktraces(ctx)
		require.NoError(t, bidi.Send(&ingestv1.MergeProfilesStacktracesRequest{
			Request: &ingestv1.SelectProfilesRequest{
				LabelSelector: `{pod="my-pod"}`,
				Type:          mustParseProfileSelector(t, "process_cpu:cpu:nanoseconds:cpu:nanoseconds"),
				Start:         start.UnixMilli(),
				End:           end.UnixMilli(),
			},
		}))
		cancel()
	})

	t.Run("test close request", func(t *testing.T) {
		bidi := client.MergeProfilesStacktraces(ctx)
		require.NoError(t, bidi.Send(&ingestv1.MergeProfilesStacktracesRequest{
			Request: &ingestv1.SelectProfilesRequest{
				LabelSelector: `{pod="my-pod"}`,
				Type:          mustParseProfileSelector(t, "process_cpu:cpu:nanoseconds:cpu:nanoseconds"),
				Start:         start.UnixMilli(),
				End:           end.UnixMilli(),
			},
		}))
		require.NoError(t, bidi.CloseRequest())
	})
}

func TestFilterProfiles(t *testing.T) {
	ctx := context.Background()
	profiles := lo.Times(11, func(i int) Profile {
		return ProfileWithLabels{
			Profile: &schemav1.Profile{TimeNanos: int64(i * int(time.Minute))},
			lbs:     firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", i)),
			fp:      model.Fingerprint(firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", i)).Hash()),
		}
	})
	in := iter.NewSliceIterator(profiles)
	bidi := &fakeBidiServerMergeProfilesStacktraces{
		keep: [][]bool{{}, {true}, {true}},
		t:    t,
	}
	filtered, err := filterProfiles(ctx, in, 5, bidi)
	require.NoError(t, err)
	require.Equal(t, 2, len(filtered))
	require.Equal(t, 3, len(bidi.profilesSent))
	testhelper.EqualProto(t, []*ingestv1.ProfileSets{
		{
			LabelsSets: lo.Times(5, func(i int) *commonv1.Labels {
				return &commonv1.Labels{Labels: firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", i))}
			}),
			Profiles: lo.Times(5, func(i int) *ingestv1.SeriesProfile {
				return &ingestv1.SeriesProfile{Timestamp: int64(model.TimeFromUnixNano(int64(i * int(time.Minute)))), LabelIndex: int32(i)}
			}),
		},
		{
			LabelsSets: lo.Times(5, func(i int) *commonv1.Labels {
				return &commonv1.Labels{Labels: firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", i+5))}
			}),
			Profiles: lo.Times(5, func(i int) *ingestv1.SeriesProfile {
				return &ingestv1.SeriesProfile{Timestamp: int64(model.TimeFromUnixNano(int64((i + 5) * int(time.Minute)))), LabelIndex: int32(i)}
			}),
		},
		{
			LabelsSets: lo.Times(1, func(i int) *commonv1.Labels {
				return &commonv1.Labels{Labels: firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", i+10))}
			}),
			Profiles: lo.Times(1, func(i int) *ingestv1.SeriesProfile {
				return &ingestv1.SeriesProfile{Timestamp: int64(model.TimeFromUnixNano(int64((i + 10) * int(time.Minute)))), LabelIndex: int32(i)}
			}),
		},
	}, bidi.profilesSent)

	require.Equal(t, []Profile{
		ProfileWithLabels{
			Profile: &schemav1.Profile{TimeNanos: int64(5 * int(time.Minute))},
			lbs:     firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", 5)),
			fp:      model.Fingerprint(firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", 5)).Hash()),
		},
		ProfileWithLabels{
			Profile: &schemav1.Profile{TimeNanos: int64(10 * int(time.Minute))},
			lbs:     firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", 10)),
			fp:      model.Fingerprint(firemodel.LabelsFromStrings("foo", "bar", "i", fmt.Sprintf("%d", 10)).Hash()),
		},
	}, filtered)
}