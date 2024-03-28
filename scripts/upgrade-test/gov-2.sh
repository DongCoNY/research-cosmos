source ./util.sh

# trading reward
PROPOSAL_ID=3
yes 12345678 | ./2-1-injectived tx exchange trading-reward-campaign-launch-proposal \
--proposal "./proposals/trading-reward-launch.json" \
--deposit 500000000000000000000inj \
--home n0 \
--from node0 \
--keyring-backend file \
--chain-id injective-1 \
--broadcast-mode sync \
--yes
vote $PROPOSAL_ID

# fee discount
PROPOSAL_ID=4
yes 12345678 | ./2-1-injectived tx exchange fee-discount-proposal \
--proposal="./proposals/fee-discount.json" \
--home n0 \
--deposit 500000000000000000000inj \
--from node0 \
--keyring-backend file \
--chain-id injective-1 \
--broadcast-mode sync \
--gas 500000 \
--yes
vote $PROPOSAL_ID
