vote() {
  ID=$1
  yes 12345678 | injectived tx gov vote $ID yes \
  --home n0 --from node0 --keyring-backend file --gas 300000 --yes \
  --chain-id injective-1 --broadcast-mode sync

  yes 12345678 | injectived tx gov vote $ID yes \
  --home n0 --from node0 --keyring-backend file --gas 300000 --yes \
  --chain-id injective-1 --broadcast-mode sync
}

waitChronosService() {
  res=$(curl -sS localhost:5500 --output - 2>&1 | grep "Connection refused")
  while [[ $res == *"Connection refused"* ]]; do
    res=$(curl -sS localhost:5500 --output - 2>&1 | grep "Connection refused")
    echo "waiting chronos service..."
    sleep 1
  done
}

startServices() {
  BIN_DIR=$PWD

  cd ../../injective-price-oracle-legacy
  injective-price-oracle start > $BIN_DIR/logs/oracle.log 2>&1 &
  sleep 10

  cd ../injective-indexer
  make mongo-stop
  rm -rf var/
  ulimit -n 1000000
  make mongo > $BIN_DIR/logs/mongo.log 2>&1

  cd app/event-provider-process
  injective-indexer eventprovider-process > $BIN_DIR/logs/eventprovider-process.log 2>&1 &

  cd ../event-provider-api
  injective-indexer eventprovider-api > $BIN_DIR/logs/eventprovider-api.log 2>&1 &

  cd ../exchange-process
  injective-indexer exchange-process > $BIN_DIR/logs/exchange-process.log 2>&1 &

  cd ../chronos-process
  rm -rf var
  injective-indexer chronos-process > $BIN_DIR/logs/chronos-process.log 2>&1 &

  sleep 5
  cd ../chronos-api
  injective-indexer chronos-api > $BIN_DIR/logs/chronos-api.log 2>&1 &

  cd ../exchange-api
  injective-indexer exchange-api > $BIN_DIR/logs/exchange-api.log 2>&1 &

  cd ../../../injective-trading-bot
  injective-trading-bot start > $BIN_DIR/logs/trading-bot.log 2>&1 &

  cd ../injective-liquidator-bot
  injective-liquidator-bot start > $BIN_DIR/logs/liquidator-bot.log 2>&1 &
}

restartServices() {
  BIN_DIR=$PWD

  stopServices
  startServices
}

stopServices() {
  rm -rf logs/*
  for id in $(ps | grep exchange | cut -f1 -d ' '); do
     kill -9 $id
  done
  pkill injective-price-oracle
  pkill injective-trading-bot
  pkill injective-liquidator-bot
  cd -
  clear
}

nukeServices() {
  rm -rf logs/*
  for id in $(ps | grep injective-indexer | cut -f1 -d ' '); do
     kill -9 $id || true
  done
  pkill injective-price-oracle
  pkill injective-trading-bot
  pkill injective-liquidator-bot
  cd ../../injective-indexer
  rm -rf app/chronos-process/var

  make mongo-stop
  rm -rf var
  cd -
  clear
}
