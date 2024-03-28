const { getMessage } = require('eip-712')
const { utils } = require('ethers')

let typedData = {
  "types": {
    "EIP712Domain": [
      {
        "name": "name",
        "type": "string"
      },
      {
        "name": "version",
        "type": "string"
      },
      {
        "name": "chainId",
        "type": "uint256"
      },
      {
        "name": "verifyingContract",
        "type": "address"
      },
      {
        "name": "salt",
        "type": "bytes32"
      }
    ],
    "OrderInfo": [
      {
        "name": "SubaccountId",
        "type": "string"
      },
      {
        "name": "FeeRecipient",
        "type": "string"
      },
      {
        "name": "Price",
        "type": "string"
      },
      {
        "name": "Quantity",
        "type": "string"
      }
    ],
    "SpotLimitOrder": [
      {
        "name": "MarketId",
        "type": "string"
      },
      {
        "name": "OrderInfo",
        "type": "OrderInfo"
      },
      {
        "name": "OrderType",
        "type": "string"
      },
      {
        "name": "TriggerPrice",
        "type": "string"
      },
      {
        "name": "Salt",
        "type": "string"
      }
    ]
  },
  "primaryType": "SpotLimitOrder",
  "domain": {
    "name": "Injective Protocol",
    "version": "2.0.0",
    "chainId": "0x378",
    "verifyingContract": "0xCcCCccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC",
    "salt": "0x0000000000000000000000000000000000000000000000000000000000000000"
  },
  "message": {
    "MarketId": "0xb0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773",
    "OrderInfo": {
      "FeeRecipient": "inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz",
      "Price": "137.000000000000000000",
      "Quantity": "24.000000000000000000",
      "SubaccountId": "90f8bf6a479f320ead074411a4b0e7944ea8c9c1000000000000000000000001"
    },
    "OrderType": "BUY",
    "Salt": "12",
    "TriggerPrice": "1.000000000000000000"
  }
}

// Generate a random private key
const privateKey = Buffer.from("4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d","hex")
const signingKey = new utils.SigningKey(privateKey);

// Get a signable message from the typed data
const message = getMessage(typedData, true)

// Sign the message with the private key
const { r, s, v } = signingKey.signDigest(message);

/* eslint-disable no-console */
console.log(`Message hash: 0x${message.toString('hex')}`);
console.log(`Signature: 0x${r.substr(2)}${s.substr(2)}${v.toString('16')}`);
