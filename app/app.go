package app

import (
	_ "embed"
	"fmt"

	"cosmossdk.io/core/registry"
	"cosmossdk.io/core/server"
	"cosmossdk.io/core/transaction"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"cosmossdk.io/log"

	_ "cosmossdk.io/api/cosmos/tx/config/v1" // import for side-effects
	_ "cosmossdk.io/x/accounts"              // import for side-effects
	_ "cosmossdk.io/x/bank"                  // import for side-effects
	_ "cosmossdk.io/x/consensus"             // import for side-effects
	_ "cosmossdk.io/x/distribution"          // import for side-effects
	_ "cosmossdk.io/x/mint"                  // import for side-effects
	_ "cosmossdk.io/x/staking"               // import for side-effects
	stakingkeeper "cosmossdk.io/x/staking/keeper"

	"go.cosmonity.xyz/evolve/runtime/v2"
	serverstore "go.cosmonity.xyz/evolve/server/v2/store"
	"go.cosmonity.xyz/evolve/store/v2"
	"go.cosmonity.xyz/evolve/store/v2/commitment/iavlv2"
	"go.cosmonity.xyz/evolve/store/v2/root"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	_ "github.com/cosmos/cosmos-sdk/x/auth"           // import for side-effects
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config" // import for side-effects
	_ "github.com/cosmos/cosmos-sdk/x/validate"       // import for side-effects
)

//go:embed app.yaml
var AppConfigYAML []byte

// MiniApp is a minimalistic Cosmos SDK application.
type MiniApp[T transaction.Tx] struct {
	*runtime.App[T]
	legacyAmino       registry.AminoRegistrar
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry
	store             store.RootStore

	// keepers (only keepers that are needed in the app commands)
	StakingKeeper *stakingkeeper.Keeper
}

// AppConfig returns the default app config.
// Because core layer does not depend on the SDK anymore,
// more dependencies need to be added here (instead of being previously abstracted by runtime).
func AppConfig() depinject.Config {
	return depinject.Configs(
		appconfig.LoadYAML(AppConfigYAML),
		runtime.DefaultServiceBindings(),
		codec.DefaultProviders,
		depinject.Provide(
			ProvideRootStoreConfig,
		),
		depinject.Invoke(
			std.RegisterInterfaces,
			std.RegisterLegacyAminoCodec,
		),
	)
}

// New returns a reference to an initialized MiniApp.
func New[T transaction.Tx](
	config depinject.Config,
	outputs ...any,
) (*MiniApp[T], error) {
	var (
		app          = &MiniApp[T]{}
		appBuilder   *runtime.AppBuilder[T]
		logger       log.Logger
		storeBuilder root.Builder
	)

	// merge AppConfig and other configuration in one config
	appConfig := depinject.Configs(
		AppConfig(),
		config,
	)

	outputs = append(outputs,
		&logger,
		&storeBuilder,
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.StakingKeeper,
	)

	if err := depinject.Inject(appConfig, outputs...); err != nil {
		return nil, err
	}

	var err error
	app.App, err = appBuilder.Build()
	if err != nil {
		return nil, err
	}

	app.store = storeBuilder.Get()
	if app.store == nil {
		return nil, fmt.Errorf("store builder did not return a db")
	}

	if err = app.LoadLatest(); err != nil {
		return nil, err
	}

	return app, nil
}

// AppCodec returns MiniApp's codec.
func (app *MiniApp[T]) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns MiniApp's InterfaceRegistry.
func (app *MiniApp[T]) InterfaceRegistry() server.InterfaceRegistry {
	return app.interfaceRegistry
}

// LegacyAmino returns MiniApp's amino codec.
func (app *MiniApp[T]) LegacyAmino() registry.AminoRegistrar {
	return app.legacyAmino
}

// Store returns the root store.
func (app *MiniApp[T]) Store() store.RootStore {
	return app.store
}

// Close overwrites the base Close method to close the stores.
func (app *MiniApp[T]) Close() error {
	if err := app.store.Close(); err != nil {
		return err
	}

	return app.App.Close()
}

// ProvideRootStoreConfig provides the root store configuration.
// The config is being read in app.go instead of being read earlier.
func ProvideRootStoreConfig(config runtime.GlobalConfig) (*root.Config, error) {
	cfg, err := serverstore.UnmarshalConfig(config)
	if err != nil {
		return nil, err
	}
	cfg.Options.IavlV2Config = iavlv2.DefaultConfig()
	cfg.Options.IavlV2Config.MinimumKeepVersions = int64(cfg.Options.SCPruningOption.KeepRecent)
	iavlv2.SetGlobalPruneLimit(1)
	return cfg, err
}
