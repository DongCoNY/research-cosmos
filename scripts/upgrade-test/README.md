# Preparation
1. Install cosmovisor
```
go install github.com/cosmos/cosmos-sdk/cosmovisor/cmd/cosmovisor@v0.1.0
```

1. Download binaries
```
# 10001-rc7
cd upgrade-test
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.0.1-1635956190/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 1-7-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10002-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.1.0-1636178708/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 2-1-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10002-rc2
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.1.1-1636733798/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 2-2-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10003-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.1.1-1640627705/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 3-1-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10004-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.4.0-1645352045/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 4-1-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10005-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.5.0-1649280277/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 5-1-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10006-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.6.0-1657048292/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 6-1-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10007-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.7.0-1662223156/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 7-1-injectived && rm darwin-amd64.zip injective-exchange peggo

# 10008-rc1
wget https://github.com/InjectiveLabs/injective-chain-releases/releases/download/v1.8.0-1668679102/darwin-amd64.zip
unzip darwin-amd64.zip
mv injectived 8-1-injectived && rm darwin-amd64.zip injective-exchange peggo
```

1. Branches
```
cd injective-indexer && git checkout dev && git pull && make install
cd injective-price-oracle-legacy && git checkout master && git pull && make install
cd injective-trading-bot && git checkout master && git pull && make install
cd injective-liquidator-bot && git checkout master && git pull && make install
cd injective-dex && git checkout dev && git pull && yarn
```

2. Env config
* injective-indexer
```
EXCHANGE_ENV="local"
EXCHANGE_LOG_LEVEL="warn"
EXCHANGE_SERVICE_WAIT_TIMEOUT="1m"
EXCHANGE_COSMOS_CHAIN_ID="injective-1"
EXCHANGE_COSMOS_GRPC="tcp://localhost:9900"
EXCHANGE_TENDERMINT_RPC="http://localhost:26657"
EXCHANGE_FEE_PAYER_PK=FA785F9C479E305980646E75DD8C450DA7DF0B64EFE7BA1A0B076E528133E425
EXCHANGE_ALCHEMY_ENDPOINT="https://eth-mainnet.alchemyapi.io/v2/vSqxSDsS7fSB0VNQfWC1r0yVq5QCTy_n"
EXCHANGE_DB_MONGO_CONNECTION="mongodb://127.0.0.1:27017"
EXCHANGE_DB_MONGO_DBNAME="exchange"
EXCHANGE_DB_ARCHIVE_EVENTS="true"
EXCHANGE_BLOCK_FETCH_JOBS=1
EXCHANGE_GRPC_LISTEN_ADDR="0.0.0.0:9910"
EXCHANGE_HTTP_LISTEN_ADDR="0.0.0.0:4444"
EXCHANGE_HTTP_TLS_CERT=cmd/injective-exchange/cert/public.crt
EXCHANGE_HTTP_TLS_KEY=cmd/injective-exchange/cert/private.key
EXCHANGE_CHRONOS_DATA_PATH="var/data/chronos"
EXCHANGE_CHRONOS_RPC_ADDR="tcp://0.0.0.0:5500"
EXCHANGE_CHRONOS_BLOCK_OFFSET=0
EXCHANGE_CHAIN_START_HEIGHT=0
EXCHANGE_STATSD_PREFIX="exchange"
EXCHANGE_STATSD_ADDR="localhost:8125"
EXCHANGE_STATSD_STUCK_DUR="5m"
EXCHANGE_STATSD_MOCKING="false"
EXCHANGE_STATSD_DISABLED="true"
# OPTIONAL env variables for testing purposes
VAL0_MNEMONIC="remember huge castle bottom apology smooth avocado ceiling tent brief detect poem"
VAL1_MNEMONIC="capable dismiss rice income open wage unveil left veteran treat vast brave"
VAL2_MNEMONIC="jealous wrist abstract enter erupt hunt victory interest aim defy camp hair"
USER0_MNEMONIC="divide report just assist salad peanut depart song voice decide fringe stumble"
USER1_MNEMONIC="physical page glare junk return scale subject river token door mirror title"
TEST_KEY_ORACLE=a7202c23be1dd1931808b3208f88c1cfa8b4f398f7b607198fdb6ba26634052b
TEST_KEY_ALBERT=51429ecfc443b42b5c8ab67f564cdebfd5128a980150d8a524cf5b7954db240c
TEST_KEY_BOJAN=5f1f154b9e2fad1ee73061c40e28f34dcbf0a9156352ea8e77075c310057a1dc
TEST_KEY_MARKUS=6d3a06923253ffcbc557fbfeda44f16f6c5282eb378ede4e8ed313faa4bbcb78
TEST_KEY_ERIC=410eca15c066c8b9cc191ba3ebeacf09beeab45a11e98bd7e785bdc494eb984a
TEST_KEY_MAXIM=bbe45ee971ca6347ae8f5d0057db87bc45226eeacc886a04744c27babc9444d5
TEST_KEY_ALEX=403a6d93a86b9c8e876408903de9588ef4b31763366bc56fbfdde006b846ed7d
TEST_KEY_MIRZA=0e78aa1b5461910b0dd929d29f4192a6a3214c545e0bc9b696eec35b5eb0c205
TEST_KEY_CHINESE_MAX=3928f3a3525bf930cd91e8a706ac77a03ef16e4f75abbe73c4929bbe0280bcd4
TEST_KEY_VENKATESH=adf5ed3a6e3477b25d76c12e6e057f794328f99174247191edf19d5bb2bbfe2b
TEST_KEY_NAM=b8f67ff46b32ab18446a6130ad19589faad1c727b940e412d854db0fb5533dd8
TEST_KEY_ANASTASIA=576f2e64cde8c56e54f1d11e31d5cdbeb5b3c834c1fd8df2168b933ca230dd91
TEST_KEY_ACHILLEAS=2398e85d3d90773abf530a571d2a87fa46a69bdd90c0abc3ba90e7d9bb8fbc4f
TEST_KEY_HANNAH=7f8d1b4d3c0238394cd6e52252bd305fd2c8ec40ad449614453f96b2b3a73fa1
TEST_KEY_VIVIAN=a214ff1293832e853a49a5126edcc4e864cbf2414b261b8898e55784b932f8b0
TEST_KEY_PEIYUN=edbde3a6da2165d23c7dbfa8a5159a1253cd1a39a210aadad43296aec1e37b48
TEST_KEY_POWEN=fafb1efedd7805bdbd883758323caae8c1648b38dbef1540141bb6e127ebf548
EXPLORER_EXCHANGE_ENDPOINT="https://dex.binance.org/api/v1"
EXPLORER_COINGECKO_ENDPOINT="https://api.coingecko.com/api/v3"
```
* injective-price-oracle-legacy
```
ORACLE_ENV="local"
ORACLE_LOG_LEVEL="info"
ORACLE_SERVICE_WAIT_TIMEOUT="1m"

ORACLE_COSMOS_CHAIN_ID=injective-1
ORACLE_COSMOS_GRPC="tcp://localhost:9900"
ORACLE_TENDERMINT_RPC="http://localhost:26657"
ORACLE_COSMOS_GAS_PRICES="500000000inj"

ORACLE_COSMOS_KEYRING="file"
ORACLE_COSMOS_KEYRING_DIR=
ORACLE_COSMOS_KEYRING_APP="injectived"
ORACLE_COSMOS_FROM=
ORACLE_COSMOS_FROM_PASSPHRASE=
ORACLE_COSMOS_PK=D873B6778109EB91930072A1F9B27D69BF7C21E868FD210AFB000C864BEB0388
ORACLE_COSMOS_USE_LEDGER=false

ORACLE_COINBASE_URL="https://api.pro.coinbase.com"
ORACLE_COINBASE_KEY=
ORACLE_COINBASE_SECRET=
ORACLE_COINBASE_PASSPHRASE=

ORACLE_BAND_ENABLED=false
ORACLE_BAND_URL="https://asia-rpc.bandchain.org"
ORACLE_BAND_ASK_COUNT=16
ORACLE_BAND_MIN_COUNT=10
ORACLE_BAND_SYMBOLS="BTC,ETH,BNB,DOT,LINK,CAKE,USDT,STX,SUSHI,MATIC,MIR,ANC,ADA,LINA,TRX,UNI,VET,XLM,XRP,YFI,XVS,INDEX,PERP,DPI,BSV,XMR,USDC,DASH,ZEC,ETC,WAVES,EWT,NXM,AMPL,DAI,TUSD,BAND,EGLD,ANT,NMR,PAX,LSK,LRC,HBAR,BAL,RUNE,YFII,LUNA,DCR,SC,ENJ,BUSD,OCEAN,RSR,SXP,BTG,BZRX,SRM,SNT,SOL,CKB,BNT,CRV,MANA,YFV,KAVA,TRB,REP,FTM,TOMO,ONE,WNXM,PAXG,WAN,SUSD,RLC,OXT,RVN,FNX,RENBTC,WBTC,DIA,BTM,IOTX,FET,JST,MCO,KMD,BTS,QKC,YAMV2,UOS,AKRO,HNT,HOT,KAI,OGN,WRX,KDA,ORN,STORJ"

ORACLE_EXTERNAL_PRICEFEED_ENABLED=false
ORACLE_BINANCE_URL="https://api.binance.com/api/v3"
ORACLE_COINGECKO_URL="https://api.coingecko.com/api/v3"
ORACLE_IEX_URL="https://cloud-sse.iexapis.com/stable"
ORACLE_IEX_KEY=
ORACLE_FTX_URL="https://ftx.com/api"
ORACLE_YEARN_TOOLS_URL="https://api.yearn.tools"
ORACLE_YFI_VAULT_ADDRESS="0x07A8fA2531aab1eA8D6a50E8a81069b370ed24BE"

ORACLE_STATSD_PREFIX="oracle."
ORACLE_STATSD_ADDR="localhost:8125"
ORACLE_STATSD_STUCK_DUR="5m"
ORACLE_STATSD_MOCKING=false
ORACLE_STATSD_DISABLED=false
```
* injective-trading-bot
```
TRADING_ENV="local"
TRADING_LOG_LEVEL="info"
TRADING_SERVICE_WAIT_TIMEOUT="1m"
TRADING_COSMOS_CHAIN_ID="injective-1"
TRADING_COSMOS_GRPC="tcp://localhost:9900"
TRADING_TENDERMINT_RPC="http://localhost:26657"
TRADING_COSMOS_GAS_PRICES="500000000inj"
TRADING_COSMOS_KEYRING="file"
TRADING_COSMOS_KEYRING_DIR=
TRADING_COSMOS_KEYRING_APP="injectived"
TRADING_COSMOS_FROM=
TRADING_COSMOS_FROM_PASSPHRASE=
TRADING_COSMOS_PK=784F937D99AF8C5B51755A25BA309274982654AB96CD12190CED99A7DDD841D0
TRADING_COSMOS_USE_LEDGER=false
TRADING_EXCHANGE_GRPC="tcp://localhost:9910"
TRADING_STATSD_PREFIX="trading."
TRADING_STATSD_ADDR="localhost:8125"
TRADING_STATSD_STUCK_DUR="5m"
TRADING_STATSD_MOCKING=false
TRADING_STATSD_DISABLED=false
```
* injective-liquidator-bot
```
LIQUIDATOR_ENV="local"
LIQUIDATOR_LOG_LEVEL="info"
LIQUIDATOR_SERVICE_WAIT_TIMEOUT="1m"
LIQUIDATOR_COSMOS_CHAIN_ID="injective-888"
LIQUIDATOR_COSMOS_GRPC="tcp://localhost:9900"
LIQUIDATOR_TENDERMINT_RPC="http://localhost:26657"
LIQUIDATOR_COSMOS_GAS_PRICES="500000000inj"
LIQUIDATOR_COSMOS_KEYRING="file"
LIQUIDATOR_COSMOS_KEYRING_DIR=
LIQUIDATOR_COSMOS_KEYRING_APP="injectived"
LIQUIDATOR_COSMOS_FROM=
LIQUIDATOR_COSMOS_FROM_PASSPHRASE=
LIQUIDATOR_COSMOS_PK=2D7E2CB09F879491F8E597DBF086BE0F99563314A478F9A8C89743CA5566186B
LIQUIDATOR_COSMOS_USE_LEDGER=false
LIQUIDATOR_EXCHANGE_GRPC="tcp://localhost:9910"
LIQUIDATOR_STATSD_PREFIX="liquidator."
LIQUIDATOR_STATSD_ADDR="localhost:8125"
LIQUIDATOR_STATSD_STUCK_DUR="5m"
LIQUIDATOR_STATSD_MOCKING=false
LIQUIDATOR_STATSD_DISABLED=false
```
* injective-dex
```
## Public
APP_NAME=
APP_BASE_URL=http://localhost:3000
APP_NETWORK=local
APP_CHAIN_ID=1
## Flags
META_TAGS_ENABLED=true
MAINTENANCE_ENABLED=false
METRICS_ENABLED=false
GEO_IP_RESTRICTIONS_ENABLED=false
TRANSFER_RESTRICTIONS_ENABLED=false
## Secret
APP_FEE_RECIPIENT=inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz
APP_GOOGLE_ANALYTICS_KEY=
APP_GOOGLE_SITE_VERIFICATION_KEY=
APP_BUGSNAG_KEY=
APP_ALCHEMY_KEY=
APP_ALCHEMY_KOVAN_KEY=
```

# Mainnet upgrade replication 1
Attempt to simulate mainnet context with 3 binaries version. current branch is the next mainnet binary, built from `dev` branch.

1. Start injective-helix
```
cd injective-helix
yarn & yarn dev
```

1. Setup network with 2 nodes
```
cd upgrade-test
./setup.sh
./chores.sh cleanup
```

1. Start 2 nodes with cosmosvisor
```
DAEMON_NAME=injectived \
UNSAFE_SKIP_BACKUP=true \
DAEMON_POLL_INTERVAL=1000 \
DAEMON_HOME=${PWD}/n0/ \
cosmovisor --home n0 start --log-level warn

DAEMON_NAME=injectived \
UNSAFE_SKIP_BACKUP=true \
DAEMON_POLL_INTERVAL=1000 \
DAEMON_HOME=${PWD}/n1/ \
cosmovisor --home n1 start --log-level warn
```

1. Send proposals, you can copy and paste the whole block into terminal
```
# Generate markets and orders
./gov-0.sh

# Send upgrade proposal v1.1
./gov-1.sh

# Trigger state change for fee discount, trading reward
./gov-2.sh

# Send upgrade proposal v1.2
./gov-3.sh

# Send upgrade proposal v1.3
./gov-4.sh

# Trigger state change for ocr
./gov-5.sh

# Send upgrade proposal v1.4
./gov-6.sh

# Send upgrade proposal v1.5
./gov-7.sh

# Send upgrade proposal v1.6
./gov-8.sh

# Send upgrade proposal v1.7
./gov-9.sh
```

1. Let the chain runs for a while and confirm exchange services, oracle, bots all work.

1. Check lists
* snapshot
* export genesis
* breaking changes in this upgrade
* backend components' logs
```
# set snapshot-interval = 5 again if needed
open n0/config/app.toml
```
