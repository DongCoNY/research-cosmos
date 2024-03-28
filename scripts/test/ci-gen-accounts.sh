HOME=$1
CHAINID=injective-1
MONIKER="injective"
PASSPHRASE="12345678"

echo "generating accounts..."

yes $PASSPHRASE | injectived keys add genesis --home $HOME
yes $PASSPHRASE | injectived add-genesis-account $(yes $PASSPHRASE | injectived keys show genesis -a --home $HOME) 1000000000000000000000000inj --home $HOME
# zero address account
yes $PASSPHRASE | injectived add-genesis-account inj1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqe2hm49 1inj --home $HOME

VAL_KEY="localkey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

USER1_KEY="user1"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

USER2_KEY="user2"
USER2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

USER3_KEY="user3"
USER3_MNEMONIC="keep liar demand upon shed essence tip undo eagle run people strong sense another salute double peasant egg royal hair report winner student diamond"

USER4_KEY="user4"
USER4_MNEMONIC="pony glide frown crisp unfold lawn cup loan trial govern usual matrix theory wash fresh address pioneer between meadow visa buffalo keep gallery swear"

NEWLINE=$'\n'

# Import keys from mnemonics
yes "$VAL_MNEMONIC$NEWLINE$PASSPHRASE" | injectived keys add $VAL_KEY --recover --home $HOME
yes "$USER1_MNEMONIC$NEWLINE$PASSPHRASE" | injectived keys add $USER1_KEY --recover --home $HOME
yes "$USER2_MNEMONIC$NEWLINE$PASSPHRASE" | injectived keys add $USER2_KEY --recover --home $HOME
yes "$USER3_MNEMONIC$NEWLINE$PASSPHRASE" | injectived keys add $USER3_KEY --recover --home $HOME
yes "$USER4_MNEMONIC$NEWLINE$PASSPHRASE" | injectived keys add $USER4_KEY --recover --home $HOME

# Allocate genesis accounts (cosmos formatted addresses)
yes $PASSPHRASE | injectived add-genesis-account $(yes $PASSPHRASE | injectived keys show $VAL_KEY -a --home $HOME) 1000000000000000000000000inj --home $HOME
yes $PASSPHRASE | injectived add-genesis-account $(yes $PASSPHRASE | injectived keys show $USER1_KEY -a --home $HOME) 1000000000000000000000000inj --home $HOME
yes $PASSPHRASE | injectived add-genesis-account $(yes $PASSPHRASE | injectived keys show $USER2_KEY -a --home $HOME) 1000000000000000000000000inj --home $HOME
yes $PASSPHRASE | injectived add-genesis-account $(yes $PASSPHRASE | injectived keys show $USER3_KEY -a --home $HOME) 1000000000000000000000000inj --home $HOME
yes $PASSPHRASE | injectived add-genesis-account $(yes $PASSPHRASE | injectived keys show $USER4_KEY -a --home $HOME) 1000000000000000000000000inj --home $HOME

echo "Setup done!"
