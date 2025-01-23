# Mini - A minimal Cosmos SDK chain

This repository contains an example of a tiny, but working Cosmos SDK chain.
It uses the least modules possible and is intended to be used as a starting point for building your own chain, without all the boilerplate that other tools generate. It is a simpler version of Cosmos SDK's [simapp](https://github.com/cosmos/cosmos-sdk/tree/main/simapp).

`Minid` uses [Cosmos-SDK](https://github.com/cosmos/cosmos-sdk) `v0.52.x` with the `v2` core layer.

> [!NOTE]
> This is an example chain for the Cosmos SDK v2.
> Looking for the v0.52.x example? Check the [v0.52.x](https://github.com/cosmonity/chain-minimal/tree/v0.52.x) branch.
> Looking for the v0.50.x example? Check the [v0.50.x](https://github.com/cosmonity/chain-minimal/tree/v0.50.x) branch.

## How to use

In addition to learn how to build a chain thanks to `minid`, you can as well directly run `minid`.

### Prerequisites

* Install Go as described [here](https://go.dev/doc/install).
* Add `GOPATH` to your `PATH`:
  * `export PATH="$PATH:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin"`

You are all set!

### Installation

Install and run `minid`:

```sh
git clone git@github.com:cosmonity/chain-minimal.git
cd chain-minimal
make install # install the minid binary
make init # initialize the chain
minid start # start the chain
```

### Troubleshoot

After running `make install`, verify `minid` has been installed by doing `which minid`.
If `minid` is not found, verify that your `$PATH` is configured correctly.

## Useful links

* [Cosmos SDK v2 Community Fork](https://github.com/cosmonity/evolve)
* [Cosmos-SDK Documentation](https://docs.cosmos.network/)
