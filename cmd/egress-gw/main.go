package main

import (
	"context"
	"fmt"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	egressgw "github.com/ysksuzuki/egress-gw-cni-plugin"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/cnirpc"
)

const rpcTimeout = 1 * time.Minute

func cmdAdd(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	if conf.PrevResult == nil {
		return types.NewError(types.ErrInvalidNetworkConfig, "egress-gw must be called next to the cilium-cni", "")
	}

	cniArgs, err := makeCNIArgs(args)
	if err != nil {
		return err
	}

	conn, err := connect(conf.Socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := cnirpc.NewCNIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	resp, err := client.Add(ctx, cniArgs)
	if err != nil {
		return convertError(err)
	}

	result, err := current.NewResult(resp.Result)
	if err != nil {
		return types.NewError(types.ErrDecodingFailure, "failed to unmarshal result", err.Error())
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	cniArgs, err := makeCNIArgs(args)
	if err != nil {
		return err
	}

	conn, err := connect(conf.Socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := cnirpc.NewCNIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	if _, err = client.Del(ctx, cniArgs); err != nil {
		return convertError(err)
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	cniArgs, err := makeCNIArgs(args)
	if err != nil {
		return err
	}

	conn, err := connect(conf.Socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := cnirpc.NewCNIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	if _, err = client.Check(ctx, cniArgs); err != nil {
		return convertError(err)
	}

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.PluginSupports("0.3.1", "0.4.0", "1.0.0"),
		fmt.Sprintf("egress-gw-cni-plugin %s", egressgw.Version()))
}
