# Babylon Staking Expiry Checker

## Overview

The **Babylon Staking Expiry Checker** is a service designed to manage Phase 1 delegations that haven't transitioned to Phase 2 in the Babylon Protocol. It operates independently to handle the unbonding and withdrawal processes for legacy Phase 1 delegations that don't exist on the Babylon chain.

### Key Responsibilities

- **Timelock Expiry Tracking**: Monitors and updates the status of timelock expirations
- **BTC Transaction Monitoring**: Tracks unbonding and withdrawal transactions on Bitcoin
- **State Management**: Maintains accurate states of Phase 1 delegations throughout the unbonding process

## Architecture

The Staking Expiry Checker shares a database with other Phase 1 services and operates independently:

- **Shared MongoDB Database**: 
  - Read/Write access to the same database used by Phase 1 indexer and Phase 1 API
  - Monitors and updates delegation states
- **Bitcoin Node**: Monitors BTC transactions for unbonding and withdrawal events

## Service Components

### 1. BTC Subscriber Poller
- Subscribes to Bitcoin spend notifications
- Monitors:
  - Staking transaction spends
  - Unbonding transaction confirmations
  - Withdrawal transaction confirmations
- Updates delegation states in shared database

### 2. Expiry Checker Routine
- Polls timelock queue table
- Identifies expired timelocks for staking/unbonding
- Updates delegation status to "Unbonded" when timelock expires
- No slashing mechanism in Phase 1

## Workflow

1. **Database Monitoring**
   - Service reads unbonding requests from shared database
   - These requests are written by other Phase 1 services

2. **BTC Transaction Monitoring**
   - BTC Subscriber monitors relevant Bitcoin transactions
   - Updates transaction states in shared database

3. **Timelock Management**
   - Expiry checker polls timelock queue
   - Updates delegation status upon timelock expiration

4. **Withdrawal Tracking**
   - BTC subscriber identifies withdrawal transactions
   - Updates final delegation status

## Installation & Setup

### Requirements

- Go 1.23.1 or later
- MongoDB
- Bitcoin Node (with RPC access)

### Configuration

The service requires two configuration files:

1. **Main Configuration** (`config.yml`):
```yaml
pollers:
  log-level: debug
  expiry-checker:
    interval: 10s
    timeout: 100s
  btc-subscriber:
    interval: 10s
    timeout: 100s
db:
  username: <username>
  password: <password>
  address: <mongodb-address>
  db-name: <database-name>
  max-pagination-limit: 1000
btc:
  rpchost: <bitcoin-node-address>
  rpcuser: <rpc-username>
  rpcpass: <rpc-password>
  netparams: signet  # or mainnet, testnet
metrics:
  host: 0.0.0.0
  port: 2112
```

### Running the Service

1. **Local Development**
```bash
make run-local
```

2. **Production Deployment**
```bash
make build-docker
make start-service
```

## Monitoring

The service exposes Prometheus metrics at `:2112/metrics` including:
- Timelock expiry statistics
- BTC transaction monitoring status
- Service health metrics

## Development

### Testing
```bash
make test
```

### Generating Mocks
```bash
make generate-mock-interface
```

## License

This project is licensed under the Business Source License 1.1 - see the [LICENSE](LICENSE) file for details.

## Support

For support and questions, please open an issue in the GitHub repository.