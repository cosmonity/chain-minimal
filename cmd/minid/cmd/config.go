package cmd

import (
	"time"

	cmtcfg "github.com/cometbft/cometbft/config"

	serverv2 "go.cosmonity.xyz/evolve/server/v2"
	"go.cosmonity.xyz/evolve/server/v2/cometbft"
)

// initCometConfig helps to override default comet config template and configs.
func initCometConfig() cometbft.CfgOption {
	cfg := cmtcfg.DefaultConfig()
	cfg.Consensus.TimeoutCommit = 3 * time.Second // overwrite the block timeout
	cfg.LogLevel = "*:error,p2p:info,state:info"  // better default logging
	cfg.DBBackend = "goleveldb"                   // use goleveldb as the default backend

	return cometbft.OverwriteDefaultConfigTomlConfig(cfg)
}

// initServerConfig overwrites the server default app toml config.
func initServerConfig() serverv2.ServerConfig {
	serverCfg := serverv2.DefaultServerConfig()

	// overwrite the minimum gas price from the app configuration
	serverCfg.MinGasPrices = "0mini"

	return serverCfg
}
