package cmd

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"go.cosmonity.xyz/chain-minimal/app"

	"go.cosmonity.xyz/evolve/runtime/v2"
	serverv2 "go.cosmonity.xyz/evolve/server/v2"
	grpcserver "go.cosmonity.xyz/evolve/server/v2/api/grpc"
	"go.cosmonity.xyz/evolve/server/v2/api/grpcgateway"
	"go.cosmonity.xyz/evolve/server/v2/api/rest"
	"go.cosmonity.xyz/evolve/server/v2/cometbft"
	serverstore "go.cosmonity.xyz/evolve/server/v2/store"

	"cosmossdk.io/client/v2/offchain"
	coreserver "cosmossdk.io/core/server"
	"cosmossdk.io/core/transaction"
	"cosmossdk.io/log"
	confixcmd "cosmossdk.io/tools/confix/cmd"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/version"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	genutilv2cli "github.com/cosmos/cosmos-sdk/x/genutil/v2/cli"
)

// commandDependencies is a struct that contains all the dependencies needed to initialize the root command.
type commandDependencies[T transaction.Tx] struct {
	GlobalConfig  coreserver.ConfigMap
	TxConfig      client.TxConfig
	ModuleManager *runtime.MM[T]
	App           *app.MiniApp[T]
	ClientContext client.Context
}

func initRootCmd[T transaction.Tx](
	rootCmd *cobra.Command,
	logger log.Logger,
	deps commandDependencies[T],
) (serverv2.ConfigWriter, error) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	rootCmd.AddCommand(
		genutilcli.InitCmd(deps.ModuleManager),
		genesisCommand(deps.ModuleManager, deps.App),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		// add keybase, auxiliary RPC, query, genesis, and tx child commands
		queryCommand(),
		txCommand(),
		keys.Commands(),
		offchain.OffChain(),
		version.NewVersionCommand(),
	)

	// build CLI skeleton for initial config parsing or a client application invocation
	if deps.App == nil {
		return serverv2.AddCommands[T](
			rootCmd,
			logger,
			io.NopCloser(nil),
			deps.GlobalConfig,
			initServerConfig(),
			cometbft.NewWithConfigOptions[T](initCometConfig()),
			&grpcserver.Server[T]{},
			&serverstore.Server[T]{},
			&rest.Server[T]{},
			&grpcgateway.Server[T]{},
		)
	}

	// store component (not a server)
	storeComponent, err := serverstore.New[T](deps.App.Store(), deps.GlobalConfig)
	if err != nil {
		return nil, err
	}

	// rest component
	restServer, err := rest.New[T](logger, deps.App.AppManager, deps.GlobalConfig)
	if err != nil {
		return nil, err
	}

	// consensus component
	consensusServer, err := cometbft.New(
		logger,
		deps.App.Name(),
		deps.App.Store(),
		deps.App.AppManager,
		cometbft.AppCodecs[T]{
			AppCodec:              deps.App.AppCodec(),
			TxCodec:               &client.DefaultTxDecoder[T]{TxConfig: deps.TxConfig},
			LegacyAmino:           deps.ClientContext.LegacyAmino,
			ConsensusAddressCodec: deps.ClientContext.ConsensusAddressCodec,
		},
		deps.App.QueryHandlers(),
		deps.App.SchemaDecoderResolver(),
		cometbft.DefaultServerOptions[T](),
		deps.GlobalConfig,
	)
	if err != nil {
		return nil, err
	}

	grpcServer, err := grpcserver.New[T](
		logger,
		deps.App.InterfaceRegistry(),
		deps.App.QueryHandlers(),
		deps.App.Query,
		deps.GlobalConfig,
		grpcserver.WithExtraGRPCHandlers[T](
			consensusServer.GRPCServiceRegistrar(
				deps.ClientContext,
				deps.GlobalConfig,
			),
		),
	)
	if err != nil {
		return nil, err
	}

	grpcgatewayServer, err := grpcgateway.New[T](
		logger,
		deps.GlobalConfig,
		deps.App.InterfaceRegistry(),
		deps.App.AppManager,
	)
	if err != nil {
		return nil, err
	}
	registerGRPCGatewayRoutes(deps.ClientContext, grpcgatewayServer)

	// wire server commands
	return serverv2.AddCommands[T](
		rootCmd,
		logger,
		deps.App,
		deps.GlobalConfig,
		initServerConfig(),
		consensusServer,
		grpcServer,
		storeComponent,
		restServer,
		grpcgatewayServer,
	)
}

// genesisCommand builds genesis-related `simd genesis` command.
func genesisCommand[T transaction.Tx](
	moduleManager *runtime.MM[T],
	app *app.MiniApp[T],
) *cobra.Command {
	var genTxValidator func([]transaction.Msg) error
	if moduleManager != nil {
		genTxValidator = moduleManager.Modules()[genutiltypes.ModuleName].(genutil.AppModule).GenTxValidator()
	}
	cmd := genutilv2cli.Commands(
		genTxValidator,
		moduleManager,
		app,
	)

	return cmd
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
	)

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	return cmd
}

// registerGRPCGatewayRoutes registers the gRPC gateway routes for all modules and other components
func registerGRPCGatewayRoutes[T transaction.Tx](
	clientContext client.Context,
	server *grpcgateway.Server[T],
) {
	// those are the extra services that the CometBFT server implements (server/v2/cometbft/grpc.go)
	cmtservice.RegisterGRPCGatewayRoutes(clientContext, server.GRPCGatewayRouter)
	_ = nodeservice.RegisterServiceHandlerClient(context.Background(), server.GRPCGatewayRouter, nodeservice.NewServiceClient(clientContext))
	_ = txtypes.RegisterServiceHandlerClient(context.Background(), server.GRPCGatewayRouter, txtypes.NewServiceClient(clientContext))
}
