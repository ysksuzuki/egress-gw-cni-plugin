package agent

import (
	"context"
	"fmt"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/cnirpc"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/constants"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NewEgressGwAgent returns an implementation of cnirpc.CNIServer for egress-gw.
func NewEgressGwAgent(l net.Listener, logger *zap.Logger) *egressGwAgent {
	return &egressGwAgent{
		listener: l,
		logger:   logger,
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups="",resources=namespaces;services,verbs=get;list;watch
// +kubebuilder:rbac:groups=egress.ysuzuki.com,resources=egresses,verbs=get;list;watch

var grpcMetrics = grpc_prometheus.NewServerMetrics()

func init() {
	// TODO: register grpc_prometheus with controller-runtime's Registry
}

type egressGwAgent struct {
	cnirpc.UnimplementedCNIServer
	listener net.Listener
	logger   *zap.Logger
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

func (e *egressGwAgent) Add(ctx context.Context, args *cnirpc.CNIArgs) (*cnirpc.AddResponse, error) {
	logger := ctxzap.Extract(ctx)

	podName := args.Args[constants.PodNameKey]
	podNS := args.Args[constants.PodNamespaceKey]
	if podName == "" || podNS == "" {
		logger.Sugar().Errorw("missing pod name/namespace", "args", args.Args)
		return nil, newError(codes.InvalidArgument, cnirpc.ErrorCode_INVALID_ENVIRONMENT_VARIABLES,
			"missing pod name/namespace", fmt.Sprintf("%+v", args.Args))
	}

	// TODO: set up fou tunnel
	//pod := &corev1.Pod{}
	//if err := s.apiReader.Get(ctx, client.ObjectKey{Namespace: podNS, Name: podName}, pod); err != nil {
	//	if apierrors.IsNotFound(err) {
	//		logger.Sugar().Errorw("pod not found", "name", podName, "namespace", podNS)
	//		return nil, newError(codes.NotFound, cnirpc.ErrorCode_UNKNOWN_CONTAINER, "pod not found", err.Error())
	//	}
	//	logger.Sugar().Errorw("failed to get pod", "name", podName, "namespace", podNS, "error", err)
	//	return nil, newInternalError(err, "failed to get pod")
	//}

	return &cnirpc.AddResponse{Result: nil}, nil
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
