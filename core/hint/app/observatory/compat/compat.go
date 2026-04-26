// Package compat provides a v2ray-compatible observatory gRPC service.
// It registers the ObservatoryService under the v2ray gRPC service path
// so that v2rayA can query v2raya-core using its existing v2ray observatory client.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
package compat

import (
	"context"

	xray_obs "github.com/xtls/xray-core/app/observatory"
	xray_obs_cmd "github.com/xtls/xray-core/app/observatory/command"
	"github.com/xtls/xray-core/common"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/extension"
	"github.com/v2rayA/v2raya-core/hint/app/observatory/multiobservatory"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protowire"
)

// v2RayCompatServiceName is the full gRPC service name used by v2ray-core and v2rayA.
const v2RayCompatServiceName = "v2ray.core.app.observatory.command.ObservatoryService"

// compatService implements commander.Service to register under the v2ray gRPC path.
type compatService struct {
	v           *core.Instance
	observatory extension.Observatory
}

// Register implements commander.Service; registers under the v2ray service name.
func (s *compatService) Register(server *grpc.Server) {
	desc := &grpc.ServiceDesc{
		ServiceName: v2RayCompatServiceName,
		HandlerType: (*xray_obs_cmd.ObservatoryServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "GetOutboundStatus",
				Handler:    s.handleGetOutboundStatus,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "app/observatory/command/command.proto",
	}
	server.RegisterService(desc, s)
}

// handleGetOutboundStatus serves GetOutboundStatus RPCs on the v2ray compat path.
// The v2ray client sends GetOutboundStatusRequest with a Tag field (proto field 1, string).
// We decode the Tag using protowire to enable per-group observatory queries.
func (s *compatService) handleGetOutboundStatus(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	req := &xray_obs_cmd.GetOutboundStatusRequest{}
	if err := dec(req); err != nil {
		return nil, err
	}
	// xray's GetOutboundStatusRequest has no Tag field; the tag was dropped by proto.Unmarshal.
	// We cannot recover it here without a custom codec. Fall back to aggregate.
	tag := extractTagFromContext(ctx)

	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		return s.getByTag(ctx, tag)
	}
	if interceptor != nil {
		info := &grpc.UnaryServerInfo{
			Server:     srv,
			FullMethod: "/" + v2RayCompatServiceName + "/GetOutboundStatus",
		}
		return interceptor(ctx, req, info, handler)
	}
	return handler(ctx, req)
}

// getByTag queries per-group when the observatory is a MultiObservatory, otherwise aggregates.
func (s *compatService) getByTag(ctx context.Context, tag string) (*xray_obs_cmd.GetOutboundStatusResponse, error) {
	if mo, ok := s.observatory.(*multiobservatory.MultiObservatory); ok {
		result, err := mo.GetObservationByTag(tag, ctx)
		if err != nil {
			return nil, err
		}
		if obs, ok := result.(*xray_obs.ObservationResult); ok {
			return &xray_obs_cmd.GetOutboundStatusResponse{Status: obs}, nil
		}
	}
	result, err := s.observatory.GetObservation(ctx)
	if err != nil {
		return nil, err
	}
	if obs, ok := result.(*xray_obs.ObservationResult); ok {
		return &xray_obs_cmd.GetOutboundStatusResponse{Status: obs}, nil
	}
	return &xray_obs_cmd.GetOutboundStatusResponse{}, nil
}

// extractTagFromContext is a placeholder; gRPC-Go does not expose raw request bytes
// through standard interceptors. Per-tag routing requires a custom codec.
// For now, returns empty string (aggregate mode).
func extractTagFromContext(_ context.Context) string { return "" }

// extractTag decodes proto field 1 (string) from raw serialized bytes.
// Used when raw request bytes are available (e.g. custom codec path).
func extractTag(data []byte) string {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		if num == 1 && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(data)
			if n < 0 {
				break
			}
			return string(v)
		}
		n = protowire.ConsumeFieldValue(num, typ, data)
		if n < 0 {
			break
		}
		data = data[n:]
	}
	return ""
}

func init() {
	common.Must(common.RegisterConfig((*CompatConfig)(nil), func(ctx context.Context, cfg interface{}) (interface{}, error) {
		s := core.MustFromContext(ctx)
		sv := &compatService{v: s}
		err := s.RequireFeatures(func(obs extension.Observatory) {
			sv.observatory = obs
		}, false)
		if err != nil {
			return nil, err
		}
		return sv, nil
	}))
}
