source ./util.sh

# ocr
PROPOSAL_ID=7
yes 12345678 | ./3-1-injectived tx gov submit-proposal param-change ./proposals/ocr.json \
--home n0 --from node0 --keyring-backend file --yes \
--chain-id injective-1 --broadcast-mode sync
vote $PROPOSAL_ID
sleep 5

export SIGNER1=$(yes 12345678 | ./3-1-injectived keys show -a signer1 --home n0)
export SIGNER2=$(yes 12345678 | ./3-1-injectived keys show -a signer2 --home n0)
export SIGNER3=$(yes 12345678 | ./3-1-injectived keys show -a signer3 --home n0)
export SIGNER4=$(yes 12345678 | ./3-1-injectived keys show -a signer4 --home n0)
export SIGNER5=$(yes 12345678 | ./3-1-injectived keys show -a signer5 --home n0)
export FEEDADMIN=$(yes 12345678 | ./3-1-injectived keys show -a ocrfeedadmin --home n0)

yes 12345678 | ./3-1-injectived tx chainlink create-feed \
--feed-id="BTC/USDT" \
--signers="$SIGNER1,$SIGNER2,$SIGNER3,$SIGNER4,$SIGNER5" \
--transmitters="$SIGNER1,$SIGNER2,$SIGNER3,$SIGNER4,$SIGNER5" \
--f=1 \
--offchain-config-version=1 \
--offchain-config="A641132V" \
--onchain-config="AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAABNCYXLHTYIdptk0WJwAAA==" \
--min-answer="0.01" \
--max-answer="100.0" \
--link-per-observation="10" \
--link-per-transmission="20" \
--link-denom="peggy0x514910771AF9Ca656af840dff83E8264EcF986CA" \
--unique-reports=true \
--feed-config-description="BTC/USDT feed" \
--feed-admin=$FEEDADMIN  \
--billing-admin=$FEEDADMIN  \
--home=n0 \
--from=ocrfeedadmin \
--keyring-backend=file \
--chain-id=injective-1 \
--broadcast-mode=sync \
--yes
