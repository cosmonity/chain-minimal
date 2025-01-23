package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.cosmonity.xyz/evolve/runtime/v2"
	serverv2 "go.cosmonity.xyz/evolve/server/v2"

	"go.cosmonity.xyz/chain-minimal/app"

	authv1 "cosmossdk.io/api/cosmos/auth/module/v1"
	stakingv1 "cosmossdk.io/api/cosmos/staking/module/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/registry"
	"cosmossdk.io/core/transaction"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtxconfig "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
)

func NewRootCmd[T transaction.Tx](args ...string) (*cobra.Command, error) {
	rootCommand := &cobra.Command{
		Use:           "minid",
		SilenceErrors: true,
	}
	configWriter, err := initRootCmd(rootCommand, log.NewNopLogger(), commandDependencies[T]{})
	if err != nil {
		return nil, err
	}
	factory, err := serverv2.NewCommandFactory(
		serverv2.WithConfigWriter(configWriter),
		serverv2.WithStdDefaultHomeDir(".minid"),
		serverv2.WithLoggerFactory(serverv2.NewLogger),
	)
	if err != nil {
		return nil, err
	}

	var autoCliOpts autocli.AppOptions
	if err := depinject.Inject(
		depinject.Configs(
			app.AppConfig(),
			depinject.Supply(runtime.GlobalConfig{}, log.NewNopLogger())),
		&autoCliOpts,
	); err != nil {
		return nil, err
	}

	if err = autoCliOpts.EnhanceRootCommand(rootCommand); err != nil {
		return nil, err
	}

	subCommand, configMap, logger, err := factory.ParseCommand(rootCommand, args)
	if err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return rootCommand, nil
		}
		return nil, err
	}

	var (
		moduleManager   *runtime.MM[T]
		clientCtx       client.Context
		miniApp         *app.MiniApp[T]
		depinjectConfig = depinject.Configs(
			depinject.Supply(logger, runtime.GlobalConfig(configMap)),
			depinject.Provide(ProvideClientContext),
		)
	)
	if serverv2.IsAppRequired(subCommand) {
		// server construction
		miniApp, err = app.New[T](depinjectConfig, &autoCliOpts, &moduleManager, &clientCtx)
		if err != nil {
			return nil, err
		}
	} else {
		// client construction
		if err = depinject.Inject(
			depinject.Configs(
				app.AppConfig(),
				depinjectConfig,
			),
			&autoCliOpts, &moduleManager, &clientCtx,
		); err != nil {
			return nil, err
		}
	}

	commandDeps := commandDependencies[T]{
		GlobalConfig:  configMap,
		TxConfig:      clientCtx.TxConfig,
		ModuleManager: moduleManager,
		App:           miniApp,
		ClientContext: clientCtx,
	}

	rootCommand = &cobra.Command{
		Use:           "minid",
		Short:         "minid - the minimal chain app",
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			clientCtx = clientCtx.WithCmdContext(cmd.Context())
			clientCtx, err := client.ReadPersistentCommandFlags(clientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			clientCtx, err = config.CreateClientConfig(clientCtx, "", nil)
			if err != nil {
				return err
			}

			if err = client.SetCmdClientContextHandler(clientCtx, cmd); err != nil {
				return err
			}

			return nil
		},
	}

	factory.EnhanceRootCommand(rootCommand)
	_, err = initRootCmd(rootCommand, logger, commandDeps)
	if err != nil {
		return nil, err
	}

	if err := autoCliOpts.EnhanceRootCommand(rootCommand); err != nil {
		return nil, err
	}

	return rootCommand, nil
}

// ProvideClientContext is a depinject Provider function which assembles and returns a client.Context.
func ProvideClientContext(
	configMap runtime.GlobalConfig,
	appCodec codec.Codec,
	interfaceRegistry codectypes.InterfaceRegistry,
	txConfigOpts tx.ConfigOptions,
	legacyAmino registry.AminoRegistrar,
	addressCodec address.Codec,
	validatorAddressCodec address.ValidatorAddressCodec,
	consensusAddressCodec address.ConsensusAddressCodec,
	authConfig *authv1.Module,
	stakingConfig *stakingv1.Module,

) client.Context {
	var err error
	amino, ok := legacyAmino.(*codec.LegacyAmino)
	if !ok {
		panic("registry.AminoRegistrar must be an *codec.LegacyAmino instance for legacy ClientContext")
	}

	homeDir, ok := configMap[serverv2.FlagHome].(string)
	if !ok {
		panic("server.ConfigMap must contain a string value for serverv2.FlagHome")
	}

	clientCtx := client.Context{}.
		WithCodec(appCodec).
		WithInterfaceRegistry(interfaceRegistry).
		WithLegacyAmino(amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithAddressCodec(addressCodec).
		WithValidatorAddressCodec(validatorAddressCodec).
		WithConsensusAddressCodec(consensusAddressCodec).
		WithHomeDir(homeDir).
		WithViper("MINI"). // env variable prefix
		WithAddressPrefix(authConfig.Bech32Prefix).
		WithValidatorPrefix(stakingConfig.Bech32PrefixValidator)

	clientCtx, err = config.CreateClientConfig(clientCtx, "", nil)
	if err != nil {
		panic(err)
	}

	// textual is enabled by default, we need to re-create the tx config grpc instead of bank keeper.
	txConfigOpts.TextualCoinMetadataQueryFn = authtxconfig.NewGRPCCoinMetadataQueryFn(clientCtx)
	txConfig, err := tx.NewTxConfigWithOptions(clientCtx.Codec, txConfigOpts)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithTxConfig(txConfig)

	return clientCtx
}
