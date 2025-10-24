# Banking System

A banking system with fraud detection and transaction processing.

```craft
actors {
  user Customer
  user Admin
  system FraudDetectionService
  system CreditBureauAPI
}

domains {
  Account {
    AccountManagement
    AccountValidation
  }

  Transaction {
    TransactionProcessing
    TransactionHistory
  }

  Fraud {
    FraudDetection
    RiskAssessment
  }

  Notification {
    AlertManagement
    EmailNotification
  }
}

services {
  AccountService {
    domains: Account
    language: java
    data-stores: account_db
    deployment: blue_green
  }

  TransactionService {
    domains: Transaction, Fraud
    language: golang
    data-stores: transaction_db, fraud_cache
    deployment: canary(30% -> staging, 100% -> production)
  }
}

use_case "Transfer Money" {
  when Customer initiates transfer
    AccountValidation validates source account
    AccountValidation validates destination account
    AccountValidation checks sufficient balance
    TransactionProcessing creates pending transaction
    TransactionProcessing notifies "Transaction Initiated"

  when FraudDetection listens "Transaction Initiated"
    RiskAssessment analyzes transaction pattern
    RiskAssessment asks FraudDetectionService for risk score
    FraudDetectionService returns to RiskAssessment the risk score
    RiskAssessment evaluates risk threshold

  when RiskAssessment notifies "High Risk Detected"
    TransactionProcessing freezes transaction
    AlertManagement notifies "Fraud Alert"

  when AlertManagement listens "Fraud Alert"
    AlertManagement asks EmailNotification to send alert

  when RiskAssessment notifies "Risk Approved"
    TransactionProcessing executes transfer
    AccountManagement updates balances
    TransactionProcessing notifies "Transfer Completed"

  when EmailNotification listens "Transfer Completed"
    EmailNotification sends confirmation email
}

use_case "Fraud Investigation" {
  when Admin reviews flagged transaction
    FraudDetection retrieves transaction details
    FraudDetection asks CreditBureauAPI for customer history
    CreditBureauAPI returns to FraudDetection the credit history

  when Admin approves transaction
    TransactionProcessing unfreezes transaction
    TransactionProcessing executes transfer
    TransactionProcessing notifies "Manual Approval"

  when Admin rejects transaction
    TransactionProcessing cancels transaction
    AccountManagement reverses holds
    TransactionProcessing notifies "Transaction Rejected"
}
```

## Key Patterns

- **Fraud detection as async process**
- **Manual approval workflows**
- **Risk assessment integration**
- **Canary deployment for critical service**
