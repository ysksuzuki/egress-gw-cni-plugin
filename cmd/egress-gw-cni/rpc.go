package main

import (
	"context"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/cnirpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// makeCNIArgs creates *CNIArgs.
func makeCNIArgs(args *skel.CmdArgs) (*cnirpc.CNIArgs, error) {
	env := &PluginEnvArgs{}
	if err := types.LoadArgs(args.Args, env); err != nil {
		return nil, types.NewError(types.ErrInvalidEnvironmentVariables, "failed to load CNI_ARGS", err.Error())
	}

	cniArgs := &cnirpc.CNIArgs{
		ContainerId: args.ContainerID,
		Netns:       args.Netns,
		Ifname:      args.IfName,
		Args:        env.Map(),
		Path:        args.Path,
		StdinData:   args.StdinData,
	}
	return cniArgs, nil
}

// connect connects to egress-gw-agent
func connect(sock string) (*grpc.ClientConn, error) {
	dialer := &net.Dialer{}
	dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", a)
	}
	conn, err := grpc.Dial(sock, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialFunc))
	if err != nil {
		return nil, types.NewError(types.ErrTryAgainLater, "failed to connect to "+sock, err.Error())
	}
	return conn, nil
}

// convertError turns err returned from gRPC library into CNI's types.Error
func convertError(err error) error {
	st := status.Convert(err)
	details := st.Details()
	if len(details) != 1 {
		return types.NewError(types.ErrInternal, st.Message(), err.Error())
	}

	cniErr, ok := details[0].(*cnirpc.CNIError)
	if !ok {
		types.NewError(types.ErrInternal, st.Message(), err.Error())
	}

	return types.NewError(uint(cniErr.Code), cniErr.Msg, cniErr.Details)
}
