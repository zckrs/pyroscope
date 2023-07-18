// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: frontend/frontendpb/frontend.proto

package frontendpbconnect

import (
	context "context"
	errors "errors"
	connect_go "github.com/bufbuild/connect-go"
	frontendpb "github.com/grafana/pyroscope/pkg/frontend/frontendpb"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect_go.IsAtLeastVersion0_1_0

const (
	// FrontendForQuerierName is the fully-qualified name of the FrontendForQuerier service.
	FrontendForQuerierName = "frontendpb.FrontendForQuerier"
)

// FrontendForQuerierClient is a client for the frontendpb.FrontendForQuerier service.
type FrontendForQuerierClient interface {
	QueryResult(context.Context, *connect_go.Request[frontendpb.QueryResultRequest]) (*connect_go.Response[frontendpb.QueryResultResponse], error)
}

// NewFrontendForQuerierClient constructs a client for the frontendpb.FrontendForQuerier service. By
// default, it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses,
// and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the
// connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewFrontendForQuerierClient(httpClient connect_go.HTTPClient, baseURL string, opts ...connect_go.ClientOption) FrontendForQuerierClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &frontendForQuerierClient{
		queryResult: connect_go.NewClient[frontendpb.QueryResultRequest, frontendpb.QueryResultResponse](
			httpClient,
			baseURL+"/frontendpb.FrontendForQuerier/QueryResult",
			opts...,
		),
	}
}

// frontendForQuerierClient implements FrontendForQuerierClient.
type frontendForQuerierClient struct {
	queryResult *connect_go.Client[frontendpb.QueryResultRequest, frontendpb.QueryResultResponse]
}

// QueryResult calls frontendpb.FrontendForQuerier.QueryResult.
func (c *frontendForQuerierClient) QueryResult(ctx context.Context, req *connect_go.Request[frontendpb.QueryResultRequest]) (*connect_go.Response[frontendpb.QueryResultResponse], error) {
	return c.queryResult.CallUnary(ctx, req)
}

// FrontendForQuerierHandler is an implementation of the frontendpb.FrontendForQuerier service.
type FrontendForQuerierHandler interface {
	QueryResult(context.Context, *connect_go.Request[frontendpb.QueryResultRequest]) (*connect_go.Response[frontendpb.QueryResultResponse], error)
}

// NewFrontendForQuerierHandler builds an HTTP handler from the service implementation. It returns
// the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewFrontendForQuerierHandler(svc FrontendForQuerierHandler, opts ...connect_go.HandlerOption) (string, http.Handler) {
	mux := http.NewServeMux()
	mux.Handle("/frontendpb.FrontendForQuerier/QueryResult", connect_go.NewUnaryHandler(
		"/frontendpb.FrontendForQuerier/QueryResult",
		svc.QueryResult,
		opts...,
	))
	return "/frontendpb.FrontendForQuerier/", mux
}

// UnimplementedFrontendForQuerierHandler returns CodeUnimplemented from all methods.
type UnimplementedFrontendForQuerierHandler struct{}

func (UnimplementedFrontendForQuerierHandler) QueryResult(context.Context, *connect_go.Request[frontendpb.QueryResultRequest]) (*connect_go.Response[frontendpb.QueryResultResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("frontendpb.FrontendForQuerier.QueryResult is not implemented"))
}
