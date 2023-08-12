package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	egressv1beta1 "github.com/ysksuzuki/egress-gw-cni-plugin/api/v1beta1"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/cnirpc"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/constants"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/founat"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// NewEgressGwAgent returns an implementation of cnirpc.CNIServer for egress-gw.
func NewEgressGwAgent(l net.Listener, mgr manager.Manager, egressPort int, logger *zap.Logger) manager.Runnable {
	return &egressGwAgent{
		listener:   l,
		apiReader:  mgr.GetAPIReader(),
		client:     mgr.GetClient(),
		egressPort: egressPort,
		logger:     logger,
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups="",resources=namespaces;services,verbs=get;list;watch
// +kubebuilder:rbac:groups=egress.ysuzuki.com,resources=egresses,verbs=get;list;watch

var grpcMetrics = grpc_prometheus.NewServerMetrics()

func init() {
	// register grpc_prometheus with controller-runtime's Registry
	metrics.Registry.MustRegister(grpcMetrics)
}

type egressGwAgent struct {
	cnirpc.UnimplementedCNIServer
	listener   net.Listener
	apiReader  client.Reader
	client     client.Client
	egressPort int
	logger     *zap.Logger
}

func (e *egressGwAgent) Start(ctx context.Context) error {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(
		grpc_middleware.ChainUnaryServer(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(fieldExtractor)),
			grpcMetrics.UnaryServerInterceptor(),
			grpc_zap.UnaryServerInterceptor(e.logger),
		),
	))
	cnirpc.RegisterCNIServer(grpcServer, e)

	// after all services are registered, initialize metrics.
	grpcMetrics.InitializeMetrics(grpcServer)

	// enable server reflection service
	// see https://github.com/grpc/grpc-go/blob/master/Documentation/server-reflection-tutorial.md
	reflection.Register(grpcServer)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(e.listener)
}

func fieldExtractor(fullMethod string, req interface{}) map[string]interface{} {
	args, ok := req.(*cnirpc.CNIArgs)
	if !ok {
		return nil
	}

	ret := make(map[string]interface{})
	if name, ok := args.Args[constants.PodNameKey]; ok {
		ret["pod.name"] = name
	}
	if namespace, ok := args.Args[constants.PodNamespaceKey]; ok {
		ret["pod.namespace"] = namespace
	}
	ret["netns"] = args.Netns
	ret["ifname"] = args.Ifname
	ret["container_id"] = args.ContainerId
	return ret
}

func newError(c codes.Code, cniCode cnirpc.ErrorCode, msg, details string) error {
	st := status.New(c, msg)
	st, err := st.WithDetails(&cnirpc.CNIError{Code: cniCode, Msg: msg, Details: details})
	if err != nil {
		panic(err)
	}

	return st.Err()
}

func newInternalError(err error, msg string) error {
	return newError(codes.Internal, cnirpc.ErrorCode_INTERNAL, msg, err.Error())
}

type PluginConf struct {
	types.NetConf

	// These are fields parsed out of the config or the environment;
	// included here for convenience
	ContainerID string    `json:"-"`
	ContIPv4    net.IPNet `json:"-"`
	ContIPv6    net.IPNet `json:"-"`
}

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte, ifName string) (*PluginConf, *current.Result, error) {
	conf := PluginConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	// Parse previous result.
	var result *current.Result
	if conf.RawPrevResult != nil {
		var err error
		if err = version.ParsePrevResult(&conf.NetConf); err != nil {
			return nil, nil, fmt.Errorf("could not parse prevResult: %v", err)
		}

		result, err = current.NewResultFromResult(conf.PrevResult)
		if err != nil {
			return nil, nil, fmt.Errorf("could not convert result to current version: %v", err)
		}
	}

	if conf.PrevResult != nil {
		for _, ip := range result.IPs {
			isIPv4 := ip.Address.IP.To4() != nil
			if !isIPv4 && conf.ContIPv6.IP != nil {
				continue
			} else if isIPv4 && conf.ContIPv4.IP != nil {
				continue
			}

			// Skip known non-sandbox interfaces
			if ip.Interface != nil {
				intIdx := *ip.Interface
				if intIdx >= 0 &&
					intIdx < len(result.Interfaces) &&
					(result.Interfaces[intIdx].Name != ifName ||
						result.Interfaces[intIdx].Sandbox == "") {
					continue
				}
			}
			if ip.Address.IP.To4() != nil {
				conf.ContIPv4 = ip.Address
			} else {
				conf.ContIPv6 = ip.Address
			}
		}
	}

	return &conf, result, nil
}

func (e *egressGwAgent) Add(ctx context.Context, args *cnirpc.CNIArgs) (*cnirpc.AddResponse, error) {
	logger := ctxzap.Extract(ctx)

	podName := args.Args[constants.PodNameKey]
	podNS := args.Args[constants.PodNamespaceKey]
	if podName == "" || podNS == "" {
		logger.Sugar().Errorw("missing pod name/namespace", "args", args.Args)
		return nil, newError(codes.InvalidArgument, cnirpc.ErrorCode_INVALID_ENVIRONMENT_VARIABLES,
			"missing pod name/namespace", fmt.Sprintf("%+v", args.Args))
	}

	pod := &corev1.Pod{}
	if err := e.apiReader.Get(ctx, client.ObjectKey{Namespace: podNS, Name: podName}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Errorw("pod not found", "name", podName, "namespace", podNS)
			return nil, newError(codes.NotFound, cnirpc.ErrorCode_UNKNOWN_CONTAINER, "pod not found", err.Error())
		}
		logger.Sugar().Errorw("failed to get pod", "name", podName, "namespace", podNS, "error", err)
		return nil, newInternalError(err, "failed to get pod")
	}

	g, err := e.getGWNets(ctx, pod)
	if err != nil {
		logger.Sugar().Errorw("failed to get egress GW", "error", err)
		return nil, newInternalError(err, "failed to get egress GW")
	}

	n, prevRes, err := parseConfig(args.StdinData, args.Ifname)
	if err != nil {
		return nil, newError(codes.InvalidArgument, cnirpc.ErrorCode_DECODING_FAILURE,
			"unable to parse CNI configuration", fmt.Sprintf("%+v", args.Args))
	}

	if g != nil {
		logger.Sugar().Info("enabling egress GW")
		err = ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
			if err := e.setupEgressGW(n.ContIPv4.IP, n.ContIPv6.IP, g, logger); err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			logger.Sugar().Errorw("failed to setup egress GW", "error", err)
			return nil, newInternalError(err, "failed to setup egress GW")
		}
	}

	data, err := json.Marshal(prevRes)
	if err != nil {
		logger.Sugar().Errorw("failed to marshal the result", "error", err)
		return nil, newInternalError(err, "failed to marshal the result")
	}
	return &cnirpc.AddResponse{Result: data}, nil
}

type GWNets struct {
	Gateway  net.IP
	Networks []*net.IPNet
}

func (e *egressGwAgent) setupEgressGW(ipv4, ipv6 net.IP, l []GWNets, log *zap.Logger) error {
	ft := founat.NewFoUTunnel(0, e.egressPort, ipv4, ipv6)
	if err := ft.Init(); err != nil {
		return err
	}

	cl := founat.NewNatClient(ipv4, ipv6, nil)
	if err := cl.Init(); err != nil {
		return err
	}

	for _, gwn := range l {
		link, err := ft.AddPeer(gwn.Gateway)
		if errors.Is(err, founat.ErrIPFamilyMismatch) {
			// ignore unsupported IP family link
			log.Sugar().Infow("ignored unsupported gateway", "gw", gwn.Gateway)
			continue
		}
		if err != nil {
			return err
		}
		if err := cl.AddEgress(link, gwn.Networks); err != nil {
			return err
		}
	}

	return nil
}

func (e *egressGwAgent) getGWNets(ctx context.Context, pod *corev1.Pod) ([]GWNets, error) {
	if pod.Spec.HostNetwork {
		// pods running in the host network cannot use egress NAT.
		// In fact, such a pod won't call CNI, so this is just a safeguard.
		return nil, nil
	}

	var egNames []client.ObjectKey

	for k, v := range pod.Annotations {
		if !strings.HasPrefix(k, constants.AnnEgressPrefix) {
			continue
		}

		ns := k[len(constants.AnnEgressPrefix):]
		for _, name := range strings.Split(v, ",") {
			egNames = append(egNames, client.ObjectKey{Namespace: ns, Name: name})
		}
	}
	if len(egNames) == 0 {
		return nil, nil
	}

	var gwlist []GWNets
	for _, n := range egNames {
		eg := &egressv1beta1.Egress{}
		svc := &corev1.Service{}

		if err := e.client.Get(ctx, n, eg); err != nil {
			return nil, newError(codes.FailedPrecondition, cnirpc.ErrorCode_INTERNAL,
				"failed to get Egress "+n.String(), err.Error())
		}
		if err := e.client.Get(ctx, n, svc); err != nil {
			return nil, newError(codes.FailedPrecondition, cnirpc.ErrorCode_INTERNAL,
				"failed to get Service "+n.String(), err.Error())
		}

		// as of k8s 1.19, dual stack Service is alpha and will be re-written
		// in 1.20.  So, we cannot use dual stack services.
		svcIP := net.ParseIP(svc.Spec.ClusterIP)
		if svcIP == nil {
			return nil, newError(codes.Internal, cnirpc.ErrorCode_INTERNAL,
				"invalid ClusterIP in Service "+n.String(), svc.Spec.ClusterIP)
		}
		var subnets []*net.IPNet
		if ip4 := svcIP.To4(); ip4 != nil {
			svcIP = ip4
			for _, sn := range eg.Spec.Destinations {
				_, subnet, err := net.ParseCIDR(sn)
				if err != nil {
					return nil, newInternalError(err, "invalid network in Egress "+n.String())
				}
				if subnet.IP.To4() != nil {
					subnets = append(subnets, subnet)
				}
			}
		} else {
			for _, sn := range eg.Spec.Destinations {
				_, subnet, err := net.ParseCIDR(sn)
				if err != nil {
					return nil, newInternalError(err, "invalid network in Egress "+n.String())
				}
				if subnet.IP.To4() == nil {
					subnets = append(subnets, subnet)
				}
			}
		}

		if len(subnets) > 0 {
			gwlist = append(gwlist, GWNets{Gateway: svcIP, Networks: subnets})
		}
	}

	return gwlist, nil
}

func (e *egressGwAgent) Del(ctx context.Context, args *cnirpc.CNIArgs) (*emptypb.Empty, error) {
	logger := ctxzap.Extract(ctx)

	// TODO
	logger.Sugar().Info("perform DEL")

	return &emptypb.Empty{}, nil
}

func (e *egressGwAgent) Check(ctx context.Context, args *cnirpc.CNIArgs) (*emptypb.Empty, error) {
	logger := ctxzap.Extract(ctx)

	// TODO
	logger.Sugar().Info("perform CHECK")

	return &emptypb.Empty{}, nil
}
